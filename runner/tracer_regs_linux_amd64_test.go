//go:build linux && amd64

package runner

import "syscall"

func setTestSyscallNumber(regs *syscall.PtraceRegs, sysno uint64) {
	regs.Orig_rax = sysno
}

func setTestSyscallArgument(regs *syscall.PtraceRegs, index int, value uint64) {
	switch index {
	case 0:
		regs.Rdi = value
	case 1:
		regs.Rsi = value
	case 2:
		regs.Rdx = value
	case 3:
		regs.R10 = value
	case 4:
		regs.R8 = value
	case 5:
		regs.R9 = value
	}
}
