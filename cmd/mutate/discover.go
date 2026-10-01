package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

type target struct {
	pkg   string
	dir   string
	files []string
}

func discover(ctx context.Context, workDir string, patterns []string) ([]target, error) {
	cmd := exec.CommandContext(ctx, "go", append([]string{"list", "-e", "-json"}, patterns...)...)
	cmd.Dir = workDir
	cmd.Env = goEnv()
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("go list %s: %s", strings.Join(patterns, ","), ee.Stderr)
		}
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	var targets []target
	seen := map[string]bool{}
	for dec.More() {
		var p struct {
			ImportPath string
			Dir        string
			GoFiles    []string
			CgoFiles   []string
			Error      *struct {
				Err string
			}
		}
		if err := dec.Decode(&p); err != nil {
			return nil, err
		}
		if p.Error != nil {
			return nil, fmt.Errorf("go list %s: %s", p.ImportPath, p.Error.Err)
		}
		if seen[p.ImportPath] {
			continue
		}
		seen[p.ImportPath] = true
		files := append(append([]string{}, p.GoFiles...), p.CgoFiles...)
		sort.Strings(files)
		targets = append(targets, target{pkg: p.ImportPath, dir: p.Dir, files: files})
	}
	return targets, nil
}
