//go:build windows

package main

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func TestMemcapRunawayHelper(t *testing.T) {
	if os.Getenv("CARO_MUTATE_MEMCAP_HELPER") == "" {
		return
	}
	buf := make([]byte, 512<<20)
	for i := range buf {
		buf[i] = 1
	}
	os.Stderr.Write(buf[:1])
}

func TestTreeCapKillsRunaway(t *testing.T) {
	if testing.Short() {
		t.Fatal("short mode must not skip the memory cap proof")
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestMemcapRunawayHelper", "-test.timeout=90s")
	cmd.Env = append(os.Environ(), "CARO_MUTATE_MEMCAP_HELPER=1")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start err = %v", err)
	}
	release, err := capStartedTree(cmd, 64<<20)
	if err != nil {
		t.Fatalf("capStartedTree err = %v", err)
	}
	defer release()
	start := time.Now()
	werr := cmd.Wait()
	if werr == nil {
		t.Fatal("runaway helper exited 0, want the tree cap to kill it")
	}
	if elapsed := time.Since(start); elapsed > 60*time.Second {
		t.Errorf("runaway took %v to die, want the cap to fail allocations immediately", elapsed)
	}
	if _, ok := werr.(*exec.ExitError); !ok {
		t.Errorf("Wait err = %v, want ExitError from the killed helper", werr)
	}
}

func TestTreeCapAttachesLiveJob(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start err = %v", err)
	}
	release, err := capStartedTree(cmd, config.MutateTreeMemCapBytes)
	if err != nil {
		t.Fatalf("capStartedTree err = %v", err)
	}
	werr := cmd.Wait()
	release()
	if werr != nil {
		t.Errorf("capped benign child Wait err = %v, want nil", werr)
	}
}
