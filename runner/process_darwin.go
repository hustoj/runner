//go:build darwin

package runner

func (process *Process) Continue() bool {
	panic("Process.Continue is not supported on darwin")
}
