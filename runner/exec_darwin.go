//go:build darwin

package runner

func (task *RunningTask) runProcess() error {
	return darwinDevelopmentOnlyError("runProcess")
}
