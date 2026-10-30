//go:build windows

package main

import (
	"errors"

	"golang.org/x/sys/windows"
)

func pidAlive(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return errors.Is(err, windows.ERROR_ACCESS_DENIED)
	}
	defer windows.CloseHandle(h)
	event, werr := windows.WaitForSingleObject(h, 0)
	return werr != nil || event == uint32(windows.WAIT_TIMEOUT)
}
