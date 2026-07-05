//go:build linux && amd64

package runner

import "syscall"

func setTestSyscallNumber(regs *syscall.PtraceRegs, sysno uint64) {
	regs.Orig_rax = sysno
}
