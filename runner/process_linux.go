//go:build linux

package runner

import (
	"syscall"
)

const syscallStopSignal = syscall.Signal(syscall.SIGTRAP | 0x80)

const ptraceOExitKill = 0x00100000
const ptraceTraceEvents = syscall.PTRACE_O_TRACECLONE | syscall.PTRACE_O_TRACEFORK | syscall.PTRACE_O_TRACEVFORK
const (
	ptraceEventFork  = syscall.PTRACE_EVENT_FORK
	ptraceEventVFork = syscall.PTRACE_EVENT_VFORK
	ptraceEventClone = syscall.PTRACE_EVENT_CLONE
)

func waitOptions() int {
	return syscall.WALL
}

func (process *Process) Continue() bool {
	if process.IsKilled {
		return false
	}
	err := syscall.PtraceSyscall(process.CurrentPid, 0)
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

func (process *Process) IsPtraceEventStop() bool {
	return process.Status.Stopped() && process.Status.StopSignal() == syscall.SIGTRAP && process.Status.TrapCause() != 0
}

func (process *Process) PtraceEvent() int {
	return process.Status.TrapCause()
}

func (process *Process) GetEventPid() (int, error) {
	msg, err := syscall.PtraceGetEventMsg(process.CurrentPid)
	return int(msg), err
}

func (process *Process) SetPtraceOptions() error {
	return syscall.PtraceSetOptions(process.CurrentPid, syscall.PTRACE_O_TRACESYSGOOD|ptraceOExitKill|ptraceTraceEvents)
}
