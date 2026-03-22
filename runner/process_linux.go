//go:build linux

package runner

import (
	"syscall"
)

func (process *Process) Continue() bool {
	if process.IsKilled {
		return false
	}
	err := syscall.PtraceSyscall(process.Pid, 0)
	if err != nil {
		log.Infof("PtraceSyscall: err %v", err)
		return false
	}
	return true
}
