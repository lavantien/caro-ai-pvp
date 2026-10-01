package main

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func TestPortsOutput(t *testing.T) {
	want := "http=" + strconv.Itoa(config.HTTPPort) + "\ndebug=" + strconv.Itoa(config.DebugPort) + "\n"
	if got := portsOutput(); got != want {
		t.Errorf("portsOutput = %q, want %q", got, want)
	}
}

func TestFirewallArgs(t *testing.T) {
	base := firewallArgs(false)
	joined := strings.Join(base, " ")
	for _, want := range []string{firewallScript, "-Ports", "-ExecutionPolicy Bypass"} {
		if !strings.Contains(joined, want) {
			t.Errorf("firewallArgs missing %q in %v", want, base)
		}
	}
	if !strings.Contains(joined, portsFlag()) {
		t.Errorf("firewallArgs missing configured ports in %v", base)
	}
	if strings.Contains(joined, "-Remove") {
		t.Errorf("firewallArgs(false) must not pass -Remove: %v", base)
	}
	withRemove := firewallArgs(true)
	if withRemove[len(withRemove)-1] != "-Remove" {
		t.Errorf("firewallArgs(true) = %v, want -Remove last", withRemove)
	}
}

func TestRunSubcommands(t *testing.T) {
	if code := run([]string{"ports"}); code != 0 {
		t.Errorf("ports exit = %d, want 0", code)
	}
	if code := run(nil); code != 2 {
		t.Errorf("no args exit = %d, want 2", code)
	}
	if code := run([]string{"bogus"}); code != 2 {
		t.Errorf("bogus exit = %d, want 2", code)
	}
}

func TestRunFirewallInvokesPwsh(t *testing.T) {
	name, args := captureExec(t, nil)
	if code := run([]string{"firewall"}); code != 0 {
		t.Fatalf("firewall exit = %d, want 0", code)
	}
	if *name != "pwsh.exe" {
		t.Errorf("invoked %q, want pwsh.exe", *name)
	}
	if !strings.Contains(strings.Join(*args, " "), "-Ports "+portsFlag()) {
		t.Errorf("args %v missing configured ports", *args)
	}
}

func TestRunFirewallRemove(t *testing.T) {
	_, args := captureExec(t, nil)
	if code := run([]string{"firewall", "-remove"}); code != 0 {
		t.Fatalf("firewall -remove exit = %d, want 0", code)
	}
	if got := *args; got[len(got)-1] != "-Remove" {
		t.Errorf("args %v want -Remove last", got)
	}
}

func TestRunFirewallFailures(t *testing.T) {
	captureExec(t, errors.New("must not run"))
	if code := run([]string{"firewall", "-nope"}); code != 1 {
		t.Errorf("bad flag exit = %d, want 1", code)
	}
	captureExec(t, errors.New("pwsh failed"))
	if code := run([]string{"firewall"}); code != 1 {
		t.Errorf("exec failure exit = %d, want 1", code)
	}
}

func TestExecRunner(t *testing.T) {
	if err := execRunner("go", "version"); err != nil {
		t.Errorf("execRunner(go version) = %v, want nil", err)
	}
}

func captureExec(t *testing.T, err error) (*string, *[]string) {
	t.Helper()
	var name string
	var args []string
	orig := execRunner
	execRunner = func(n string, a ...string) error {
		name, args = n, a
		return err
	}
	t.Cleanup(func() { execRunner = orig })
	return &name, &args
}

func portsFlag() string {
	return strconv.Itoa(config.HTTPPort) + "," + strconv.Itoa(config.DebugPort)
}
