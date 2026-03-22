//go:build darwin

package runner

import "errors"

func (process *Process) Continue() bool {
	panic("Process.Continue is not supported on darwin")
}

func (process *Process) IsInitialTraceStop() bool {
	panic("Process.IsInitialTraceStop is not supported on darwin")
}

func (process *Process) IsSyscallStop() bool {
	panic("Process.IsSyscallStop is not supported on darwin")
}

func (process *Process) SetPtraceOptions() error {
	return errors.New("Process.SetPtraceOptions is not supported on darwin")
}
