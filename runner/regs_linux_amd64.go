//go:build linux && amd64

package runner

import "syscall"

// getSyscallNumber extracts the syscall number from ptrace registers.
// On x86_64, this is the Orig_rax register.
func getSyscallNumber(regs *syscall.PtraceRegs) uint64 {
	return regs.Orig_rax
}

// getSyscallReturn extracts the syscall return value from ptrace registers.
// On x86_64, this is the Rax register.
func getSyscallReturn(regs *syscall.PtraceRegs) uint64 {
	return regs.Rax
}
