package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
func TestSweepStaleTempRemovesGateColony(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range [...]string{"caro-mutate-123", "go-build999", "keep-me"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	sweepStaleTemp(root)
	for _, name := range [...]string{"caro-mutate-123", "go-build999"} {
		if _, serr := os.Stat(filepath.Join(root, name)); !os.IsNotExist(serr) {
			t.Errorf("sweep left %s behind", name)
		}
	}
	if _, serr := os.Stat(filepath.Join(root, "keep-me")); serr != nil {
		t.Errorf("sweep removed an unrelated dir: %v", serr)
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

func TestIsolateModuleWithoutTempRoot(t *testing.T) {
	src := t.TempDir()
	missing := filepath.Join(src, "no-temp")
	t.Setenv("TMP", missing)
	t.Setenv("TEMP", missing)
	t.Setenv("TMPDIR", missing)
	if _, err := isolateModule(src); err == nil {
		t.Error("isolateModule(no temp root) err = nil, want the mkdir failure")
	}
}

func TestIsolateModuleBadGlobRoot(t *testing.T) {
	src := filepath.Join(t.TempDir(), "weird[dir")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(src, "go.mod"), "module probe\n\ngo 1.27\n")
	mustWrite(t, filepath.Join(src, "internal", "a", "a.go"), "package a\n\nfunc F() int { return 1 }\n")
	if _, err := isolateModule(src); err == nil {
		t.Error("isolateModule(unclosed-bracket root) err = nil, want the root glob failure")
	}
}

func TestIsolateModuleRootGoEntryAsDirectory(t *testing.T) {
	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "go.mod"), "module probe\n\ngo 1.27\n")
	if err := os.Mkdir(filepath.Join(src, "zz.go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := isolateModule(src); err == nil {
		t.Error("isolateModule(root glob entry is a directory) err = nil, want the read failure")
	}
}

func TestTreeHashBadGlobRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "weird[dir")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := treeHash(root); err == nil {
		t.Error("treeHash(unclosed-bracket root) err = nil, want the glob failure")
	}
}

func TestTreeHashRootGoEntryAsDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "zz.go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := treeHash(root); err == nil {
		t.Error("treeHash(root glob entry is a directory) err = nil, want the hash failure")
	}
}

func TestTreeHashCoversRootGoFiles(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "root.go")
	mustWrite(t, target, "package root\n\nfunc R() int { return 1 }\n")
	got, err := treeHash(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got[target]; !ok {
		t.Errorf("treeHash missing the root .go file %s: %v", target, got)
	}
}

func TestSweepStaleTempBadRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "weird[dir")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "caro-mutate-1")
	if err := os.Mkdir(marker, 0o755); err != nil {
		t.Fatal(err)
	}
	sweepStaleTemp(root)
	if _, serr := os.Stat(marker); serr != nil {
		t.Errorf("sweep deleted %s despite the glob failure: %v", marker, serr)
	}
}

func TestResidueCheckBadRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "weird[dir")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := residueCheck(root); err == nil {
		t.Error("residueCheck(unclosed-bracket root) err = nil, want the glob failure")
	}
}

func TestResidueCheckReportsPinnedColony(t *testing.T) {
	root := t.TempDir()
	colony := filepath.Join(root, "caro-mutate-999")
	if err := os.MkdirAll(filepath.Join(colony, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	pinnedFile := filepath.Join(colony, "sub", "pinned.go")
	mustWrite(t, pinnedFile, "package p\n")
	if runtime.GOOS == "windows" {
		held, herr := os.OpenFile(pinnedFile, os.O_RDWR, 0o644)
		if herr != nil {
			t.Fatal(herr)
		}
		defer func() { _ = held.Close() }()
	} else {
		if cerr := os.Chmod(colony, 0o500); cerr != nil {
			t.Fatal(cerr)
		}
		t.Cleanup(func() {
			_ = os.Chmod(colony, 0o755)
			_ = os.RemoveAll(colony)
		})
	}
	err := residueCheck(root)
	if err == nil {
		t.Fatal("residueCheck(pinned colony) err = nil, want the pinned-residue report")
	}
	if !strings.Contains(err.Error(), "survived the run") || !strings.Contains(err.Error(), colony) {
		t.Errorf("residue error %q does not name the pinned colony", err)
	}
}
