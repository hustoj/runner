//go:build linux && arm64

package runner

import "syscall"

func getSyscallNumber(regs *syscall.PtraceRegs) uint64 {
	return regs.Regs[8]
}

func getSyscallReturn(regs *syscall.PtraceRegs) uint64 {
	return regs.Regs[0]
}
