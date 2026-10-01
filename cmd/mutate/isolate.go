package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
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

// treeHash fingerprints every .go file under the mutable dirs of root. The
// mutator snapshots this before and after a run: the live tree must be
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
	return out, nil
}

func fileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
