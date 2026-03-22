//go:build darwin

package runner

func (process *Process) Continue() bool {
	// Ptrace is not fully supported on Darwin in this manner
	return false
}
