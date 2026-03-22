//go:build linux

package runner

import (
	"syscall"
)

const syscallStopSignal = syscall.Signal(syscall.SIGTRAP | 0x80)

const ptraceOExitKill = 0x00100000

func (process *Process) Continue() bool {
	if process.IsKilled {
		return false
	}
	err := syscall.PtraceSyscall(process.Pid, 0)
	if err != nil {
		log.Infof("PtraceSyscall: err %v", err)
		return false
	}
	return true
}

func (process *Process) IsInitialTraceStop() bool {
	return process.Status.Stopped() && process.Status.StopSignal() == syscall.SIGTRAP
}

func (process *Process) IsSyscallStop() bool {
	return process.Status.Stopped() && process.Status.StopSignal() == syscallStopSignal
}

func (process *Process) SetPtraceOptions() error {
	return syscall.PtraceSetOptions(process.Pid, syscall.PTRACE_O_TRACESYSGOOD|ptraceOExitKill)
}
