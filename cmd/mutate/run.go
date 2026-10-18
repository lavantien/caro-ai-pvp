package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

const (
	verdictKilled     = "KILLED"
	verdictSurvived   = "SURVIVED"
	verdictChallenged = "CHALLENGED"
)

var errInterrupted = errors.New("interrupted")

type runner interface {
	runTest(ctx context.Context, pkg string) error
}

type execRunner struct {
	dir      string
	cacheDir string
	timeout  time.Duration
}

func newRunCache() (string, error) {
	return os.MkdirTemp("", "caro-mutate-cache-")
}

func goEnv(cacheDir string) []string {
	env := make([]string, 0, len(os.Environ())+2)
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CGO_ENABLED=") || strings.HasPrefix(e, "GOCACHE=") {
			continue
		}
		env = append(env, e)
	}
	return append(env, "CGO_ENABLED=1", "GOCACHE="+cacheDir)
}

func (r execRunner) runTest(ctx context.Context, pkg string) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	argv := []string{"go", "test", "-count=1", "-short", "-timeout", (r.timeout - time.Duration(config.MutateTestTimeoutSlack)*time.Millisecond).String(), pkg}
	wrapped := cappedArgv(argv, config.MutateTreeMemCapBytes)
	cmd := exec.CommandContext(ctx, wrapped[0], wrapped[1:]...)
	cmd.Dir = r.dir
	cmd.Env = goEnv(r.cacheDir)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return err
	}
	release, err := capStartedTree(cmd, config.MutateTreeMemCapBytes)
	if err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	defer release()
	return cmd.Wait()
}

func classify(err error) string {
	if err == nil {
		return verdictSurvived
	}
	return verdictKilled
}

func isInfraError(err error) bool {
	if err == nil {
		return false
	}
	var ee *exec.ExitError
	return !errors.As(err, &ee)
}

func hasRunnableMutants(ms []mutation, workDir string, resumeKilled map[string]bool, allows allowlist, challenge bool) bool {
	for _, m := range ms {
		key := m.key(workDir)
		if resumeKilled[key] {
			continue
		}
		if _, isAllowed := allows[key]; isAllowed && !challenge {
			continue
		}
		return true
	}
	return false
}

func runBaseline(ctx context.Context, out io.Writer, ms []mutation, r runner) error {
	seen := map[string]bool{}
	for _, m := range ms {
		if seen[m.pkg] {
			continue
		}
		seen[m.pkg] = true
		if err := r.runTest(ctx, m.pkg); err != nil {
			return fmt.Errorf("baseline suite failed for %s: %v (unmutated suite must be green before scoring mutants)", m.pkg, err)
		}
	}
	if len(seen) > 0 {
		_, _ = fmt.Fprintf(out, "mutate: baseline green for %d package(s)\n", len(seen))
	}
	return nil
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

func loadResumeLog(r io.Reader) (map[string]bool, error) {
	resumed := map[string]bool{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if key, ok := strings.CutSuffix(line, " KILLED"); ok && key != line {
			resumed[key] = true
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return resumed, nil
}

func executeMutants(ctx context.Context, out io.Writer, workDir string, ms []mutation, store *fileStore, r runner, allows allowlist, resumeKilled map[string]bool, challenge bool) (res result, consumed allowlist, survived []mutation, challenged []mutation, err error) {
	res.total = len(ms)
	consumed = allowlist{}
	defer func() {
		if rerr := store.restoreAll(); rerr != nil && err == nil {
			err = rerr
		}
	}()
	for _, m := range ms {
		if ctx.Err() != nil {
			return res, consumed, survived, challenged, fmt.Errorf("interrupted after %d/%d mutants", res.run, res.total)
		}
		key := m.key(workDir)
		if resumeKilled[key] {
			res.run++
			res.killed++
			_, _ = fmt.Fprintf(out, "%s %s\n", key, verdictKilled)
			continue
		}
		reason, isAllowed := allows[key]
		if isAllowed && !challenge {
			res.run++
			res.allowed++
			consumed[key] = reason
			_, _ = fmt.Fprintf(out, "%s ALLOWED # %s\n", key, reason)
			continue
		}
		orig := store.orig[m.file]
		mutated, aerr := applyEdits(orig, m.edits)
		if aerr != nil {
			return res, consumed, survived, challenged, aerr
		}
		if werr := os.WriteFile(m.file, mutated, 0o644); werr != nil {
			return res, consumed, survived, challenged, werr
		}
		testErr := r.runTest(ctx, m.pkg)
		res.run++
		if werr := os.WriteFile(m.file, orig, 0o644); werr != nil {
			return res, consumed, survived, challenged, werr
		}
		if ctx.Err() != nil {
			return res, consumed, survived, challenged, fmt.Errorf("interrupted after %d/%d mutants", res.run, res.total)
		}
		if isInfraError(testErr) {
			return res, consumed, survived, challenged, fmt.Errorf("infra failure at %s: %v (verdicts so far void, rerun the gate)", key, testErr)
		}
		switch {
		case classify(testErr) == verdictKilled && isAllowed:
			challenged = append(challenged, m)
			_, _ = fmt.Fprintf(out, "%s %s\n", key, verdictChallenged)
		case classify(testErr) == verdictKilled:
			res.killed++
			_, _ = fmt.Fprintf(out, "%s %s\n", key, verdictKilled)
		case isAllowed:
			res.allowed++
			consumed[key] = reason
			_, _ = fmt.Fprintf(out, "%s ALLOWED # %s\n", key, reason)
		default:
			res.survived++
			survived = append(survived, m)
			_, _ = fmt.Fprintf(out, "%s %s\n", key, verdictSurvived)
		}
	}
	return res, consumed, survived, challenged, nil
}

func demotedError(keys []string) error {
	sort.Strings(keys)
	return fmt.Errorf("%d allow entries no longer equivalent (suite killed the mutant; prune or re-prove): %s", len(keys), strings.Join(keys, "; "))
}

func runMutation(ctx context.Context, out io.Writer, workDir string, patterns []string, cacheDir string, r runner, allows allowlist, resumeKilled map[string]bool, challenge bool) (result, error) {
	ms, paths, err := discoverMutants(ctx, out, workDir, patterns, cacheDir)
	if err != nil {
		return result{}, err
	}
	if hasRunnableMutants(ms, workDir, resumeKilled, allows, challenge) {
		if berr := runBaseline(ctx, out, ms, r); berr != nil {
			return result{}, berr
		}
	}
	store := newFileStore()
	if err := store.snapshot(paths); err != nil {
		return result{}, err
	}
	res, consumed, _, challenged, err := executeMutants(ctx, out, workDir, ms, store, r, allows, resumeKilled, challenge)
	if err != nil {
		return res, err
	}
	if len(challenged) > 0 {
		keys := make([]string, 0, len(challenged))
		for _, m := range challenged {
			keys = append(keys, m.key(workDir))
		}
		return res, demotedError(keys)
	}
	if uerr := unusedAllowError(unconsumed(allows, consumed)); uerr != nil {
		return res, uerr
	}
	return res, nil
}

func discoverMutants(ctx context.Context, out io.Writer, workDir string, patterns []string, cacheDir string) ([]mutation, []string, error) {
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
	targets, err := discover(ctx, workDir, present, cacheDir)
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
