//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

func cappedArgv(argv []string, _ int64) []string {
	return argv
}

func capStartedTree(cmd *exec.Cmd, capBytes int64) (release func(), err error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create job object: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_JOB_MEMORY | windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
		JobMemoryLimit: uintptr(capBytes),
	}
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("set job memory limit %d: %w", capBytes, err)
	}
	proc, perr := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if perr != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("open pid %d: %w", cmd.Process.Pid, perr)
	}
	if err = windows.AssignProcessToJobObject(job, proc); err != nil {
		_ = windows.CloseHandle(proc)
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("assign pid %d to job: %w", cmd.Process.Pid, err)
	}
	_ = windows.CloseHandle(proc)
	return func() { _ = windows.CloseHandle(job) }, nil
}
