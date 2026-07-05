//go:build linux

package runner

import (
	"syscall"

	"golang.org/x/sys/unix"
)

const syscallStopSignal = syscall.Signal(syscall.SIGTRAP | 0x80)

const ptraceOExitKill = 0x00100000
const ptraceForkTraceEvents = syscall.PTRACE_O_TRACECLONE | syscall.PTRACE_O_TRACEFORK | syscall.PTRACE_O_TRACEVFORK
const (
	ptraceEventFork    = syscall.PTRACE_EVENT_FORK
	ptraceEventVFork   = syscall.PTRACE_EVENT_VFORK
	ptraceEventClone   = syscall.PTRACE_EVENT_CLONE
	ptraceEventSeccomp = unix.PTRACE_EVENT_SECCOMP
)

func waitOptions() int {
	return syscall.WALL
}

func (process *Process) Continue() bool {
	return process.ContinueWithSignal(0)
}

func (process *Process) ContinueWithSignal(sig int) bool {
	if process.IsKilled {
		return false
	}
	err := syscall.PtraceSyscall(process.CurrentPid, sig)
	if err != nil {
		log.Infof("PtraceSyscall(sig=%d): err %v", sig, err)
		return false
	}
	return true
}

func ptraceCont(pid int, sig int) error {
	return syscall.PtraceCont(pid, sig)
}

func (process *Process) IsInitialTraceStop() bool {
	return process.Status.Stopped() && process.Status.StopSignal() == syscall.SIGTRAP && process.Status.TrapCause() == 0
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

func (process *Process) SetPtraceOptions(traceSeccomp bool) error {
	return setPtraceOptions(process.CurrentPid, traceSeccomp)
}

func setPtraceOptions(pid int, traceSeccomp bool) error {
	options := syscall.PTRACE_O_TRACESYSGOOD | ptraceOExitKill | ptraceForkTraceEvents
	if traceSeccomp {
		options |= unix.PTRACE_O_TRACESECCOMP
	}
	return syscall.PtraceSetOptions(pid, options)
}
