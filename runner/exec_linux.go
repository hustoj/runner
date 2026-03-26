//go:build linux

package runner

import (
	"fmt"
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

func (task *RunningTask) runProcess() error {
	pid, err := task.startBootstrapProcess()
	if err != nil {
		return fmt.Errorf("start child: %w", err)
	}

	task.process = new(Process)
	task.process.Pid = pid
	log.Debugf("child pid is %d", pid)
	return nil
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

	memoryBytes := uint64(task.memoryLimit) << 10 // KB → bytes

	// RLIMIT_STACK: use independent Stack config (MB → bytes).
	// Previously this was set to the same 4× amplified memory value,
	// ignoring the dedicated Stack field entirely.
	stackBytes := uint64(task.setting.Stack) << 20
	if err := syscall.Setrlimit(syscall.RLIMIT_STACK, &syscall.Rlimit{
		Max: stackBytes,
		Cur: stackBytes,
	}); err != nil {
		return err
	}

	// RLIMIT_DATA: limit heap/data-segment growth.
	// A small margin (16 MB) accommodates runtime metadata (e.g. libc
	// malloc arenas, TLS) without inflating the effective limit.
	const dataOverhead = 16 << 20 // 16 MB
	if err := syscall.Setrlimit(syscall.RLIMIT_DATA, &syscall.Rlimit{
		Max: memoryBytes + dataOverhead,
		Cur: memoryBytes,
	}); err != nil {
		return err
	}

	// RLIMIT_AS: total virtual address space.
	// Virtual address space can far exceed resident memory: mmap'd shared
	// libraries, JVM/Go arena reservations, guard pages, etc.  We allow
	// max(memoryBytes, 64 MB) of overhead so that heavy runtimes (Java, Go)
	// can start without ENOMEM, while the ptrace-based VmHWM checker still
	// enforces the real memory limit at the configured threshold.
	asOverhead := memoryBytes
	if asOverhead < 64<<20 {
		asOverhead = 64 << 20 // minimum 64 MB overhead
	}
	asLimit := memoryBytes + asOverhead + stackBytes
	if err := syscall.Setrlimit(syscall.RLIMIT_AS, &syscall.Rlimit{
		Max: asLimit,
		Cur: asLimit,
	}); err != nil {
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
