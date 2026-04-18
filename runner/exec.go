package runner

import (
	"os"
	"runtime"
	"syscall"
)

type RunningTask struct {
	setting     *TaskConfig
	process     *Process
	Result      *Result
	timeLimit   int64
	memoryLimit int64
}

func (task *RunningTask) Init(setting *TaskConfig) {
	task.setting = setting
	task.timeLimit = int64(setting.CPU) * 1e6
	task.memoryLimit = int64(setting.Memory) * 1024

	task.Result = &Result{}
	task.Result.Init()

	log.Debugf("load case config %#v", task.setting)
	log.Debugf("Time limit: %d, PeakMemory limit: %d", task.timeLimit, task.memoryLimit)
}

func (task *RunningTask) Run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// execute task
	task.runProcess()
	task.trace()
}

func (task *RunningTask) GetResult() *Result {
	log.Debugf(task.Result.String())

	return task.Result
}

func (task *RunningTask) trace() {
	process := task.process
	process.IsKilled = false

	tracer := TracerDetect{
		Pid:     task.process.Pid,
		prevRax: 0,
	}

	allowedCalls := make([]string, 0, len(task.setting.AllowedCalls)+len(task.setting.AdditionCalls))
	allowedCalls = append(allowedCalls, task.setting.AllowedCalls...)
	allowedCalls = append(allowedCalls, task.setting.AdditionCalls...)
	log.Debugf("allowed syscall is: %s", allowedCalls)
	tracer.setCallPolicy(makeCallPolicy(&task.setting.OneTimeCalls, &allowedCalls))
	tracer.consumeBootstrapCall(syscall.SYS_EXECVE)

	process.Wait()
	if process.Exited() {
		log.Infof("program exited before tracing loop")
		task.parseRunningInfo()
		task.applyExitCode(process.Status)
		task.check()
		return
	}
	if !process.IsInitialTraceStop() {
		task.handleBrokenTraceStop("unexpected initial ptrace stop")
		task.check()
		return
	}
	if err := process.SetPtraceOptions(); err != nil {
		log.Infof("PtraceSetOptions: err %v", err)
		task.Result.RetCode = RUNTIME_ERROR
		process.Kill()
		task.parseRunningInfo()
		task.check()
		return
	}
	if !process.Continue() {
		log.Infof("Program not alive after ptrace setup")
		task.parseRunningInfo()
		task.check()
		return
	}

	for {
		process.Wait()

		if process.Exited() {
			log.Infof("program exited! %v", process.Status.StopSignal())
			task.parseRunningInfo()
			task.applyExitCode(process.Status)
			break
		}
		if !process.IsSyscallStop() {
			task.handleBrokenTraceStop("unexpected non-syscall ptrace stop")
			break
		}

		if tracer.checkSyscall() {
			log.Debugf("------- check syscall failed")
			process.Kill()
			task.Result.RetCode = RUNTIME_ERROR
			break
		}
		// before next ptrace, get result, always pass
		task.parseRunningInfo()
		task.checkLimit()

		if !process.Continue() {
			log.Infof("Program not alive! break")
			break
		}

	}
	task.check()
}

func (task *RunningTask) handleBrokenTraceStop(reason string) {
	process := task.process
	log.Infof("%s: %v", reason, process.Status.StopSignal())
	task.parseRunningInfo()
	if process.Status.Stopped() {
		task.Result.detectSignal(process.Status.StopSignal())
	} else if process.Status.Signaled() {
		task.applyTerminationSignal(process.Status.Signal())
	} else {
		if task.outOfTime() {
			task.Result.RetCode = TIME_LIMIT
		} else if task.outOfMemory() {
			task.Result.RetCode = MEMORY_LIMIT
		} else {
			log.Warnf("process broken, but cause can't detect")
		}
	}

	log.Debugf("Process broken, will kill process")
	process.Kill()
}

func (task *RunningTask) applyExitCode(status syscall.WaitStatus) {
	if !status.Exited() || status.ExitStatus() == 0 || !task.Result.isAccept() {
		return
	}
	task.Result.RetCode = RUNTIME_ERROR
}

func (task *RunningTask) applyTerminationSignal(signal os.Signal) {
	if !task.Result.isAccept() {
		return
	}
	task.Result.detectSignal(signal)
}

func (task *RunningTask) check() {
	log.Debug(task.Result.String())
	if !task.Result.isAccept() {
		return
	}
	if task.outOfMemory() {
		task.Result.RetCode = MEMORY_LIMIT
	}
	if task.outOfTime() {
		task.Result.RetCode = TIME_LIMIT
	}
}

func (task *RunningTask) checkLimit() {
	if task.outOfTime() {
		task.Result.RetCode = TIME_LIMIT
		log.Debugf("kill by time limit: current %d, limit %d", task.Result.TimeCost, task.timeLimit)
		task.process.Kill()
		return
	}
	if task.outOfMemory() {
		task.Result.RetCode = MEMORY_LIMIT
		log.Debugf("kill by memory limit: peak %d, rusage: %d, limit %d", task.Result.PeakMemory, task.Result.RusageMemory, task.memoryLimit)
		task.process.Kill()
		return
	}
}

func (task *RunningTask) outOfTime() bool {
	isTLE := task.Result.TimeCost > task.timeLimit
	if isTLE {
		log.Infof("TLE: Time limit: %d, time coast: %d", task.timeLimit, task.Result.TimeCost)
	}
	return isTLE
}

func (task *RunningTask) outOfMemory() bool {
	// check memory is over limit
	isMLE := (task.Result.PeakMemory > task.memoryLimit) || (task.Result.RusageMemory > task.memoryLimit)
	if isMLE {
		log.Infof("MLE: Memory Limit: %d. Peak %d, Rusage %d.", task.memoryLimit, task.Result.PeakMemory, task.Result.RusageMemory)
	}

	return isMLE
}

func (task *RunningTask) refreshTimeCost() {
	task.Result.TimeCost = task.process.GetTimeCost()
	log.Debugf("current time cost: %dus(1e-6s)", task.Result.TimeCost)
}

func (task *RunningTask) refreshMemory() {
	memory, err := GetProcMemory(task.process.Pid)
	if err != nil {
		log.Infof("Get status memory failed: %v", err)
	} else {
		log.Debugf("peak memory is: %d", memory)
		if memory > task.Result.PeakMemory {
			task.Result.PeakMemory = memory
		}
	}

	memory = task.process.Memory()
	log.Debugf("rusage memory is: %d", memory)
	if memory > task.Result.RusageMemory {
		task.Result.RusageMemory = memory
	}
}

func (task *RunningTask) parseRunningInfo() {
	task.refreshTimeCost()
	task.refreshMemory()
}
