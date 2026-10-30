//go:build windows

package server

import (
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestHostAvailMemReportsTheLiveStatus(t *testing.T) {
	avail, err := hostAvailMem()
	if err != nil {
		t.Fatalf("live GlobalMemoryStatusEx: %v", err)
	}
	if avail == 0 {
		t.Fatal("live available physical memory = 0, want a positive reading")
	}
}

func TestHostAvailMemSurfacesTheFailedStatusCall(t *testing.T) {
	saved := globalMemoryStatusEx
	globalMemoryStatusEx = windows.NewLazySystemDLL("kernel32.dll").NewProc("IsDebuggerPresent")
	t.Cleanup(func() { globalMemoryStatusEx = saved })
	avail, err := hostAvailMem()
	if err == nil {
		t.Fatal("failed status call err = nil, want the wrapped call error")
	}
	if !strings.Contains(err.Error(), "GlobalMemoryStatusEx") {
		t.Errorf("failed call error = %q, want the api named", err.Error())
	}
	if avail != 0 {
		t.Errorf("failed call avail = %d, want 0", avail)
	}
}
