package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

var isolateDirs = [...]string{"internal", "cmd"}

// isolateModule copies the mutable surface of the module at src into a fresh
// temp dir so mutants never touch the working tree. Returns the temp root.
func isolateModule(src string) (string, error) {
	dst, err := os.MkdirTemp("", "caro-mutate-")
	if err != nil {
		return "", err
	}
	for _, name := range [...]string{"go.mod", "go.sum"} {
		data, rerr := os.ReadFile(filepath.Join(src, name))
		if rerr != nil {
			if os.IsNotExist(rerr) {
				continue
			}
			return "", rerr
		}
		if werr := os.WriteFile(filepath.Join(dst, name), data, 0o644); werr != nil {
			return "", werr
		}
	}
	for _, dir := range isolateDirs {
		werr := copyDir(filepath.Join(src, dir), filepath.Join(dst, dir))
		if werr != nil {
			if os.IsNotExist(werr) {
				continue
			}
			return "", werr
		}
	}
	rootGo, gerr := filepath.Glob(filepath.Join(src, "*.go"))
	if gerr != nil {
		return "", gerr
	}
	for _, p := range rootGo {
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return "", rerr
		}
		if werr := os.WriteFile(filepath.Join(dst, filepath.Base(p)), data, 0o644); werr != nil {
			return "", werr
		}
	}
	return dst, nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(src, path)
		if rerr != nil {
			return rerr
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// treeHash fingerprints every .go file under the mutable dirs of root plus
// the root level, mirroring what isolateModule copies. The mutator
// snapshots this before and after a run: the live tree must be
// byte-identical when a mutation session ends.
func treeHash(root string) (map[string]string, error) {
	out := map[string]string{}
	for _, dir := range isolateDirs {
		base := filepath.Join(root, dir)
		werr := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() || filepath.Ext(path) != ".go" {
				return nil
			}
			sum, herr := fileHash(path)
			if herr != nil {
				return herr
			}
			out[path] = sum
			return nil
		})
		if werr != nil {
			return nil, werr
		}
	}
	rootGo, gerr := filepath.Glob(filepath.Join(root, "*.go"))
	if gerr != nil {
		return nil, gerr
	}
	for _, p := range rootGo {
		sum, herr := fileHash(p)
		if herr != nil {
			return nil, herr
		}
		out[p] = sum
	}
	return out, nil
}

// sweepStaleIsolates removes leftover caro-mutate-* temp copies from runs
// that died too hard for their deferred cleanup (TerminateProcess skips
// defers). Safe at gate start: the exclusive-machine rule means no sibling
// gate owns a live copy.
func sweepStaleIsolates() {
	stale, err := filepath.Glob(filepath.Join(os.TempDir(), "caro-mutate-*"))
	if err != nil {
		return
	}
	for _, p := range stale {
		_ = os.RemoveAll(p)
	}
}

// removeAllRetried removes a temp entry with a bounded retry: on Windows a
// directory handle can outlive its process by milliseconds, and a single
// RemoveAll racing that teardown deletes the contents but leaves the empty
// directory shell. Missing paths are success.
func removeAllRetried(dir string) {
	for range config.MutateRemoveRetryAttempts {
		if err := os.RemoveAll(dir); err == nil {
			return
		}
		time.Sleep(time.Duration(config.MutateRemoveRetryDelayMs) * time.Millisecond)
	}
}

// residueCheck fails while any caro-mutate-* temp entry remains under root:
// a leftover isolate or cache dir means a child outlived its run, so the
// gate reports the leak instead of exiting green over pinned resources.
// Entries a final retried sweep can still delete are release races, not
// leaks; only the truly pinned fail the gate.
func residueCheck(root string) error {
	left, err := filepath.Glob(filepath.Join(root, "caro-mutate-*"))
	if err != nil {
		return err
	}
	if len(left) == 0 {
		return nil
	}
	var pinned []string
	for _, p := range left {
		removeAllRetried(p)
		if _, err := os.Stat(p); err == nil {
			pinned = append(pinned, p)
		}
	}
	if len(pinned) == 0 {
		return nil
	}
	return fmt.Errorf("%d caro-mutate-* entries survived the run (a child process pinned them): %s", len(pinned), strings.Join(pinned, "; "))
}

func fileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
