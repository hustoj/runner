package runner

import (
	"os"
	"syscall"
)

type RunningTask struct {
	setting     *Setting
	process     Process
	Result      *Result
}

func (task *RunningTask) Init(setting *Setting) {
	task.Result = &Result{}
	task.setting = setting

	task.Result.Init()
	log.Debugln(task.setting)
}

func (task *RunningTask) execute() int {
	pid := fork()
	if pid < 0 {
		log.Panic("fork child failed")
	}
	if pid == 0 {
		// here is child
		task.resetIO()
		task.setResourceLimit()
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
			log.Debugln("program exited!", process.Status.StopSignal())
			task.refreshTimeCost()
			task.Result.RetCode = ACCEPT
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
	if task.Result.TimeCost > int64(task.setting.TimeLimit) * 1000 {
		task.Result.RetCode = TIME_LIMIT
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
	if task.Result.Memory > int64(task.setting.MemoryLimit) * 1024 {
		task.Result.RetCode = MEMORY_LIMIT
		task.process.Kill()
	}
}

func (task *RunningTask) parseRunningInfo() {
	task.refreshTimeCost()
	task.refreshMemory()
}

func (task *RunningTask) resetIO() {
	infile, err := os.OpenFile("user.in", os.O_RDONLY|os.O_CREATE, 0666);
	if err != nil {
		log.Panic("open input data file failed", err)
	}
	outfile, err := os.OpenFile("user.out", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		log.Panic("open user output file failed", err)
	}
	errfile, err := os.OpenFile("user.err", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		log.Panic("open err file failed:", err)
	}

	fileDup(infile, os.Stdin)
	fileDup(outfile, os.Stdout)
	fileDup(errfile, os.Stderr)
}

func (task *RunningTask) setResourceLimit() {
	var rLimit syscall.Rlimit
	// max execute time
	rLimit.Max = uint64(task.setting.TimeLimit + 1)
	rLimit.Cur = uint64(task.setting.TimeLimit)
	err := syscall.Setrlimit(syscall.RLIMIT_CPU, &rLimit)
	if err != nil {
		log.Panic(err)
	}

	syscall.Syscall(syscall.SYS_ALARM, uintptr(rLimit.Max), 0, 0)

	// max file output size
	rLimit.Max = 256 << 10 // 256kb
	rLimit.Cur = 128 << 10 // 128kb
	err = syscall.Setrlimit(syscall.RLIMIT_FSIZE, &rLimit)
	if err != nil {
		log.Panic(err)
	}

	// max memory size
	rLimit.Max = uint64(task.setting.MemoryLimit<<20) + 1<<20
	rLimit.Cur = uint64(task.setting.MemoryLimit << 20)
	err = syscall.Setrlimit(syscall.RLIMIT_STACK, &rLimit)
	err = syscall.Setrlimit(syscall.RLIMIT_DATA, &rLimit)
	if err != nil {
		log.Panic(err)
	}
}

