//go:build darwin

package runner

func (tracer *TracerDetect) checkSyscall(pid int) syscallCheckResult {
	panic("checkSyscall is not supported on darwin")
}
