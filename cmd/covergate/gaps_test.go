package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func fileAsOutDir(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestBadgeWritersRefuseFileOutDirs(t *testing.T) {
	now := time.Now()
	if err := badgesCmd(writeProfile(t, badgeFixture), fileAsOutDir(t), "abc", "42", config.VerdictPassed, now); err == nil {
		t.Fatal("badges over a file out dir must fail")
	}
	if err := verdictCmd(config.VerdictPassed, fileAsOutDir(t), "abc", "42", now); err == nil {
		t.Fatal("verdict over a file out dir must fail")
	}
	if err := defaultsCmd(fileAsOutDir(t), now); err == nil {
		t.Fatal("defaults over a file out dir must fail")
	}
}

func TestBadgeWritesFailOnCollidingEntries(t *testing.T) {
	dirWith := func(t *testing.T, name string) string {
		t.Helper()
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	if err := writeBadgeFiles(dirWith(t, "ci.svg"), config.VerdictPassed, "{}"); err == nil {
		t.Fatal("a directory named ci.svg must fail the badge write")
	}
	if err := writeBadgeFiles(dirWith(t, "source.json"), config.VerdictPassed, "{}"); err == nil {
		t.Fatal("a directory named source.json must fail the badge write")
	}
	if err := defaultsCmd(dirWith(t, "coverage-global.svg"), time.Now()); err == nil {
		t.Fatal("a directory named coverage-global.svg must fail the defaults write")
	}
}
