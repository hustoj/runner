//go:build darwin

package runner

func (tracer *TracerDetect) checkSyscall() bool {
	// Ptrace syscall inspection is not supported on Darwin in this manner
	return false
}
