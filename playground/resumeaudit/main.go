package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type verdictAt struct {
	line int
	kind string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: resumeaudit path/to/mutate.log [more.log ...]")
		os.Exit(2)
	}
	for _, path := range os.Args[1:] {
		audit(path)
	}
}

func audit(path string) {
	f, err := os.Open(path)
	if err != nil {
		fmt.Println(path, "open err:", err)
		os.Exit(1)
	}
	defer f.Close()
	order := []string{}
	seen := map[string][]verdictAt{}
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		n++
		line := sc.Text()
		key, kind, ok := splitVerdict(line)
		if !ok {
			continue
		}
		if _, dup := seen[key]; !dup {
			order = append(order, key)
		}
		seen[key] = append(seen[key], verdictAt{line: n, kind: kind})
	}
	if err := sc.Err(); err != nil {
		fmt.Println(path, "scan err:", err)
		os.Exit(1)
	}
	killedSet := map[string]bool{}
	replayed := 0
	for _, key := range order {
		vs := seen[key]
		last := vs[len(vs)-1]
		anyKilled := false
		for _, v := range vs {
			if v.kind == "KILLED" {
				anyKilled = true
			}
		}
		if anyKilled {
			killedSet[key] = true
			replayed++
		}
		if len(vs) > 1 {
			fmt.Printf("%s: multi key %q:", path, key)
			for _, v := range vs {
				fmt.Printf(" %d=%s", v.line, v.kind)
			}
			if anyKilled && last.kind != "KILLED" {
				fmt.Printf("  REPLAY-CONFLICT: loadResumeLog replays KILLED, last verdict is %s", last.kind)
			}
			fmt.Println()
		}
	}
	fmt.Printf("%s: %d verdict lines, %d unique keys, resume replays %d (any-KILLED rule)\n", path, n, len(order), replayed)
}

func splitVerdict(line string) (string, string, bool) {
	for _, kind := range []string{"KILLED", "SURVIVED", "CHALLENGED", "RUN ERROR"} {
		if key, ok := strings.CutSuffix(line, " "+kind); ok && key != line {
			return key, kind, true
		}
	}
	if key, rest, ok := strings.Cut(line, " ALLOWED # "); ok && key != line && rest != "" {
		return key, "ALLOWED", true
	}
	return "", "", false
}
