package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

const firewallScript = "scripts/firewall.ps1"

var execRunner = func(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func portsOutput() string {
	return fmt.Sprintf("http=%d\ndebug=%d\n", config.HTTPPort, config.DebugPort)
}

func firewallArgs(remove bool) []string {
	args := []string{
		"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", firewallScript,
		"-Ports", fmt.Sprintf("%d,%d", config.HTTPPort, config.DebugPort),
	}
	if remove {
		args = append(args, "-Remove")
	}
	return args
}

func runFirewall(args []string) error {
	fs := flag.NewFlagSet("firewall", flag.ContinueOnError)
	remove := fs.Bool("remove", false, "remove the rules instead of adding them")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return execRunner("pwsh.exe", firewallArgs(*remove)...)
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "subcommands: ports, firewall")
		return 2
	}
	switch args[0] {
	case "ports":
		fmt.Print(portsOutput())
		return 0
	case "firewall":
		if err := runFirewall(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "caro:", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(os.Stderr, "caro: unknown subcommand %q\n", args[0])
		return 2
	}
}

func main() {
	os.Exit(run(os.Args[1:]))
}
