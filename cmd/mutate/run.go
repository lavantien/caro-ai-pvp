package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

const (
	verdictKilled   = "KILLED"
	verdictSurvived = "SURVIVED"
)

type runner interface {
	runTest(ctx context.Context, pkg string) error
}

type execRunner struct {
	dir     string
	timeout time.Duration
}

func goEnv() []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CGO_ENABLED=") {
			continue
		}
		env = append(env, e)
	}
	return append(env, "CGO_ENABLED=1")
}

func (r execRunner) runTest(ctx context.Context, pkg string) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-short", "-timeout", (r.timeout - time.Duration(config.MutateTestTimeoutSlack)*time.Millisecond).String(), pkg)
	cmd.Dir = r.dir
	cmd.Env = goEnv()
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func classify(err error) string {
	if err == nil {
		return verdictSurvived
	}
	return verdictKilled
}

type fileStore struct {
	orig map[string][]byte
}

func newFileStore() *fileStore {
	return &fileStore{orig: map[string][]byte{}}
}

func (s *fileStore) snapshot(paths []string) error {
	for _, p := range paths {
		if _, ok := s.orig[p]; ok {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		s.orig[p] = b
	}
	return nil
}

func (s *fileStore) restoreAll() error {
	for path, want := range s.orig {
		got, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Equal(got, want) {
			continue
		}
		if err := os.WriteFile(path, want, 0o644); err != nil {
			return err
		}
		if got, err = os.ReadFile(path); err != nil {
			return err
		}
		if !bytes.Equal(got, want) {
			return fmt.Errorf("restore verification failed for %s", path)
		}
	}
	return nil
}

type result struct {
	killed   int
	survived int
	allowed  int
	run      int
	total    int
}

func displayPath(workDir, path string) string {
	rel, err := filepath.Rel(workDir, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

// loadResumeLog reads a prior run log and collects the keys that ended as
// KILLED. Those verdicts are replayed verbatim on a resume: the tree is
// unchanged between segments, and the suite only ever grows, so a killed
// mutant stays killed. Survivors and allows are re-decided fresh by the
// current suite and allowlist.
func loadResumeLog(r io.Reader) map[string]bool {
	resumed := map[string]bool{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if key, ok := strings.CutSuffix(line, " KILLED"); ok && key != line {
			resumed[key] = true
		}
	}
	return resumed
}

// executeMutants runs every mutant in ms against r inside workDir and
// returns the verdict counts plus the set of allowlist keys it consumed. An
// allowlisted mutant is classified without running the suite: the entry
// asserts a proven equivalence, so the suite verdict under it is noise and
// the run is skipped. Callers aggregate consumed sets across parallel
// workers and fail on unconsumed entries.
func executeMutants(ctx context.Context, out io.Writer, workDir string, ms []mutation, store *fileStore, r runner, allows allowlist, resumeKilled map[string]bool) (res result, consumed allowlist, survived []mutation, err error) {
	res.total = len(ms)
	consumed = allowlist{}
	defer func() {
		if rerr := store.restoreAll(); rerr != nil && err == nil {
			err = rerr
		}
	}()
	for _, m := range ms {
		if ctx.Err() != nil {
			return res, consumed, survived, fmt.Errorf("interrupted after %d/%d mutants", res.run, res.total)
		}
		key := m.key(workDir)
		if resumeKilled[key] {
			res.run++
			res.killed++
			_, _ = fmt.Fprintf(out, "%s %s\n", key, verdictKilled)
			continue
		}
		if reason, ok := allows[key]; ok {
			res.run++
			res.allowed++
			consumed[key] = reason
			_, _ = fmt.Fprintf(out, "%s ALLOWED # %s\n", key, reason)
			continue
		}
		orig := store.orig[m.file]
		mutated, aerr := applyEdits(orig, m.edits)
		if aerr != nil {
			return res, consumed, survived, aerr
		}
		if werr := os.WriteFile(m.file, mutated, 0o644); werr != nil {
			return res, consumed, survived, werr
		}
		testErr := r.runTest(ctx, m.pkg)
		res.run++
		if werr := os.WriteFile(m.file, orig, 0o644); werr != nil {
			return res, consumed, survived, werr
		}
		if ctx.Err() != nil {
			return res, consumed, survived, fmt.Errorf("interrupted after %d/%d mutants", res.run, res.total)
		}
		if verdict := classify(testErr); verdict == verdictKilled {
			res.killed++
			_, _ = fmt.Fprintf(out, "%s %s\n", key, verdict)
		} else {
			res.survived++
			survived = append(survived, m)
			_, _ = fmt.Fprintf(out, "%s %s\n", key, verdict)
		}
	}
	return res, consumed, survived, nil
}

func runMutation(ctx context.Context, out io.Writer, workDir string, patterns []string, r runner, allows allowlist, resumeKilled map[string]bool) (result, error) {
	ms, paths, err := discoverMutants(ctx, out, workDir, patterns)
	if err != nil {
		return result{}, err
	}
	store := newFileStore()
	if err := store.snapshot(paths); err != nil {
		return result{}, err
	}
	res, consumed, _, err := executeMutants(ctx, out, workDir, ms, store, r, allows, resumeKilled)
	if err != nil {
		return res, err
	}
	if uerr := unusedAllowError(unconsumed(allows, consumed)); uerr != nil {
		return res, uerr
	}
	return res, nil
}

// discoverMutants collects the sorted mutant population of the pattern
// targets plus the file paths the caller must snapshot before mutating.
func discoverMutants(ctx context.Context, out io.Writer, workDir string, patterns []string) ([]mutation, []string, error) {
	present := patterns[:0]
	for _, p := range patterns {
		if strings.HasPrefix(p, "./") {
			if _, serr := os.Stat(filepath.Join(workDir, filepath.FromSlash(p))); serr != nil {
				_, _ = fmt.Fprintf(out, "mutate: skip missing target %s\n", p)
				continue
			}
		}
		present = append(present, p)
	}
	targets, err := discover(ctx, workDir, present)
	if err != nil {
		return nil, nil, err
	}
	var ms []mutation
	var paths []string
	for _, tg := range targets {
		for _, name := range tg.files {
			path := filepath.Join(tg.dir, name)
			src, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil, nil, rerr
			}
			fms, cerr := collect(path, src)
			if cerr != nil {
				return nil, nil, cerr
			}
			for i := range fms {
				fms[i].pkg = tg.pkg
			}
			if len(fms) > 0 {
				ms = append(ms, fms...)
				paths = append(paths, path)
			}
		}
	}
	sortMutants(ms)
	return ms, paths, nil
}
