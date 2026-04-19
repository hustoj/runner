//go:build linux && amd64

package runner

import "syscall"

func getSyscallNumber(regs *syscall.PtraceRegs) uint64 {
	return regs.Orig_rax
}

func getSyscallReturn(regs *syscall.PtraceRegs) uint64 {
	return regs.Rax
}
