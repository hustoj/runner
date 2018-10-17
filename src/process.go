package runner

import (
	"github.com/sirupsen/logrus"
	"syscall"
)

type Process struct {
	Pid    int
	Status syscall.WaitStatus
	Rusage syscall.Rusage
}

func (process *Process) Wait() {
	pid1, err := syscall.Wait4(process.Pid, &process.Status, 0, &process.Rusage)
	if pid1 == 0 {
		logrus.Panic("not found process")
	}
	checkPanic(err)
}

func (process *Process) Broken() bool {
	if !process.Trapped() {
		logrus.Debugln("Signal by: ", process.Status.StopSignal())
		return true
	}
	return false
}

func (process *Process) Trapped() bool {
	return process.Status.StopSignal() == syscall.SIGTRAP
}

func (process *Process) Exited() bool {
	if process.Status.Exited() {
		logrus.Infof("Exited: %#v\n", process.Rusage)
		return true
	}
	return false
}

func (process *Process) GetTimeCost() int64 {
	total := process.Rusage.Utime.Sec*1000 + int64(process.Rusage.Utime.Usec/1000)
	total = total + process.Rusage.Stime.Sec*1000 + int64(process.Rusage.Utime.Usec/1000)

	return total
}

func (process *Process) Kill() {
	logrus.Debugf("\n%#v\n", process.Rusage)
	syscall.Kill(process.Pid, syscall.SIGKILL)
}

func (process *Process) Continue() {
	err := syscall.PtraceSyscall(process.Pid, 0)
	if err != nil {
		logrus.Infof("PtraceSyscall: err %v", err)
	}
}
