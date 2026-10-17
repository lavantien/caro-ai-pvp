//go:build !windows

package main

import (
	"fmt"
	"os/exec"
)

func ulimitArgv(kb int64, argv []string) []string {
	wrapped := make([]string, 0, len(argv)+4)
	wrapped = append(wrapped, "sh", "-c", fmt.Sprintf("ulimit -v %d; exec \"$@\"", kb))
	return append(wrapped, argv...)
}

func cappedArgv(argv []string, capBytes int64) []string {
	return ulimitArgv(capBytes/1024, argv)
}

func capStartedTree(cmd *exec.Cmd, capBytes int64) (release func(), err error) {
	return func() {}, nil
}
