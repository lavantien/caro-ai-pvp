package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestSweepHelperProcess(t *testing.T) {
	if os.Getenv("CARO_MUTATE_SWEEP_HELPER") == "" {
		return
	}
	var b [1]byte
	_, _ = os.Stdin.Read(b[:])
}

func startLiveOwner(t *testing.T) (pid int, stop func()) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestSweepHelperProcess", "-test.timeout=60s")
	cmd.Env = append(os.Environ(), "CARO_MUTATE_SWEEP_HELPER=1")
	cmd.Stdout = nil
	cmd.Stderr = nil
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("helper stdin pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = cmd.Wait()
	})
	return cmd.Process.Pid, func() {
		_ = stdin.Close()
		_ = cmd.Wait()
	}
}

func TestSweepSparesLiveOwnerTrees(t *testing.T) {
	root := t.TempDir()
	pid, stop := startLiveOwner(t)
	tag := "p" + strconv.Itoa(pid)
	live := []string{
		filepath.Join(root, "caro-mutate-"+tag+"-1"),
		filepath.Join(root, "caro-mutate-cache-"+tag+"-2"),
		filepath.Join(root, "caro-mutate-tmp-"+tag+"-3"),
	}
	for _, p := range live {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	stale := filepath.Join(root, "caro-mutate-123")
	if err := os.Mkdir(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	sweepStaleTemp(root)
	for _, p := range live {
		if _, serr := os.Stat(p); serr != nil {
			t.Errorf("sweep deleted a live mutate's tree %s: %v", p, serr)
		}
	}
	if _, serr := os.Stat(stale); !os.IsNotExist(serr) {
		t.Errorf("sweep left the unowned stale tree behind: %v", serr)
	}
	stop()
	dead := filepath.Join(root, "caro-mutate-"+tag+"-4")
	if err := os.Mkdir(dead, 0o755); err != nil {
		t.Fatal(err)
	}
	sweepStaleTemp(root)
	if _, serr := os.Stat(dead); !os.IsNotExist(serr) {
		t.Errorf("sweep spared a dead owner's tree %s: %v", dead, serr)
	}
}

func TestResidueCheckSparesLiveOwnerTrees(t *testing.T) {
	root := t.TempDir()
	pid, _ := startLiveOwner(t)
	live := filepath.Join(root, "caro-mutate-cache-p"+strconv.Itoa(pid)+"-1")
	if err := os.Mkdir(live, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := residueCheck(root); err != nil {
		t.Errorf("residueCheck(live foreign owner) = %v, want nil", err)
	}
	if _, serr := os.Stat(live); serr != nil {
		t.Errorf("residueCheck deleted a live mutate's cache %s: %v", live, serr)
	}
}

func TestNewRunDirsCarryOwnerTag(t *testing.T) {
	cache, err := newRunCache()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(cache) })
	if name := filepath.Base(cache); !strings.HasPrefix(name, "caro-mutate-cache-"+procTag()+"-") {
		t.Errorf("cache dir %q lacks the %q owner tag", name, "caro-mutate-cache-"+procTag()+"-")
	}
	tmp, err := newRunTmp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })
	if name := filepath.Base(tmp); !strings.HasPrefix(name, "caro-mutate-tmp-"+procTag()+"-") {
		t.Errorf("tmp dir %q lacks the %q owner tag", name, "caro-mutate-tmp-"+procTag()+"-")
	}
	iso, err := isolateModule(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(iso) })
	if name := filepath.Base(iso); !strings.HasPrefix(name, "caro-mutate-"+procTag()+"-") {
		t.Errorf("isolate dir %q lacks the %q owner tag", name, "caro-mutate-"+procTag()+"-")
	}
}

func TestOwnedByLiveMutateRejectsForeignFormats(t *testing.T) {
	own := "caro-mutate-p" + strconv.Itoa(os.Getpid()) + "-1"
	for name, want := range map[string]bool{
		"caro-mutate-123":                     false,
		"caro-mutate-cache-999":               false,
		"caro-mutate-tmp-999":                 false,
		own:                                   false,
		"caro-mutate-p0-1":                    false,
		"caro-mutate-p-1":                     false,
		"caro-mutate-p4x-1":                   false,
		"caro-mutate-p99999999999999999999-1": false,
		"caro-mutate-cache-p-1":               false,
		"unrelated-1":                         false,
	} {
		if got := ownedByLiveMutate(name); got != want {
			t.Errorf("ownedByLiveMutate(%q) = %v, want %v", name, got, want)
		}
	}
}
