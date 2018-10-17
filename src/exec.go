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
			task.GetResult()
			break
		}
		if process.StoppedBySignal() {
			// if receive term then stop
			if process.GotTermSignal() {
				process.Kill()
				break
			}

			// other signal, send kill to process
			process.Kill()
			break
		}

		if tracer.detect() {
			break
		}
		// before next ptrace,get result, always pass
		task.GetResult()
	}
	logrus.Infof("Time: %d, Memory: %dkb", task.Time, task.Memory)
}

func (task *RunningTask) GetResult()  {
	task.Time = task.process.GetTimeCost()
	memory, err := GetProcMemory(task.process.Pid)
	if err != nil {
		logrus.Infoln("Get memory failed:", err)
		return
	}
	task.Memory = memory
}