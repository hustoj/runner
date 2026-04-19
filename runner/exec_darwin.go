//go:build darwin

package runner

func (task *RunningTask) runProcess() bool {
	panic("runProcess is not supported on darwin")
}
