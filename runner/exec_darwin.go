//go:build darwin

package runner

func (task *RunningTask) runProcess() {
	panic("runProcess is not supported on darwin")
}
