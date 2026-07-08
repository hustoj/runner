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

var (
	ptraceSyscallCall     = syscall.PtraceSyscall
	ptraceContCall        = syscall.PtraceCont
	ptraceGetEventMsgCall = syscall.PtraceGetEventMsg
	ptraceSetOptionsCall  = syscall.PtraceSetOptions
)

func (process *Process) Continue() bool {
	return process.ContinueWithSignal(0)
}

func (process *Process) ContinueWithSignal(sig int) bool {
	return process.ContinueWithMode(traceResumeSyscallStops, sig)
}

func (process *Process) ContinueWithMode(mode traceResumeMode, sig int) bool {
	if process.IsKilled {
		return false
	}
	err := resumeTracee(process.CurrentPid, mode, sig)
	if err != nil {
		log.Infof("ptrace resume mode=%d sig=%d: err %v", mode, sig, err)
		return false
	}
	return true
}

func resumeTracee(pid int, mode traceResumeMode, sig int) error {
	if mode == traceResumeEventStops {
		return ptraceContCall(pid, sig)
	}
	return ptraceSyscallCall(pid, sig)
}

func ptraceCont(pid int, sig int) error {
	return ptraceContCall(pid, sig)
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
	msg, err := ptraceGetEventMsgCall(process.CurrentPid)
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
	return ptraceSetOptionsCall(pid, options)
}
