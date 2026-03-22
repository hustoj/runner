//go:build darwin

package runner

func ptraceTraceme() {
	panic("ptraceTraceme is not supported on darwin")
}

func setAlarm(seconds uint64) {
	panic("setAlarm is not supported on darwin")
}

func (task *RunningTask) runProcess() {
	panic("runProcess is not supported on darwin")
}
