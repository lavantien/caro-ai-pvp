package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type runningProcess struct {
	Image string
	PID   string
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func buildOutputImage(s step, goExe string) (string, bool) {
	isGoBuild := s.path == "go" && hasArg(s.args, "build")
	isNvcc := s.path == "nvcc"
	if !isGoBuild && !isNvcc {
		return "", false
	}
	for i := 0; i+1 < len(s.args); i++ {
		if s.args[i] != "-o" {
			continue
		}
		target := s.args[i+1]
		if dirOutput(target) {
			name := moduleNameFor(s, i)
			if name == "" {
				return "", false
			}
			return name + goExe, true
		}
		name := filepath.Base(filepath.Clean(target))
		if filepath.Ext(name) == "" {
			name += goExe
		}
		return name, true
	}
	return "", false
}

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func dirOutput(target string) bool {
	clean := filepath.Clean(target)
	return strings.HasSuffix(target, "/") || strings.HasSuffix(target, string(os.PathSeparator)) || clean == "." || clean == ".."
}

func moduleNameFor(s step, oIdx int) string {
	pkg := "."
	if oIdx+2 < len(s.args) {
		pkg = s.args[oIdx+2]
	}
	if pkg != "." && !strings.HasSuffix(pkg, "/.") && pkg != "./." {
		return filepath.Base(pkg)
	}
	for j := 0; j+1 < len(s.args); j++ {
		if s.args[j] == "-C" {
			return filepath.Base(filepath.Clean(s.args[j+1]))
		}
	}
	if s.dir != "" {
		return filepath.Base(filepath.Clean(s.dir))
	}
	return ""
}

func runningPIDsFor(image string) ([]string, error) {
	if image == "" {
		return nil, nil
	}
	procs, err := listRunningProcesses()
	if err != nil {
		return nil, err
	}
	var pids []string
	for _, p := range procs {
		if imageMatches(image, p.Image) {
			pids = append(pids, p.PID)
		}
	}
	return pids, nil
}

func guardRunningImage(s step, image string) error {
	pids, err := runningPIDsFor(image)
	if err != nil {
		return fmt.Errorf("run: guard for %s failed at %s: %v", image, s.name, err)
	}
	if len(pids) == 0 {
		return nil
	}
	return fmt.Errorf("run: refused at %s: %s is running as pid %s, stop the process or defer this build, never replace a running binary", s.name, image, strings.Join(pids, ","))
}
