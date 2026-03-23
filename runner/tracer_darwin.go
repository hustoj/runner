//go:build darwin

package runner

func (_ *TracerDetect) checkSyscall() bool {
	panicDarwinDevelopmentOnly("TracerDetect.checkSyscall")
	return false
}
