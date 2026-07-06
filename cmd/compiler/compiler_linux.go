//go:build linux

package main

import (
	"fmt"
	"os"
	"runtime"
	"syscall"

	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/hustoj/runner/runner"
)

var log *zap.SugaredLogger

const compilerBootstrapEnv = "RUNNER_COMPILER_BOOTSTRAP"

const (
	compileExitDupFailure   = 124
	compileExitSetupFailure = 125
	compileExitExecFailure  = 126
)

var compilerSetitimer = unix.Setitimer

func setNoNewPrivs() syscall.Errno {
	_, _, errno := syscall.RawSyscall6(syscall.SYS_PRCTL, uintptr(runner.PrSetNoNewPrivs), 1, 0, 0, 0, 0)
	return errno
}

func initLog(m *CompileConfig) {
	var err error
	log, err = runner.InitLogger(m.LogPath, m.Verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compiler: init logger: %v\n", err)
		os.Exit(1)
	}
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
	if err := syscall.Setrlimit(unix.RLIMIT_NPROC, &syscall.Rlimit{Max: 32, Cur: 32}); err != nil {
		return err
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &syscall.Rlimit{Max: 64, Cur: 64}); err != nil {
		return err
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_CORE, &syscall.Rlimit{Max: 0, Cur: 0}); err != nil {
		return err
	}
	return setCompileAlarm(cpu)
}

func setCompileAlarm(cpu uint64) error {
	_, err := compilerSetitimer(unix.ITIMER_REAL, unix.Itimerval{
		Value: unix.Timeval{Sec: int64(cpu*3 + 2)},
	})
	return err
}

func doCompile(cfg *CompileConfig) error {
	runtime.LockOSThread()

	if err := applyCompilerSandbox(cfg); err != nil {
		return err
	}
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

	return syscall.Exec(binary, args, runner.BuildMinimalEnv(compilerBootstrapEnv, compilerBootstrapConfigEnv))
}

func applyCompilerSandbox(cfg *CompileConfig) error {
	if err := unshareCompilerNamespaces(cfg); err != nil {
		return err
	}
	if err := applyCompilerRootFS(cfg); err != nil {
		return err
	}
	if cfg.NoNewPrivs {
		if errno := setNoNewPrivs(); errno != 0 {
			return fmt.Errorf("set no_new_privs: %w", errno)
		}
	}
	if err := dropCompilerCredentials(cfg); err != nil {
		return err
	}
	return nil
}

func unshareCompilerNamespaces(cfg *CompileConfig) error {
	flags := compilerNamespaceFlags(cfg)
	if flags == 0 {
		return nil
	}
	if err := unix.Unshare(flags); err != nil {
		return fmt.Errorf("unshare compiler namespaces: %w", err)
	}
	return nil
}

func compilerNamespaceFlags(cfg *CompileConfig) int {
	flags := 0
	if cfg.UseMountNS {
		flags |= unix.CLONE_NEWNS
	}
	if cfg.UseIPCNS {
		flags |= unix.CLONE_NEWIPC
	}
	if cfg.UseUTSNS {
		flags |= unix.CLONE_NEWUTS
	}
	if cfg.UseNetNS {
		flags |= unix.CLONE_NEWNET
	}
	return flags
}

func applyCompilerRootFS(cfg *CompileConfig) error {
	workDir, err := cfg.sandboxWorkDir()
	if err != nil {
		return err
	}

	if cfg.ChrootDir != "" {
		if err := syscall.Chroot(cfg.ChrootDir); err != nil {
			return fmt.Errorf("chroot to %q: %w", cfg.ChrootDir, err)
		}
	}
	if workDir == "" {
		return nil
	}
	if err := os.Chdir(workDir); err != nil {
		return fmt.Errorf("chdir to %q: %w", workDir, err)
	}
	return nil
}

func dropCompilerCredentials(cfg *CompileConfig) error {
	uidSet := cfg.RunUID > 0
	gidSet := cfg.RunGID > 0
	if !uidSet && !gidSet {
		return nil
	}
	if uidSet != gidSet {
		return fmt.Errorf("compiler uid/gid must be configured together (got uid=%d, gid=%d)", cfg.RunUID, cfg.RunGID)
	}
	if err := syscall.Setgroups(nil); err != nil {
		return fmt.Errorf("clear supplementary groups: %w", err)
	}
	if err := syscall.Setgid(cfg.RunGID); err != nil {
		return fmt.Errorf("setgid %d: %w", cfg.RunGID, err)
	}
	if err := syscall.Setuid(cfg.RunUID); err != nil {
		return fmt.Errorf("setuid %d: %w", cfg.RunUID, err)
	}
	return nil
}

func startCompileBootstrapProcess(cfg *CompileConfig) (int, error) {
	encodedConfig, err := encodeBootstrapConfig(cfg)
	if err != nil {
		return 0, fmt.Errorf("encode bootstrap config: %w", err)
	}
	return runner.StartBootstrapChildWithEnv(compilerBootstrapEnv, []string{compilerBootstrapConfigEnv + "=" + encodedConfig})
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

func handle(cfg *CompileConfig) *RunResult {
	if err := cfg.ValidateSandbox(); err != nil {
		warnf("invalid sandbox config: %v", err)
		return &RunResult{Success: false}
	}

	var status syscall.WaitStatus
	pid, err := startCompileBootstrapProcess(cfg)
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
