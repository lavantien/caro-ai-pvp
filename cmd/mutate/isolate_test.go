package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTree(t *testing.T, root string) {
	t.Helper()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/isolated\n\ngo 1.27.1\n")
	mustWrite(t, filepath.Join(root, "internal", "rules", "a.go"), "package rules\n\nfunc A() int { return 1 }\n")
	mustWrite(t, filepath.Join(root, "internal", "rules", "a_test.go"), "package rules\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { if A() != 1 { t.Fatal() } }\n")
	mustWrite(t, filepath.Join(root, "internal", "rules", "web", "pwa", "sw.js"), "self.register()\n")
	mustWrite(t, filepath.Join(root, "internal", "notes.txt"), "not Go source, part of the module mirror\n")
	mustWrite(t, filepath.Join(root, "cmd", "x", "main.go"), "package main\n\nfunc main() {}\n")
	mustWrite(t, filepath.Join(root, "ref", "ignore.txt"), "must not be copied\n")
}
func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
func TestIsolateModule(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	writeTree(t, src)
	dst, err := isolateModule(src)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dst) })
	for _, rel := range [...]string{
		filepath.Join("go.mod"),
		filepath.Join("internal", "rules", "a.go"),
		filepath.Join("internal", "rules", "a_test.go"),
		filepath.Join("internal", "rules", "web", "pwa", "sw.js"),
		filepath.Join("internal", "notes.txt"),
		filepath.Join("cmd", "x", "main.go"),
	} {
		if _, serr := os.Stat(filepath.Join(dst, rel)); serr != nil {
			t.Errorf("isolate missing %s: %v", rel, serr)
		}
	}
	if _, serr := os.Stat(filepath.Join(dst, "ref")); !os.IsNotExist(serr) {
		t.Errorf("isolate copied excluded dir ref")
	}
}
func TestTreeHashDetectsDrift(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root)
	before, err := treeHash(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 3 {
		t.Fatalf("treeHash = %d files, want 3", len(before))
	}
	target := filepath.Join(root, "internal", "rules", "a.go")
	mustWrite(t, target, "package rules\n\nfunc A() int { return 2 }\n")
	after, err := treeHash(root)
	if err != nil {
		t.Fatal(err)
	}
	if before[target] == after[target] {
		t.Fatal("treeHash did not detect mutation")
	}
}
func TestIsolateModuleWithoutMutableDirs(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "go.mod"), "module example.com/bare\n\ngo 1.27.1\n")
	dst, err := isolateModule(src)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dst) })
	if _, serr := os.Stat(filepath.Join(dst, "go.mod")); serr != nil {
		t.Errorf("isolate missing go.mod: %v", serr)
	}
}
func TestIsolateModuleUnreadableGoMod(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	if err := os.Mkdir(filepath.Join(src, "go.mod"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := isolateModule(src); err == nil {
		t.Error("isolateModule(go.mod as directory) err = nil, want read error")
	}
}
func TestFileHashUnreadable(t *testing.T) {
	if _, err := fileHash(t.TempDir()); err == nil {
		t.Error("fileHash(directory) err = nil, want read error")
	}
}
func TestResidueCheckSweepsReleaseRaces(t *testing.T) {
	root := t.TempDir()
	if err := residueCheck(root); err != nil {
		t.Errorf("residueCheck(clean) = %v, want nil", err)
	}
	left := filepath.Join(root, "caro-mutate-cache-999")
	if err := os.Mkdir(left, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := residueCheck(root); err != nil {
		t.Errorf("residueCheck(sweepable leftover) = %v, want nil after sweep", err)
	}
	if _, serr := os.Stat(left); !os.IsNotExist(serr) {
		t.Error("residueCheck left the sweepable leftover on disk")
	}
}
