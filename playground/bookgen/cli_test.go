package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func TestValidateCorpusArgs(t *testing.T) {
	if err := validateCorpusArgs(1, 0, config.TierMaster.Name); err != nil {
		t.Fatalf("minimal corpus args rejected: %v", err)
	}
	if err := validateCorpusArgs(0, 0, config.TierMaster.Name); err == nil {
		t.Errorf("zero games accepted")
	}
	if err := validateCorpusArgs(1, -1, config.TierMaster.Name); err == nil {
		t.Errorf("negative tc accepted")
	}
	if err := validateCorpusArgs(1, len(config.TimeControls), config.TierMaster.Name); err == nil {
		t.Errorf("tc past the ladder accepted")
	}
	if err := validateCorpusArgs(1, 0, "no-such-tier"); err == nil {
		t.Errorf("unknown tier accepted")
	}
}

func TestValidateFloor(t *testing.T) {
	if err := validateFloor(1); err != nil {
		t.Fatalf("floor 1 rejected: %v", err)
	}
	if err := validateFloor(0); err == nil {
		t.Errorf("floor 0 accepted")
	}
	if err := validateFloor(-3); err == nil {
		t.Errorf("negative floor accepted")
	}
}

func TestValidateReadSidecar(t *testing.T) {
	dir := t.TempDir()
	if err := validateReadSidecar(filepath.Join(dir, "absent.txt"), "corpus"); err == nil {
		t.Errorf("missing corpus accepted")
	}
	empty := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatalf("write empty: %v", err)
	}
	if err := validateReadSidecar(empty, "overlay"); err == nil {
		t.Errorf("empty overlay accepted")
	}
	if err := validateReadSidecar(dir, "corpus"); err == nil {
		t.Errorf("directory accepted as a sidecar")
	}
	filled := filepath.Join(dir, "filled.txt")
	if err := os.WriteFile(filled, []byte("x"), 0o644); err != nil {
		t.Fatalf("write filled: %v", err)
	}
	if err := validateReadSidecar(filled, "corpus"); err != nil {
		t.Fatalf("filled sidecar rejected: %v", err)
	}
}
