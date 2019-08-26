package runner

import (
	"syscall"
)

type Process struct {
	Pid      int
	Status   syscall.WaitStatus
	Rusage   syscall.Rusage
	IsKilled bool
}

func (process *Process) Wait() {
	if process.IsKilled {
		return
	}
	pid1, err := syscall.Wait4(process.Pid, &process.Status, 0, &process.Rusage)
	if pid1 == 0 {
		log.Panic("not found process")
	}
	checkErr(err)
}

func (process *Process) Broken() bool {
	if !process.Trapped() {
		log.Debugf("Signal by: %v", process.Status.StopSignal())
		return true
	}
	return false
}

func (process *Process) Trapped() bool {
	return process.Status.StopSignal() == syscall.SIGTRAP
}

func (process *Process) Memory() int64 {
	return process.Rusage.Maxrss
}

func (process *Process) Exited() bool {
	if process.IsKilled {
		return true
	}
	if process.Status.Exited() {
		log.Debugf("Exited: %#v", process.Rusage)
		return true
	}
	return false
}

func (process *Process) GetTimeCost() int64 {
	ru := process.Rusage

	uSec := ru.Utime.Usec + ru.Stime.Usec

	return uSec + (ru.Utime.Sec+ru.Stime.Sec)*1e6
}

func (process *Process) Kill() {
	if process.IsKilled {
		return
	}
	log.Debugf("kill, %#v", process.Rusage)
	process.IsKilled = true
	syscall.Kill(process.Pid, syscall.SIGKILL)
}

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
