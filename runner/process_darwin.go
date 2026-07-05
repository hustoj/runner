//go:build darwin

package runner

import "errors"

const (
	ptraceEventFork    = 1
	ptraceEventVFork   = 2
	ptraceEventClone   = 3
	ptraceEventSeccomp = 4
)

func waitOptions() int {
	return 0
}

func (process *Process) Continue() bool {
	panic("Process.Continue is not supported on darwin")
}

func (process *Process) ContinueWithSignal(_ int) bool {
	panic("Process.ContinueWithSignal is not supported on darwin")
}

func (process *Process) ContinueWithMode(_ traceResumeMode, _ int) bool {
	panic("Process.ContinueWithMode is not supported on darwin")
}

func (process *Process) IsInitialTraceStop() bool {
	panic("Process.IsInitialTraceStop is not supported on darwin")
}

func (process *Process) IsSyscallStop() bool {
	panic("Process.IsSyscallStop is not supported on darwin")
}

func (process *Process) IsPtraceEventStop() bool {
	panic("Process.IsPtraceEventStop is not supported on darwin")
}

func (process *Process) PtraceEvent() int {
	panic("Process.PtraceEvent is not supported on darwin")
}

func (process *Process) GetEventPid() (int, error) {
	return 0, errors.New("Process.GetEventPid is not supported on darwin")
}

func (process *Process) SetPtraceOptions(_ bool) error {
	return errors.New("Process.SetPtraceOptions is not supported on darwin")
}
