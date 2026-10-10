package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, " ")
}

func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

func main() {
	dirs := &multiFlag{}
	out := flag.String("out", "", "write the markdown report here instead of stdout")
	flag.Var(dirs, "dir", "tournament run directory, repeatable")
	flag.Parse()
	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "ponderscan: "+format+"\n", args...)
		os.Exit(2)
	}
	if len(*dirs) == 0 {
		fail("no --dir given")
	}
	var report strings.Builder
	report.WriteString("# ponder scan\n\n")
	for _, dir := range *dirs {
		s, err := scanDir(dir)
		if err != nil {
			fail("%v", err)
		}
		report.WriteString(renderScan(s))
	}
	if *out == "" {
		fmt.Print(report.String())
		return
	}
	if err := os.WriteFile(*out, []byte(report.String()), 0o644); err != nil {
		fail("write %s: %v", *out, err)
	}
}
