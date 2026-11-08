//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

func capSupported() bool { return true }

func applyCap(cmd *exec.Cmd, capMiB int64) error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_JOB_MEMORY | windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
		JobMemoryLimit: uintptr(capMiB << 20),
	}
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		return fmt.Errorf("set job limit: %w", err)
	}
	proc, perr := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if perr != nil {
		return fmt.Errorf("open pid: %w", perr)
	}
	if err = windows.AssignProcessToJobObject(job, proc); err != nil {
		return fmt.Errorf("assign pid: %w", err)
	}
	_ = windows.CloseHandle(proc)
	keepJob = job
	return nil
}

var keepJob windows.Handle
