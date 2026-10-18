//go:build !windows

package main

import (
	"reflect"
	"testing"
)

func TestUlimitArgvWrapsTestCommand(t *testing.T) {
	argv := []string{"go", "test", "-count=1", "./internal/server"}
	got := ulimitArgv(2*1024*1024, argv)
	want := []string{"sh", "-c", "ulimit -v 2097152; exec \"$@\"", "sh", "go", "test", "-count=1", "./internal/server"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ulimitArgv = %v, want %v", got, want)
	}
	if len(got) < 4 || got[3] == argv[0] {
		t.Errorf("argv[0] %q must not double as the sh -c $0 placeholder, exec \"$@\" would drop it", argv[0])
	}
}
