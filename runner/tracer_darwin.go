//go:build darwin

package runner

func (tracer *TracerDetect) checkSyscall(pid int) syscallCheckResult {
	panic("checkSyscall is not supported on darwin")
}

func (tracer *TracerDetect) checkSeccompTrace(pid int) syscallCheckResult {
	panic("checkSeccompTrace is not supported on darwin")
}
