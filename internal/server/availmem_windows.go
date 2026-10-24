//go:build windows

package server

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

type memStatusEx struct {
	length               uint32
	memoryLoad           uint32
	totalPhys            uint64
	availPhys            uint64
	totalPageFile        uint64
	availPageFile        uint64
	totalVirtual         uint64
	availVirtual         uint64
	availExtendedVirtual uint64
}

var globalMemoryStatusEx = windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")

func hostAvailMem() (uint64, error) {
	status := memStatusEx{length: uint32(unsafe.Sizeof(memStatusEx{}))}
	ret, _, callErr := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status)))
	if ret == 0 {
		return 0, fmt.Errorf("GlobalMemoryStatusEx: %w", callErr)
	}
	return status.availPhys, nil
}
