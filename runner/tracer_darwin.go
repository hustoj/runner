//go:build darwin

package runner

func (tracer *TracerDetect) checkSyscall() bool {
	panic("checkSyscall is not supported on darwin")
}
