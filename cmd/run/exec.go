package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type exitCoder struct {
	code int
	err  error
}

func (e *exitCoder) Error() string { return e.err.Error() }
func (e *exitCoder) Unwrap() error { return e.err }

func execStep(s step, env []string) (int, error) {
	cmd := exec.Command(s.path, s.args...)
	cmd.Env = env
	cmd.Dir = s.dir
	if s.logTo != "" {
		f, err := os.OpenFile(s.logTo, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return 0, fmt.Errorf("open log %s: %w", s.logTo, err)
		}
		defer func() {
			if cerr := f.Close(); cerr != nil {
				fmt.Fprintf(os.Stderr, "run: close log %s: %v\n", s.logTo, cerr)
			}
		}()
		cmd.Stdout = f
		cmd.Stderr = f
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	if err := cmd.Wait(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode(), nil
		}
		return 0, err
	}
	return 0, nil
}

func execCapture(path string, args []string, env []string, dir string) (string, int, error) {
	cmd := exec.Command(path, args...)
	cmd.Env = env
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return strings.TrimSpace(string(out)), ee.ExitCode(), nil
		}
		return "", 0, err
	}
	return strings.TrimSpace(string(out)), 0, nil
}

func exitCode(err error) {
	var ec *exitCoder
	if errors.As(err, &ec) {
		os.Exit(ec.code)
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		os.Exit(int(errno))
	}
	os.Exit(1)
}

func goEnvGOEXE() (string, error) {
	out, code, err := execCapture("go", []string{"env", "GOEXE"}, os.Environ(), "")
	if err != nil || code != 0 {
		return "", fmt.Errorf("go env GOEXE: %v (exit %d)", err, code)
	}
	return out, nil
}
