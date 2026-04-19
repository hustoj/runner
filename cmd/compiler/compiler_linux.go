//go:build linux

package main

import (
	"fmt"
	"os"
	"syscall"

	"go.uber.org/zap"

	"github.com/hustoj/runner/runner"
)

var log *zap.SugaredLogger

const (
	compileExitDupFailure   = 124
	compileExitSetupFailure = 125
	compileExitExecFailure  = 126
)

type compileExecSpec struct {
	binary string
	args   []string
	env    []string
}

func initLog(m *CompileConfig) {
	log = runner.InitLogger(m.LogPath, m.Verbose)
}

func warnf(format string, args ...interface{}) {
	if log != nil {
		log.Warnf(format, args...)
	}
}

func infof(format string, args ...interface{}) {
	if log != nil {
		log.Infof(format, args...)
	}
}

func setrLimits(cpu, memory, output, stack uint64) error {
	if err := syscall.Setrlimit(syscall.RLIMIT_CPU, &syscall.Rlimit{Max: cpu + 1, Cur: cpu}); err != nil {
		return err
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Max: output << 20, Cur: output << 20}); err != nil {
		return err
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_STACK, &syscall.Rlimit{Max: stack << 20, Cur: stack << 20}); err != nil {
		return err
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_AS, &syscall.Rlimit{Max: memory << 20, Cur: memory << 20}); err != nil {
		return err
	}
	if _, _, errno := syscall.Syscall(syscall.SYS_ALARM, uintptr(cpu*3+2), 0, 0); errno != 0 {
		return errno
	}
	return nil
}

func prepareCompileExecSpec(cfg *CompileConfig) (compileExecSpec, error) {
	binary, args, err := cfg.ResolveExec()
	if err != nil {
		return compileExecSpec{}, err
	}

	return compileExecSpec{
		binary: binary,
		args:   args,
		env:    os.Environ(),
	}, nil
}

func dupFileForWrite(filename string, fd int) error {
	target, err := syscall.Open(filename, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer syscall.Close(target)

	return syscall.Dup2(target, fd)
}

func doCompile(spec compileExecSpec, cfg *CompileConfig) int {
	if err := setrLimits(uint64(cfg.CPU), uint64(cfg.Memory), uint64(cfg.Output), uint64(cfg.Stack)); err != nil {
		return compileExitSetupFailure
	}
	if err := dupFileForWrite("compile.err", int(os.Stderr.Fd())); err != nil {
		return compileExitDupFailure
	}
	if err := dupFileForWrite("compile.out", int(os.Stdout.Fd())); err != nil {
		return compileExitDupFailure
	}
	if err := syscall.Exec(spec.binary, spec.args, spec.env); err != nil {
		return compileExitExecFailure
	}

	return 0
}

func exitChild(code int) {
	_, _, _ = syscall.RawSyscall(syscall.SYS_EXIT, uintptr(code), 0, 0)
	for {
	}
}

func fork() (int, syscall.Errno) {
	r1, _, errno := syscall.RawSyscall(syscall.SYS_FORK, 0, 0, 0)
	if errno != 0 || r1 < 0 {
		return -1, errno
	}
	return int(r1), 0
}

func runProcessC(spec compileExecSpec, cfg *CompileConfig) (int, error) {
	pid, errno := fork()
	if errno != 0 || pid < 0 {
		if errno == 0 {
			errno = syscall.EINVAL
		}
		return 0, errno
	}
	if pid == 0 {
		exitChild(doCompile(spec, cfg))
	}
	return pid, nil
}

func compileSucceeded(status syscall.WaitStatus) bool {
	return status.Exited() && status.ExitStatus() == 0
}

func compileFailureReason(status syscall.WaitStatus) string {
	if status.Exited() {
		switch status.ExitStatus() {
		case compileExitDupFailure:
			return "compiler child failed while redirecting output"
		case compileExitSetupFailure:
			return "compiler child failed while applying startup limits"
		case compileExitExecFailure:
			return "compiler child failed to exec compiler binary"
		default:
			return fmt.Sprintf("compiler exited with status %d", status.ExitStatus())
		}
	}
	if status.Signaled() {
		return fmt.Sprintf("compiler terminated by signal %d", status.Signal())
	}
	return "compiler finished with an unexpected wait status"
}

func handle(cfg *CompileConfig) *RunResult {
	var status syscall.WaitStatus
	spec, err := prepareCompileExecSpec(cfg)
	if err != nil {
		warnf("prepare compiler child failed: %v", err)
		return &RunResult{Success: false}
	}

	pid, err := runProcessC(spec, cfg)
	if err != nil {
		warnf("fork compiler child failed: %v", err)
		return &RunResult{Success: false}
	}
	infof("Child Pid is: %d", pid)

	result := RunResult{Success: false}

	_, err = syscall.Wait4(pid, &status, 0, nil)
	if err != nil {
		warnf("wait compiler child failed: %v", err)
		return &result
	}
	result.Success = compileSucceeded(status)
	if !result.Success {
		warnf("%s", compileFailureReason(status))
	}
	return &result
}
