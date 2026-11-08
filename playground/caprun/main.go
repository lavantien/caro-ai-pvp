package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func main() {
	capMiB := flag.Int64("cap", 2048, "job commit cap in MiB")
	killAfter := flag.Duration("kill-after", 5*time.Minute, "outer kill window")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: caprun [-cap MiB] [-kill-after d] command...")
		os.Exit(2)
	}
	argv := flag.Args()
	ctx, cancel := context.WithTimeout(context.Background(), *killAfter)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	start := time.Now()
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "caprun: start: %v\n", err)
		os.Exit(2)
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "caprun: create job: %v\n", err)
		os.Exit(2)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_JOB_MEMORY | windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
		JobMemoryLimit: uintptr(*capMiB << 20),
	}
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		fmt.Fprintf(os.Stderr, "caprun: set job limit: %v\n", err)
		os.Exit(2)
	}
	proc, perr := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if perr != nil {
		fmt.Fprintf(os.Stderr, "caprun: open pid: %v\n", perr)
		os.Exit(2)
	}
	if err = windows.AssignProcessToJobObject(job, proc); err != nil {
		fmt.Fprintf(os.Stderr, "caprun: assign pid: %v\n", err)
		os.Exit(2)
	}
	_ = windows.CloseHandle(proc)
	defer windows.CloseHandle(job)
	werr := cmd.Wait()
	elapsed := time.Since(start)
	fmt.Printf("caprun: exit=%v elapsed=%s ctx_err=%v output_bytes=%d\n", werr, elapsed.Round(time.Second), ctx.Err(), out.Len())
	if out.Len() > 0 {
		fmt.Printf("caprun: output tail:\n%s\n", tail(out.String(), 3000))
	}
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n:]
}
