package runner

import (
	"github.com/sirupsen/logrus"
)

type RunningTask struct {
	TimeLimit   int64
	MemoryLimit int64
	Memory      int64
	Time        int64
	Status      uint64
	process     Process
}

func (task *RunningTask) Run() {
	pid := runProcess()
	logrus.Debugf("child pid is %d", pid)

	task.process.Pid = pid

	task.trace()
}

func (task *RunningTask) trace() {
	process := &task.process

	tracer := TracerDetect{
		Pid:     task.process.Pid,
		Exit:    true,
		prevRax: 0,
	}

	for ; ; process.Continue() {
		process.Wait()

		if process.Exited() {
			break
		}
		if process.StoppedBySignal() {
			// if receive term then stop
			if process.GotTermSignal() {
				process.Kill()
				break
			}

			// other signal, send signal term to process
			process.Terminate()
			continue
		}

		if tracer.detect() {
			break
		}
	}
}
