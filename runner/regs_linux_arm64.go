//go:build linux && arm64

package runner

import "syscall"

// getSyscallNumber extracts the syscall number from ptrace registers.
// On ARM64, the syscall number is in register X8 (Regs[8]).
func getSyscallNumber(regs *syscall.PtraceRegs) uint64 {
	return regs.Regs[8]
}

// getSyscallReturn extracts the syscall return value from ptrace registers.
// On ARM64, the return value is in register X0 (Regs[0]).
func getSyscallReturn(regs *syscall.PtraceRegs) uint64 {
	return regs.Regs[0]
}
