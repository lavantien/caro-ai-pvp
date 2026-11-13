package main

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func headerConstrainedToE2E(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "package ") {
			return false, nil
		}
		fields := strings.Fields(line)
		if fields[0] != "//go:build" {
			continue
		}
		for _, term := range fields[1:] {
			if term == "e2e" {
				return true, nil
			}
		}
	}
	return false, sc.Err()
}

func TestE2ETagPartition(t *testing.T) {
	want := map[string]bool{
		"internal/tourney/e2e_test.go":        true,
		"internal/tourney/hexrun_e2e_test.go": true,
	}
	root := filepath.Join("..", "..")
	got := map[string]bool{}
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		tagged, terr := headerConstrainedToE2E(path)
		if terr != nil {
			return terr
		}
		if tagged {
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			got[filepath.ToSlash(rel)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk internal: %v", err)
	}
	for _, p := range []string{"internal/tourney/e2e_test.go", "internal/tourney/hexrun_e2e_test.go"} {
		if !got[p] {
			t.Errorf("tagged file %s missing from the partition, found %v", p, got)
		}
	}
	for p := range got {
		if !want[p] {
			t.Errorf("test file %s carries the e2e build tag outside the partition", p)
		}
	}
}
