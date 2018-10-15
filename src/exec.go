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
	pid			int
	Stop		bool
}

func (task *RunningTask) Run() {
	var status syscall.WaitStatus
	var rusage syscall.Rusage
	var err error

	task.Exit = true
	task.prevRax = 0

	task.pid = runProcess()
	logrus.Debugf("child pid is %d", task.pid)

	for {
		var pid1 int
		pid1, err = syscall.Wait4(task.pid, &status, 0, &rusage)
		if pid1 == 0 {
			logrus.Debugf("not found process")
			break
		}
		checkPanic(err)

		if status.Exited() {
			logrus.Infof("Exited: %#v\n", rusage)
			break
		}
		if status.StopSignal() != syscall.SIGTRAP {
			logrus.Infoln("Signal: ", status.StopSignal())
			if status.StopSignal() != syscall.SIGTERM {
				logrus.Infof("get signal, send SIGTERM")
				syscall.Kill(task.pid, syscall.SIGTERM)
			} else {
				logrus.Debugf("\n%#v\n", rusage)
				syscall.Kill(task.pid, syscall.SIGKILL)
				break
			}
			continue
		}
		task.DetectRegs()
		if task.Stop {
			break
		}
		task.Exit = !task.Exit

		err = syscall.PtraceSyscall(task.pid, 0)
		if err != nil {
			logrus.Infof("err is %v", err)
		}
	}
}

func (task *RunningTask) DetectRegs() {
	var regs syscall.PtraceRegs
	err := syscall.PtraceGetRegs(task.pid, &regs)
	if task.Exit {
		if err != nil {
			task.Stop = true
			return
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
}
