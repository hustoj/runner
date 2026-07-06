//go:build linux && arm64

package runner

import "syscall"

func getSyscallNumber(regs *syscall.PtraceRegs) uint64 {
	return regs.Regs[8]
}

func getSyscallReturn(regs *syscall.PtraceRegs) uint64 {
	return regs.Regs[0]
}

func getSyscallArgument(regs *syscall.PtraceRegs, index int) uint64 {
	if index < 0 || index >= 6 {
		return 0
	}
	return regs.Regs[index]
}
