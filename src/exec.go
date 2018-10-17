package runner

import (
	"github.com/sirupsen/logrus"
	"os"
	"syscall"
)

type RunningTask struct {
	setting     *Setting
	process     Process
	Result      Result
}

func (task *RunningTask) init() {
	task.Result.Init()

	task.setting = LoadConfig()
	logrus.Infoln(task.setting)
}

func (task *RunningTask) execute() int {
	pid := fork()
	if pid < 0 {
		logrus.Panic("fork child failed")
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
	// loading config
	task.init()
	// execute task
	pid := task.execute()

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
			task.Result.RetCode = ACCEPT
			break
		}
		if process.Broken() {
			// break by other signal but SIGTRAP
			logrus.Debugln("Signal by: ", process.Status.StopSignal())
			task.GetResult()
			task.Result.detectSignal(process.Status.StopSignal())
			// send kill to process
			process.Kill()
			break
		}

		if tracer.detect() {
			break
		}
		// before next ptrace,get result, always pass
		task.GetResult()
	}
	logrus.Infoln(task.Result.String())
}

func (task *RunningTask) GetResult() {
	task.Result.TimeCost = task.process.GetTimeCost()
	memory, err := GetProcMemory(task.process.Pid)
	if err != nil {
		logrus.Infoln("Get memory failed:", err)
		return
	}
	task.Result.Memory = memory
}

func (task *RunningTask) resetIO() {
	infile, err := os.OpenFile("user.in", os.O_RDONLY|os.O_CREATE, 0666);
	if err != nil {
		logrus.Panic("open input data file failed", err)
	}
	outfile, err := os.OpenFile("user.out", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		logrus.Panic("open user output file failed", err)
	}
	errfile, err := os.OpenFile("user.err", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		logrus.Panic("open err file failed:", err)
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
		logrus.Panic(err)
	}

	syscall.Syscall(syscall.SYS_ALARM, uintptr(rLimit.Max), 0, 0)

	// max file output size
	rLimit.Max = 256 << 10 // 256kb
	rLimit.Cur = 128 << 10 // 128kb
	err = syscall.Setrlimit(syscall.RLIMIT_FSIZE, &rLimit)
	if err != nil {
		logrus.Panic(err)
	}

	// max memory size
	rLimit.Max = uint64(task.setting.MemoryLimit<<20) + 1<<20
	rLimit.Cur = uint64(task.setting.MemoryLimit << 20)
	err = syscall.Setrlimit(syscall.RLIMIT_STACK, &rLimit)
	if err != nil {
		logrus.Panic(err)
	}
}
