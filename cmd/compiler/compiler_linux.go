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

func initLog(m *CompileConfig) {
	var err error
	log, err = runner.InitLogger(m.LogPath, m.Verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compiler: init logger: %v\n", err)
		os.Exit(1)
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
	syscall.Syscall(syscall.SYS_ALARM, uintptr(cpu*3+2), 0, 0)
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

func handle(_ *CompileConfig) *RunResult {
	var status syscall.WaitStatus
	var rusage syscall.Rusage
	pid, err := startCompileBootstrapProcess()
	if err != nil {
		log.Errorw("start compile bootstrap failed", "error", err)
		return &RunResult{Success: false}
	}
	log.Info("Child Pid is: ", pid)

	result := RunResult{Success: true}

	_, err = syscall.Wait4(pid, &status, 0, &rusage)
	if err != nil || !compileSucceeded(status) {
		result.Success = false
	}
	return &result
}

func compileSucceeded(status syscall.WaitStatus) bool {
	return status.Exited() && status.ExitStatus() == 0
}

func isCompilerBootstrapProcess() bool {
	return os.Getenv(compilerBootstrapEnv) == "1"
}
