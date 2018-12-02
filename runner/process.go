package runner

import (
	"syscall"
)

type Process struct {
	Pid    int
	Status syscall.WaitStatus
	Rusage syscall.Rusage
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
	checkPanic(err)
}

func (process *Process) Broken() bool {
	if !process.Trapped() {
		log.Debugln("Signal by: ", process.Status.StopSignal())
		return true
	}
	return false
}

func (process *Process) Trapped() bool {
	return process.Status.StopSignal() == syscall.SIGTRAP
}

func (process *Process) Exited() bool {
	if process.IsKilled {
		return true
	}
	if process.Status.Exited() {
		log.Debugf("Exited: %#v\n", process.Rusage)
		return true
	}
	return false
}

func (process *Process) GetTimeCost() int64 {
	total := process.Rusage.Utime.Nano() + process.Rusage.Stime.Nano()

	return total / 1000 / 1000
}

func (process *Process) Kill() {
	log.Debugf("kill, %#v\n", process.Rusage)
	process.IsKilled = true
	syscall.Kill(process.Pid, syscall.SIGKILL)
}

func (process *Process) Continue() {
	if process.IsKilled {
		return
	}
	err := syscall.PtraceSyscall(process.Pid, 0)
	if err != nil {
		log.Debugf("PtraceSyscall: err %v\n", err)
	}
}
