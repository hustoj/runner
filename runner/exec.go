package runner

import (
	"fmt"
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

func (task *RunningTask) runProcess() int {
	pid := fork()
	if pid < 0 {
		log.Panic("fork child failed")
	}
	if pid == 0 {
		// enter child
		task.limitResource()
		task.redirectIO()

		syscall.Syscall(syscall.SYS_PTRACE, syscall.PTRACE_TRACEME, 0, 0)

		env := os.Environ()
		err := syscall.Exec(task.setting.GetCommand(), task.setting.GetArgs(), env)
		if err != nil {
			panic(fmt.Sprintf("Exec failed: %v", err))
		}
	}
	// return to parent
	task.process = new(Process)
	task.process.Pid = pid
	log.Debugf("child pid is %d", pid)
	return pid
}

func (task *RunningTask) Run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// execute task
	task.runProcess()
	task.trace()
}

func (task *RunningTask) GetResult() *Result {
	log.Debugln(task.Result.String())

	return task.Result
}

func (task *RunningTask) trace() {
	process := task.process
	process.IsKilled = false

	tracer := TracerDetect{
		Pid:     task.process.Pid,
		Exit:    true,
		prevRax: 0,
	}

	tracer.setCallPolicy(makeCallPolicy(&task.setting.OneTimeCalls, &task.setting.AllowedCalls))

	for {
		process.Wait()

		if process.Exited() {
			log.Infoln("program exited!", process.Status.StopSignal())
			task.parseRunningInfo()
			break
		}
		if process.Broken() {
			// break by other signal but SIGTRAP
			log.Infoln("-------- Signal by: ", process.Status.StopSignal())
			task.parseRunningInfo()
			task.Result.detectSignal(process.Status.StopSignal())
			// send kill to process
			log.Debugf("Process broken, will kill process")
			process.Kill()
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
			log.Infoln("Program not alive! break")
			break
		}

	}
	task.check()
}

func (task *RunningTask) check() {
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
		log.Debugf("kill by memory limit: current %d, limit %d", task.Result.PeakMemory, task.memoryLimit)
		task.process.Kill()
		return
	}
}

func (task *RunningTask) outOfTime() bool {
	return task.Result.TimeCost > task.timeLimit
}

func (task *RunningTask) outOfMemory() bool {
	// check memory is over limit
	return (task.Result.PeakMemory > task.memoryLimit) && (task.Result.RusageMemory > task.memoryLimit)
}

func (task *RunningTask) refreshTimeCost() {
	task.Result.TimeCost = task.process.GetTimeCost()
	log.Debugf("current time cost: %dus(1e-6s)", task.Result.TimeCost)
}

func (task *RunningTask) refreshMemory() {
	memory, err := GetProcMemory(task.process.Pid)
	if err != nil {
		log.Infoln("Get status memory failed:", err)
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

func (task *RunningTask) redirectIO() {
	DupFileForRead("user.in", os.Stdin)
	DupFileForWrite("user.out", os.Stdout)
	DupFileForWrite("user.err", os.Stderr)
}

func (task *RunningTask) limitResource() {
	// max runProcess time
	timeLimit := uint64(task.setting.CPU)
	setResourceLimit(syscall.RLIMIT_CPU, &syscall.Rlimit{Max: timeLimit + 1, Cur: timeLimit})

	syscall.Syscall(syscall.SYS_ALARM, uintptr(timeLimit+5), 0, 0)

	// max file output size
	setResourceLimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{
		Max: uint64(task.setting.Output << 21),
		Cur: uint64(task.setting.Output << 20),
	})

	// max memory size
	// The maximum size of the process stack, in bytes
	// will cause SIGSEGV
	memoryLimit := uint64(task.memoryLimit<<10) * 4 // 4 times
	rLimit := &syscall.Rlimit{
		Max: memoryLimit + 5<<20, // more 5M
		Cur: memoryLimit,         // 4 times
	}
	setResourceLimit(syscall.RLIMIT_STACK, rLimit)
	// The maximum size of the process's data segment (initialized data, uninitialized data, and heap)
	setResourceLimit(syscall.RLIMIT_DATA, rLimit)
	// The maximum size of the process's virtual memory (address space) in bytes
	// will cause SIGSEGV
	setResourceLimit(syscall.RLIMIT_AS, rLimit)
}

func setResourceLimit(code int, rLimit *syscall.Rlimit) {
	err := syscall.Setrlimit(code, rLimit)
	if err != nil {
		log.Panic(err)
	}
}
