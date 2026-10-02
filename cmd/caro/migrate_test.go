package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The migrate subcommand owns the schema lifecycle of an operator's database
// file: exit 0 with the file created and the ledger complete, exit 2 on flag
// misuse, exit 1 on an unusable path. The run dispatch arms for serve and
// migrate ride along, the two subcommands main forwards to.

func TestRunMigrateAppliesSchemaAndIsIdempotent(t *testing.T) {
	db := filepath.Join(t.TempDir(), "caro.db")
	if code := runMigrate([]string{"-db", db}); code != 0 {
		t.Fatalf("migrate exit = %d, want 0", code)
	}
	fi, err := os.Stat(db)
	if err != nil {
		t.Fatalf("database file after migrate: %v", err)
	}
	if fi.Size() == 0 {
		t.Fatal("database file is empty, want the schema written")
	}
	// A second pass over the migrated file applies zero scripts and still
	// exits 0: the startup chain is idempotent.
	if code := runMigrate([]string{"-db", db}); code != 0 {
		t.Fatalf("second migrate exit = %d, want 0", code)
	}
}

func TestRunMigrateFlagErrorExitsTwo(t *testing.T) {
	if code := runMigrate([]string{"-nope"}); code != 2 {
		t.Errorf("bad flag exit = %d, want 2", code)
	}
}

func TestRunMigrateUnusablePathExitsOne(t *testing.T) {
	// A directory is not an openable database file: the startup migration
	// fails and the command answers 1, not a panic.
	if code := runMigrate([]string{"-db", t.TempDir()}); code != 1 {
		t.Errorf("directory path exit = %d, want 1", code)
	}
}

func TestRunDispatchesServeAndMigrate(t *testing.T) {
	if code := run([]string{"serve", "-nope"}); code != 2 {
		t.Errorf("serve bad flag exit = %d, want 2", code)
	}
	db := filepath.Join(t.TempDir(), "caro.db")
	if code := run([]string{"migrate", "-db", db}); code != 0 {
		t.Errorf("migrate exit = %d, want 0", code)
	}
	if _, err := os.Stat(db); err != nil {
		t.Errorf("database file after run migrate: %v", err)
	}
}

func TestRunServeUnusableDBPathExitsOne(t *testing.T) {
	// The store must open before anything listens, so a dead path answers 1
	// with no server goroutine left behind.
	if code := runServe([]string{"-db", t.TempDir()}); code != 1 {
		t.Errorf("serve on a directory exit = %d, want 1", code)
	}
}
