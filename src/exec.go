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
	status syscall.WaitStatus
	rusage syscall.Rusage
}

func (task *RunningTask) Run() {
	task.pid = runProcess()
	logrus.Debugf("child pid is %d", task.pid)

	task.trace()
}
func (task *RunningTask) trace() {
	task.Exit = true
	task.prevRax = 0

	for {
		failed := task.wait()
		if failed {
			break
		}

		if task.Exited() {
			break
		}
		if !task.trapped() {
			logrus.Infoln("Signal by: ", task.status.StopSignal())

			if task.gotTerminate() {
				logrus.Debugf("\n%#v\n", task.rusage)
				task.kill(syscall.SIGKILL)
				break
			} else {
				logrus.Infof("get signal, send SIGTERM")
				task.kill(syscall.SIGTERM)
			}
			continue
		}

		if task.detectRegs() {
			break
		}

		task.Exit = !task.Exit

		task.Continue()
	}
}

func (task *RunningTask) wait() bool {
	pid1, err := syscall.Wait4(-1, &task.status, 0, &task.rusage)
	if pid1 == 0 {
		logrus.Debugf("not found process")
		return true
	}
	checkPanic(err)
	return false
}

func (task *RunningTask) Continue() {
	err := syscall.PtraceSyscall(task.pid, 0)
	if err != nil {
		logrus.Infof("err is %v", err)
	}
}

func (task *RunningTask) kill(signal syscall.Signal) {
	syscall.Kill(task.pid, signal)
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
			logrus.Infof(">> %16v", getName(regs.Orig_rax))
		}
		logrus.Infof(">> %16v ", regs.Orig_rax)
	} else {
		logrus.Infof(">> %16v ", getName(regs.Orig_rax))
	}
	task.prevRax = regs.Orig_rax
	if task.prevRax == syscall.SYS_EXIT_GROUP {
		logrus.Infof("SYS_EXIT_GROUP")
	}
	return false
}
