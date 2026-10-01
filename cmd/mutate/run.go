package main

import (
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

func executeMutants(ctx context.Context, out io.Writer, workDir string, ms []mutation, store *fileStore, r runner) (res result, err error) {
	res.total = len(ms)
	defer func() {
		if rerr := store.restoreAll(); rerr != nil && err == nil {
			err = rerr
		}
	}()
	for _, m := range ms {
		if ctx.Err() != nil {
			return res, fmt.Errorf("interrupted after %d/%d mutants", res.run, res.total)
		}
		orig := store.orig[m.file]
		mutated, aerr := applyEdits(orig, m.edits)
		if aerr != nil {
			return res, aerr
		}
		if werr := os.WriteFile(m.file, mutated, 0o644); werr != nil {
			return res, werr
		}
		testErr := r.runTest(ctx, m.pkg)
		res.run++
		if werr := os.WriteFile(m.file, orig, 0o644); werr != nil {
			return res, werr
		}
		if ctx.Err() != nil {
			return res, fmt.Errorf("interrupted after %d/%d mutants", res.run, res.total)
		}
		if testErr == nil {
			res.survived++
		} else {
			res.killed++
		}
		_, _ = fmt.Fprintf(out, "%s:%d:%d %s %s\n", displayPath(workDir, m.file), m.line, m.col, m.desc, classify(testErr))
	}
	return res, nil
}

func runMutation(ctx context.Context, out io.Writer, workDir string, patterns []string, r runner) (result, error) {
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
		return result{}, err
	}
	var ms []mutation
	var paths []string
	for _, tg := range targets {
		for _, name := range tg.files {
			path := filepath.Join(tg.dir, name)
			src, rerr := os.ReadFile(path)
			if rerr != nil {
				return result{}, rerr
			}
			fms, cerr := collect(path, src)
			if cerr != nil {
				return result{}, cerr
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
	store := newFileStore()
	if err := store.snapshot(paths); err != nil {
		return result{}, err
	}
	return executeMutants(ctx, out, workDir, ms, store, r)
}
