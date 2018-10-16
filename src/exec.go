package runner

import (
	"github.com/sirupsen/logrus"
	"syscall"
)

type RunningTask struct {
	TimeLimit   int64
	MemoryLimit int64
	Memory      int64
	Time        int64
	Status      uint64
	Exit        bool
	prevRax     uint64
	pid         int
	status      syscall.WaitStatus
	rusage      syscall.Rusage
}

func (task *RunningTask) Run() {
	task.pid = runProcess()
	logrus.Debugf("child pid is %d", task.pid)

	task.trace()
}
func (task *RunningTask) trace() {
	task.Exit = true
	task.prevRax = 0

	for ; ; task.Continue() {
		task.wait()

		if task.Exited() {
			break
		}
		if !task.trapped() {
			logrus.Infoln("Signal by: ", task.status.StopSignal())

			if task.gotTerminate() {
				task.kill()
				break
			} else {
				task.terminate()
			}
			continue
		}

		if task.detectRegs() {
			break
		}

		task.Exit = !task.Exit
	}
}

func (task *RunningTask) wait() {
	pid1, err := syscall.Wait4(task.pid, &task.status, 0, &task.rusage)
	if pid1 == 0 {
		logrus.Panic("not found process")
	}
	checkPanic(err)
}

func (task *RunningTask) Continue() {
	err := syscall.PtraceSyscall(task.pid, 0)
	if err != nil {
		logrus.Infof("PtraceSyscall: err %v", err)
	}
}

func (task *RunningTask) gotTerminate() bool {
	return task.status.StopSignal() == syscall.SIGTERM
}

func (task *RunningTask) Exited() bool {
	if task.status.Exited() {
		logrus.Infof("Exited: %#v\n", task.rusage)
		return true
	}
	return false
}

func (task *RunningTask) trapped() bool {
	return task.status.StopSignal() == syscall.SIGTRAP
}

func (task *RunningTask) detectRegs() bool {
	var regs syscall.PtraceRegs
	err := syscall.PtraceGetRegs(task.pid, &regs)
	if task.Exit {
		if err != nil {
			return true
		}
		if task.prevRax != regs.Orig_rax {
			logrus.Infof(">>Name %16v", getName(regs.Orig_rax))
		}
		logrus.Infof(">>Value %40X", regs.Rax)
	} else {
		logrus.Infof(">>Name %16v", getName(regs.Orig_rax))
	}
	task.prevRax = regs.Orig_rax
	if task.prevRax == syscall.SYS_EXIT_GROUP {
		logrus.Infof("SYS_EXIT_GROUP")
	}
	return false
}

func (task *RunningTask) terminate() {
	logrus.Infof("get signal, send SIGTERM")
	syscall.Kill(task.pid, syscall.SIGTERM)
}

func (task *RunningTask) kill() {
	logrus.Debugf("\n%#v\n", task.rusage)
	syscall.Kill(task.pid, syscall.SIGKILL)
}
