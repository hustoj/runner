//go:build linux

package runner

import (
	"fmt"
	"os"
	"syscall"
)

func ptraceTraceme() {
	syscall.Syscall(syscall.SYS_PTRACE, syscall.PTRACE_TRACEME, 0, 0)
}

func setAlarm(seconds uint64) {
	syscall.Syscall(syscall.SYS_ALARM, uintptr(seconds), 0, 0)
}

func (task *RunningTask) runProcess() {
	pid := fork()
	if pid < 0 {
		log.Panic("fork child failed")
	}
	if pid == 0 {
		// enter child
		task.limitResource()
		task.redirectIO()

		ptraceTraceme()

		env := os.Environ()
		log.Debugf("Command is %s, Args is %s", task.setting.GetCommand(), task.setting.GetArgs())
		err := syscall.Exec(task.setting.GetCommand(), task.setting.GetArgs(), env)
		if err != nil {
			panic(fmt.Sprintf("Exec failed: %v", err))
		}
	}
	// return to parent
	task.process = new(Process)
	task.process.Pid = pid
	log.Debugf("child pid is %d", pid)
}

func (task *RunningTask) limitResource() {
	// max runProcess time
	timeLimit := uint64(task.setting.CPU)
	setResourceLimit(syscall.RLIMIT_CPU, &syscall.Rlimit{Max: timeLimit + 3, Cur: timeLimit + 1})

	setAlarm(timeLimit + 5)

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
