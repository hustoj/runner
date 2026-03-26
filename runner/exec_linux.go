//go:build linux

package runner

import (
	"os"
	"syscall"
)

func ptraceTraceme() error {
	_, _, errno := syscall.Syscall(syscall.SYS_PTRACE, syscall.PTRACE_TRACEME, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

func setAlarm(seconds uint64) {
	syscall.Syscall(syscall.SYS_ALARM, uintptr(seconds), 0, 0)
}

func (task *RunningTask) runProcess() {
	pid, err := task.startBootstrapProcess()
	if err != nil {
		log.Panicf("start child failed: %v", err)
	}

	task.process = new(Process)
	task.process.Pid = pid
	log.Debugf("child pid is %d", pid)
}

func (task *RunningTask) limitResource() error {
	timeLimit := uint64(task.setting.CPU)
	if err := syscall.Setrlimit(syscall.RLIMIT_CPU, &syscall.Rlimit{Max: timeLimit + 3, Cur: timeLimit + 1}); err != nil {
		return err
	}

	setAlarm(timeLimit + 5)

	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{
		Max: uint64(task.setting.Output << 21),
		Cur: uint64(task.setting.Output << 20),
	}); err != nil {
		return err
	}

	memoryLimit := uint64(task.memoryLimit<<10) * 4
	rLimit := &syscall.Rlimit{
		Max: memoryLimit + 5<<20,
		Cur: memoryLimit,
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_STACK, rLimit); err != nil {
		return err
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_DATA, rLimit); err != nil {
		return err
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_AS, rLimit); err != nil {
		return err
	}

	// Prevent fork bombs: limit to 1 process for the sandbox UID.
	const rlimitNProc = 6 // RLIMIT_NPROC not exported by Go's syscall package
	if err := syscall.Setrlimit(rlimitNProc, &syscall.Rlimit{Max: 1, Cur: 1}); err != nil {
		return err
	}
	// Restrict open file descriptors (stdin/stdout/stderr + a few extras).
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &syscall.Rlimit{Max: 16, Cur: 16}); err != nil {
		return err
	}
	// Disable core dumps to avoid filling disk with large dumps.
	if err := syscall.Setrlimit(syscall.RLIMIT_CORE, &syscall.Rlimit{Max: 0, Cur: 0}); err != nil {
		return err
	}
	return nil
}

func (task *RunningTask) redirectIO() error {
	if err := dupFileForRead("user.in", os.Stdin); err != nil {
		return err
	}
	if err := dupFileForWrite("user.out", os.Stdout); err != nil {
		return err
	}
	return dupFileForWrite("user.err", os.Stderr)
}
