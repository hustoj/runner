//go:build linux && amd64

package runner

import "syscall"

func getSyscallNumber(regs *syscall.PtraceRegs) uint64 {
	return regs.Orig_rax
}

func getSyscallReturn(regs *syscall.PtraceRegs) uint64 {
	return regs.Rax
}

func getSyscallArgument(regs *syscall.PtraceRegs, index int) uint64 {
	switch index {
	case 0:
		return regs.Rdi
	case 1:
		return regs.Rsi
	case 2:
		return regs.Rdx
	case 3:
		return regs.R10
	case 4:
		return regs.R8
	case 5:
		return regs.R9
	default:
		return 0
	}
}
