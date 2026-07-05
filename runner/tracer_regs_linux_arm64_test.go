//go:build linux && arm64

package runner

import "syscall"

func setTestSyscallNumber(regs *syscall.PtraceRegs, sysno uint64) {
	regs.Regs[8] = sysno
}
