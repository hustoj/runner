//go:build linux && arm64

package runner

import "syscall"

func setTestSyscallNumber(regs *syscall.PtraceRegs, sysno uint64) {
	regs.Regs[8] = sysno
}

func setTestSyscallArgument(regs *syscall.PtraceRegs, index int, value uint64) {
	regs.Regs[index] = value
}
