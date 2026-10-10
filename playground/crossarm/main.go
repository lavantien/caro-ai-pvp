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
	arms := &multiFlag{}
	focus := &multiFlag{}
	flag.Var(arms, "arm", "label=dir tournament run, repeatable")
	flag.Var(focus, "focus", "seat name whose losses and phase telemetry get rendered, repeatable")
	out := flag.String("out", "", "write the markdown report here instead of stdout")
	flag.Parse()
	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "crossarm: "+format+"\n", args...)
		os.Exit(2)
	}
	if len(*arms) == 0 {
		fail("no --arm given")
	}
	var report strings.Builder
	report.WriteString("# crossarm seat decomposition\n\n")
	loaded := make([]*armData, 0, len(*arms))
	for _, spec := range *arms {
		label, dir, ok := strings.Cut(spec, "=")
		if !ok || label == "" || dir == "" {
			fail("--arm wants label=dir, got %q", spec)
		}
		a, err := loadArm(label, dir)
		if err != nil {
			fail("%v", err)
		}
		loaded = append(loaded, a)
	}
	for _, a := range loaded {
		report.WriteString(renderArm(a, *focus))
	}
	report.WriteString(renderSynthesis(loaded))
	if *out == "" {
		fmt.Print(report.String())
		return
	}
	if err := os.WriteFile(*out, []byte(report.String()), 0o644); err != nil {
		fail("write %s: %v", *out, err)
	}
}
