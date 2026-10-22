package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveRepoRejectsMissingRoot(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-root")
	_, _, err := resolveRepo(missing)
	if err == nil {
		t.Fatal("resolveRepo accepted a nonexistent root")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Fatalf("error must name the path, got %q", err.Error())
	}
}

func TestResolveRepoRejectsRootWithoutSources(t *testing.T) {
	empty := t.TempDir()
	_, _, err := resolveRepo(empty)
	if err == nil {
		t.Fatal("resolveRepo accepted a root with no arena or corpus sources")
	}
	if !strings.Contains(err.Error(), empty) {
		t.Fatalf("error must name the path, got %q", err.Error())
	}
}

func TestResolveRepoAcceptsArchiveOnlyRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, arenaDirArchive), 0o755); err != nil {
		t.Fatal(err)
	}
	roots, corpusPath, err := resolveRepo(root)
	if err != nil {
		t.Fatalf("resolveRepo rejected a usable root: %v", err)
	}
	if len(roots) != 3 || corpusPath != filepath.Join(root, corpusDir) {
		t.Fatalf("roots = %v, corpus = %q", roots, corpusPath)
	}
}
