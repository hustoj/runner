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

const compilerBootstrapEnv = "RUNNER_COMPILER_BOOTSTRAP"

const (
	compileExitDupFailure   = 124
	compileExitSetupFailure = 125
	compileExitExecFailure  = 126
)

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

func doCompile(cfg *CompileConfig) error {
	if err := setrLimits(uint64(cfg.CPU), uint64(cfg.Memory), uint64(cfg.Output), uint64(cfg.Stack)); err != nil {
		return err
	}
	if err := runner.DupFileForWrite("compile.err", os.Stderr); err != nil {
		return err
	}
	if err := runner.DupFileForWrite("compile.out", os.Stdout); err != nil {
		return err
	}
	if err := runner.CloseNonStdioFiles(); err != nil {
		return err
	}

	binary, args, err := cfg.ResolveExec()
	if err != nil {
		return err
	}

	return syscall.Exec(binary, args, runner.BuildMinimalEnv(compilerBootstrapEnv))
}

func startCompileBootstrapProcess() (int, error) {
	return runner.StartBootstrapChild(compilerBootstrapEnv)
}

func bootstrapCompile(cfg *CompileConfig) {
	if err := doCompile(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "compile bootstrap failed: %v\n", err)
		syscall.Exit(1)
	}
}

func isCompilerBootstrapProcess() bool {
	return os.Getenv(compilerBootstrapEnv) == "1"
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

func handle(_ *CompileConfig) *RunResult {
	var status syscall.WaitStatus
	pid, err := startCompileBootstrapProcess()
	if err != nil {
		warnf("start compile bootstrap failed: %v", err)
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
