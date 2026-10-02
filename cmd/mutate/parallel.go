package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// lockedWriter serializes verdict lines from concurrent workers into one
// log so a resume scan sees whole lines only.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// rewriteMutants retargets absolute mutant file paths from one isolated
// module copy to another. Keys are relative to the work dir, so they stay
// byte-identical across copies and resume logs stay portable.
func rewriteMutants(ms []mutation, from, to string) []mutation {
	out := make([]mutation, len(ms))
	copy(out, ms)
	for i := range out {
		rel, err := filepath.Rel(from, out[i].file)
		if err != nil {
			continue
		}
		out[i].file = filepath.Join(to, rel)
	}
	return out
}

// runMutationParallel spreads the mutant population round-robin over the
// isolated module copies in dirs, each driven by its own runner from
// newRunner(dir). Round-robin mixes files and cost profiles so workers
// finish together. After the workers join, every survivor is re-verified
// once serially on dirs[0]: concurrent load can lose kills, and a survivor
// the serial pass kills is counted as killed with a fresh verdict line.
func runMutationParallel(ctx context.Context, out io.Writer, dirs []string, patterns []string, newRunner func(dir string) runner, allows allowlist, resumeKilled map[string]bool) (result, error) {
	ms, paths, err := discoverMutants(ctx, out, dirs[0], patterns)
	if err != nil {
		return result{}, err
	}
	byKey := make(map[string]mutation, len(ms))
	for _, m := range ms {
		byKey[m.key(dirs[0])] = m
	}

	log := &lockedWriter{w: out}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	type workerOut struct {
		res      result
		consumed allowlist
		survived []mutation
		err      error
	}
	results := make([]workerOut, len(dirs))
	var wg sync.WaitGroup
	for w := range dirs {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			wo := &results[w]
			slice := make([]mutation, 0, len(ms)/len(dirs)+1)
			for i := w; i < len(ms); i += len(dirs) {
				slice = append(slice, ms[i])
			}
			mine := rewriteMutants(slice, dirs[0], dirs[w])
			seen := map[string]bool{}
			myPaths := make([]string, 0, len(paths))
			for _, m := range mine {
				if !seen[m.file] {
					seen[m.file] = true
					myPaths = append(myPaths, m.file)
				}
			}
			store := newFileStore()
			if serr := store.snapshot(myPaths); serr != nil {
				wo.err = fmt.Errorf("worker %d: %w", w, serr)
				cancel()
				return
			}
			res, consumed, survived, rerr := executeMutants(ctx, log, dirs[w], mine, store, newRunner(dirs[w]), allows, resumeKilled)
			wo.res, wo.consumed, wo.survived, wo.err = res, consumed, survived, rerr
			if rerr != nil {
				cancel()
			}
		}(w)
	}
	wg.Wait()

	var res result
	consumedUnion := allowlist{}
	var survivors []mutation
	var firstErr error
	for w := range results {
		wo := &results[w]
		if wo.err != nil && firstErr == nil {
			firstErr = wo.err
		}
		res.run += wo.res.run
		res.killed += wo.res.killed
		res.survived += wo.res.survived
		res.allowed += wo.res.allowed
		for k, v := range wo.consumed {
			consumedUnion[k] = v
		}
		for _, m := range wo.survived {
			if orig, ok := byKey[m.key(dirs[w])]; ok {
				survivors = append(survivors, orig)
			}
		}
	}
	res.total = len(ms)
	if firstErr != nil {
		return res, firstErr
	}
	if serr := confirmSurvivorsSerially(ctx, out, dirs[0], survivors, newRunner(dirs[0]), &res); serr != nil {
		return res, serr
	}
	if uerr := unusedAllowError(unconsumed(allows, consumedUnion)); uerr != nil {
		return res, uerr
	}
	return res, nil
}

// confirmSurvivorsSerially re-runs every survivor alone on one copy and
// flips load-lost kills back: the gate's promise is that a reported
// survivor truly survives without concurrent siblings.
func confirmSurvivorsSerially(ctx context.Context, out io.Writer, dir string, survivors []mutation, r runner, res *result) error {
	if len(survivors) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var paths []string
	for _, m := range survivors {
		if !seen[m.file] {
			seen[m.file] = true
			paths = append(paths, m.file)
		}
	}
	store := newFileStore()
	if err := store.snapshot(paths); err != nil {
		return err
	}
	defer func() { _ = store.restoreAll() }()
	for _, m := range survivors {
		if ctx.Err() != nil {
			return fmt.Errorf("interrupted after %d/%d mutants", res.run, res.total)
		}
		mutated, aerr := applyEdits(store.orig[m.file], m.edits)
		if aerr != nil {
			return aerr
		}
		if werr := os.WriteFile(m.file, mutated, 0o644); werr != nil {
			return werr
		}
		testErr := r.runTest(ctx, m.pkg)
		if werr := os.WriteFile(m.file, store.orig[m.file], 0o644); werr != nil {
			return werr
		}
		if classify(testErr) == verdictKilled {
			res.survived--
			res.killed++
			_, _ = fmt.Fprintf(out, "%s %s\n", m.key(dir), verdictKilled)
		}
	}
	return nil
}
