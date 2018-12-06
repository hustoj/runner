package runner

import (
	"os"
	"syscall"
)

type RunningTask struct {
	setting *Setting
	process Process
	Result  *Result
	timeLimit int64
}

func (task *RunningTask) Init(setting *Setting) {
	task.Result = &Result{}
	task.setting = setting
	task.timeLimit = int64(setting.TimeLimit) * 1000

	task.Result.Init()
	log.Debugf("load case config %v\n", task.setting)
}

func (task *RunningTask) execute() int {
	pid := fork()
	if pid < 0 {
		log.Panic("fork child failed")
	}
	if pid == 0 {
		// enter child
		task.resetIO()
		task.limitResource()
		task.allowSyscall()
		syscall.Syscall(syscall.SYS_PTRACE, syscall.PTRACE_TRACEME, 0, 0)
		syscall.Exec("./Main", nil, nil)
	}
	// return to parent
	return pid
}

func (task *RunningTask) Run() {
	// execute task
	pid := task.execute()

	log.Debugf("child pid is %d", pid)
	task.process.Pid = pid
	task.trace()
}

func (task *RunningTask) GetResult() *Result {
	log.Debugln(task.Result.String())

	return task.Result
}

func (task *RunningTask) trace() {
	process := &task.process
	process.IsKilled = false

	tracer := TracerDetect{
		Pid:     task.process.Pid,
		Exit:    true,
		prevRax: 0,
	}

	for {
		process.Wait()

		if process.Exited() {
			log.Infoln("program exited!", process.Status.StopSignal())
			task.refreshTimeCost()
			break
		}
		if process.Broken() {
			// break by other signal but SIGTRAP
			log.Infoln("Signal by: ", process.Status.StopSignal())
			task.parseRunningInfo()
			task.Result.detectSignal(process.Status.StopSignal())
			// send kill to process
			process.Kill()
			break
		}

		if tracer.detect() {
			break
		}
		// before next ptrace, get result, always pass
		task.parseRunningInfo()
		process.Continue()
	}
}

func (task *RunningTask) refreshTimeCost() {
	task.Result.TimeCost = task.process.GetTimeCost()
	if task.Result.TimeCost > task.timeLimit {
		task.Result.RetCode = TIME_LIMIT
		log.Debugf("kill by time limit: current %d, limit %d\n", task.Result.TimeCost, task.setting.TimeLimit)
		task.process.Kill()
	}
}

func (task *RunningTask) refreshMemory() {
	memory, err := GetProcMemory(task.process.Pid)
	if err != nil {
		log.Infoln("Get memory failed:", err)
		return
	}
	if memory > task.Result.Memory {
		task.Result.Memory = memory
	}

	// detect memory is over limit
	if task.Result.Memory > int64(task.setting.MemoryLimit)*1024 {
		task.Result.RetCode = MEMORY_LIMIT
		log.Debugln("kill by memory limit:", task.Result.Memory, task.setting.MemoryLimit)
		task.process.Kill()
	}
}

func (task *RunningTask) parseRunningInfo() {
	task.refreshTimeCost()
	task.refreshMemory()
}

func (task *RunningTask) resetIO() {
	dupFileForRead("user.in", os.Stdin)
	dupFileForWrite("user.out", os.Stdout)
	dupFileForWrite("user.err", os.Stderr)
}

func (task *RunningTask) limitResource() {
	// max execute time
	timeLimit := uint64(task.setting.TimeLimit)
	setResourceLimit(syscall.RLIMIT_CPU, &syscall.Rlimit{Max: timeLimit + 1, Cur: timeLimit})

	syscall.Syscall(syscall.SYS_ALARM, uintptr(timeLimit), 0, 0)

	// max file output size
	setResourceLimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{
		Max: 256 << 10, // 256kb
		Cur: 128 << 10, // 128kb
	})

	// max memory size
	// The maximum size of the process stack, in bytes
	// will cause SIGSEGV
	memoryLimit := uint64(task.setting.MemoryLimit<<20)*4 // 4 times
	rLimit := &syscall.Rlimit{
		Max: memoryLimit + 5 << 20, // more 5M
		Cur: memoryLimit, // 4 times
	}
	setResourceLimit(syscall.RLIMIT_STACK, rLimit)
	// The maximum size of the process's data segment (initialized data, uninitialized data, and heap)
	setResourceLimit(syscall.RLIMIT_DATA, rLimit)
	// The maximum size of the process's virtual memory (address space) in bytes
	// will cause SIGSEGV
	setResourceLimit(syscall.RLIMIT_AS, rLimit)
}

func (task *RunningTask) allowSyscall() {
	//scs := []string{"ptrace", "execve", "read", "write", "brk", "fstat",
	//	"uname", "mmap", "arch_prctl", "exit_group", "nanosleep", "readlink",
	//	"access"}
	//allowSyscall(scs)
}

func setResourceLimit(code int, rLimit *syscall.Rlimit) {
	err := syscall.Setrlimit(code, rLimit)
	if err != nil {
		log.Panic(err)
	}
}
