//go:build darwin

package runner

func (_ *Process) Continue() bool {
	panicDarwinDevelopmentOnly("Process.Continue")
	return false
}

func (_ *Process) IsInitialTraceStop() bool {
	panicDarwinDevelopmentOnly("Process.IsInitialTraceStop")
	return false
}

func (_ *Process) IsSyscallStop() bool {
	panicDarwinDevelopmentOnly("Process.IsSyscallStop")
	return false
}

func (_ *Process) SetPtraceOptions() error {
	return darwinDevelopmentOnlyError("Process.SetPtraceOptions")
}
