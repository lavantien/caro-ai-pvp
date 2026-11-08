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
	verdictRunError   = "RUN ERROR"
	testFailPrefix    = "--- FAIL:"
	buildFailedSuffix = " [build failed]"
	runErrTailBytes   = 512
)

var errInterrupted = errors.New("interrupted")

type runner interface {
	runTest(ctx context.Context, pkg string) (string, error)
}

type execRunner struct {
	dir      string
	cacheDir string
	tmpDir   string
	timeout  time.Duration
}

func newRunCache() (string, error) {
	return os.MkdirTemp("", "caro-mutate-cache-"+procTag()+"-")
}

func newRunTmp() (string, error) {
	return os.MkdirTemp("", "caro-mutate-tmp-"+procTag()+"-")
}

func goEnv(cacheDir, tmpDir string) []string {
	env := make([]string, 0, len(os.Environ())+3)
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CGO_ENABLED=") || strings.HasPrefix(e, "GOCACHE=") || strings.HasPrefix(e, "GOTMPDIR=") {
			continue
		}
		env = append(env, e)
	}
	return append(env, "CGO_ENABLED=1", "GOCACHE="+cacheDir, "GOTMPDIR="+tmpDir)
}

func testArgv(outer time.Duration, pkg string) []string {
	return []string{"go", "test", "-count=1", "-short", "-timeout", config.SuiteTimeout(outer).String(), pkg}
}

func (r execRunner) runTest(ctx context.Context, pkg string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	argv := testArgv(r.timeout, pkg)
	wrapped := cappedArgv(argv, config.MutateTreeMemCapBytes)
	cmd := exec.CommandContext(ctx, wrapped[0], wrapped[1:]...)
	cmd.Dir = r.dir
	cmd.Env = goEnv(r.cacheDir, r.tmpDir)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		return "", err
	}
	release, err := capStartedTree(cmd, config.MutateTreeMemCapBytes)
	if err != nil {
		_ = cmd.Process.Kill()
		return "", err
	}
	defer release()
	werr := cmd.Wait()
	return out.String(), werr
}

func classify(err error, out, pkg string, baseline map[string]bool) string {
	if err == nil {
		return verdictSurvived
	}
	if hasTestFailure(out) {
		return verdictKilled
	}
	if baseline[pkg] && hasBuildFailure(out, pkg) {
		return verdictKilled
	}
	return verdictRunError
}

func hasTestFailure(out string) bool {
	for line := range strings.SplitSeq(out, "\n") {
		if strings.HasPrefix(line, testFailPrefix) {
			return true
		}
	}
	return false
}

func hasBuildFailure(out, pkg string) bool {
	marker := false
	ownedHeader := false
	diagnostic := false
	for line := range strings.SplitSeq(out, "\n") {
		switch {
		case line == "FAIL\t"+pkg+buildFailedSuffix:
			marker = true
		case line == "# "+pkg || strings.HasPrefix(line, "# "+pkg+" ") || line == "# ["+pkg+"]":
			ownedHeader = true
		case isCompileDiagnostic(line):
			diagnostic = true
		}
	}
	return marker && ownedHeader && diagnostic
}

func isCompileDiagnostic(line string) bool {
	at := strings.LastIndex(line, ".go:")
	if at < 0 {
		return false
	}
	rest := line[at+len(".go:"):]
	lineNo, rest, ok := strings.Cut(rest, ":")
	colNo, _, _ := strings.Cut(rest, ":")
	return ok && isDigits(lineNo) && isDigits(colNo)
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func runErrorf(site string, err error, out string) error {
	return fmt.Errorf("run error %s: %v (no test failure signature and no build failure of the mutated package in the suite output; verdicts so far void, rerun the gate)%s", site, err, suiteTailSuffix(out))
}

func suiteTailSuffix(out string) string {
	if tail := strings.TrimSpace(outputTail(out)); tail != "" {
		return "\nsuite output tail:\n" + tail
	}
	return ""
}

func outputTail(out string) string {
	if len(out) <= runErrTailBytes {
		return out
	}
	return "..." + out[len(out)-runErrTailBytes:]
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

func runBaseline(ctx context.Context, out io.Writer, ms []mutation, r runner) (map[string]bool, error) {
	seen := map[string]bool{}
	green := map[string]bool{}
	for _, m := range ms {
		if seen[m.pkg] {
			continue
		}
		seen[m.pkg] = true
		testOut, err := r.runTest(ctx, m.pkg)
		if err != nil {
			return nil, fmt.Errorf("baseline suite failed for %s: %v (unmutated suite must be green before scoring mutants)%s", m.pkg, err, suiteTailSuffix(testOut))
		}
		green[m.pkg] = true
	}
	if len(seen) > 0 {
		_, _ = fmt.Fprintf(out, "mutate: baseline green for %d package(s)\n", len(seen))
	}
	return green, nil
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

func executeMutants(ctx context.Context, out io.Writer, workDir string, ms []mutation, store *fileStore, r runner, baseline map[string]bool, allows allowlist, resumeKilled map[string]bool, challenge bool) (res result, consumed allowlist, survived []mutation, challenged []mutation, err error) {
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
			reason, isAllowed := allows[key]
			if isAllowed && challenge {
				res.run++
				challenged = append(challenged, m)
				_, _ = fmt.Fprintf(out, "%s %s\n", key, verdictChallenged)
				continue
			}
			res.run++
			res.killed++
			if isAllowed {
				consumed[key] = reason
			}
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
		testOut, testErr := r.runTest(ctx, m.pkg)
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
		verdict := classify(testErr, testOut, m.pkg, baseline)
		switch {
		case verdict == verdictKilled && isAllowed:
			challenged = append(challenged, m)
			_, _ = fmt.Fprintf(out, "%s %s\n", key, verdictChallenged)
		case verdict == verdictKilled:
			res.killed++
			_, _ = fmt.Fprintf(out, "%s %s\n", key, verdictKilled)
		case verdict == verdictRunError:
			_, _ = fmt.Fprintf(out, "%s %s\n", key, verdictRunError)
			return res, consumed, survived, challenged, runErrorf("at "+key, testErr, testOut)
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

func runMutation(ctx context.Context, out io.Writer, workDir string, patterns []string, cacheDir, tmpDir string, r runner, allows allowlist, resumeKilled map[string]bool, challenge bool) (result, error) {
	ms, paths, err := discoverMutants(ctx, out, workDir, patterns, cacheDir, tmpDir)
	if err != nil {
		return result{}, err
	}
	var baseline map[string]bool
	if hasRunnableMutants(ms, workDir, resumeKilled, allows, challenge) {
		green, berr := runBaseline(ctx, out, ms, r)
		if berr != nil {
			return result{}, berr
		}
		baseline = green
	}
	store := newFileStore()
	if err := store.snapshot(paths); err != nil {
		return result{}, err
	}
	res, consumed, _, challenged, err := executeMutants(ctx, out, workDir, ms, store, r, baseline, allows, resumeKilled, challenge)
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

func discoverMutants(ctx context.Context, out io.Writer, workDir string, patterns []string, cacheDir, tmpDir string) ([]mutation, []string, error) {
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
	targets, err := discover(ctx, workDir, present, cacheDir, tmpDir)
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
	pkgsSeen := map[string]bool{}
	for _, m := range ms {
		pkgsSeen[m.pkg] = true
	}
	_, _ = fmt.Fprintf(out, "mutate: plan %d mutants over %d package(s)\n", len(ms), len(pkgsSeen))
	return ms, paths, nil
}
