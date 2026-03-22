//go:build linux

package runner

import (
	"syscall"
)

func ptraceTraceme() {
	syscall.Syscall(syscall.SYS_PTRACE, syscall.PTRACE_TRACEME, 0, 0)
}

func setAlarm(seconds uint64) {
	syscall.Syscall(syscall.SYS_ALARM, uintptr(seconds), 0, 0)
}
