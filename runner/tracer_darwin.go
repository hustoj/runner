//go:build darwin

package runner

func (tracer *TracerDetect) checkSyscall(pid int) bool {
	panic("checkSyscall is not supported on darwin")
}
