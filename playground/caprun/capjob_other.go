//go:build !windows

package main

import "os/exec"

func capSupported() bool { return false }

func applyCap(*exec.Cmd, int64) error { return nil }
