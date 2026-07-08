//go:build linux

package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/hustoj/runner/runner"
)

var log *zap.SugaredLogger

const compilerBootstrapEnv = "RUNNER_COMPILER_BOOTSTRAP"
const compilerCgroupGateFDEnv = "RUNNER_COMPILER_CGROUP_GATE_FD"

const (
	compileExitDupFailure   = 124
	compileExitSetupFailure = 125
	compileExitExecFailure  = 126
)

const (
	compilerCPULimitBufferSeconds   uint64 = 1
	compilerOpenFileLimit           uint64 = 64
	compilerCoreDumpDisabled        uint64 = 0
	compilerKillGraceTimeoutSeconds        = 2
	compilerWallClockMultiplier            = 3
	compilerWallClockBufferSeconds         = 2
)

var (
	compilerSetitimer             = unix.Setitimer
	compilerWait4                 = syscall.Wait4
	compilerKillGraceTimeout      = time.Duration(compilerKillGraceTimeoutSeconds) * time.Second
	startCompilerBootstrapProcess = startCompileBootstrapProcess
	newCompilerTaskController     = func(cfg *CompileConfig) (runner.TaskController, error) {
		return runner.NewCgroupTaskController(cfg.Memory, cfg.MaxProcs)
	}
)

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
	if err := syscall.Setrlimit(syscall.RLIMIT_CPU, &syscall.Rlimit{Max: cpu + compilerCPULimitBufferSeconds, Cur: cpu}); err != nil {
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
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &syscall.Rlimit{Max: compilerOpenFileLimit, Cur: compilerOpenFileLimit}); err != nil {
		return err
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_CORE, &syscall.Rlimit{Max: compilerCoreDumpDisabled, Cur: compilerCoreDumpDisabled}); err != nil {
		return err
	}
	return setCompileAlarm(cpu)
}

func setCompileAlarm(cpu uint64) error {
	_, err := compilerSetitimer(unix.ITIMER_REAL, unix.Itimerval{
		Value: unix.Timeval{Sec: int64(compilerWallClockTimeoutSeconds(int(cpu)))},
	})
	return err
}

func compilerWallClockTimeout(cpu int) time.Duration {
	return time.Duration(compilerWallClockTimeoutSeconds(cpu)) * time.Second
}

func compilerWallClockTimeoutSeconds(cpu int) int {
	if cpu < 0 {
		return compilerWallClockBufferSeconds
	}
	return cpu*compilerWallClockMultiplier + compilerWallClockBufferSeconds
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
	if os.Geteuid() == cfg.RunUID && os.Getegid() == cfg.RunGID {
		return nil
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

func startCompileBootstrapProcess(cfg *CompileConfig, gateReader *os.File) (int, error) {
	encodedConfig, err := encodeBootstrapConfig(cfg)
	if err != nil {
		return 0, fmt.Errorf("encode bootstrap config: %w", err)
	}
	self, err := os.Executable()
	if err != nil {
		return 0, err
	}

	env := append(runner.BuildMinimalEnv(compilerBootstrapEnv, compilerBootstrapConfigEnv, compilerCgroupGateFDEnv), compilerBootstrapEnv+"=1", compilerBootstrapConfigEnv+"="+encodedConfig)
	files := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	if gateReader != nil {
		env = append(env, compilerCgroupGateFDEnv+"=3")
		files = append(files, gateReader)
	}

	proc, err := os.StartProcess(self, []string{self}, &os.ProcAttr{
		Env:   env,
		Files: files,
		Sys:   &syscall.SysProcAttr{Setpgid: true},
	})
	if err != nil {
		return 0, err
	}
	return proc.Pid, nil
}

func awaitCompilerCgroupGate() error {
	fdValue := os.Getenv(compilerCgroupGateFDEnv)
	if fdValue == "" {
		return nil
	}
	fd, err := strconv.Atoi(fdValue)
	if err != nil {
		return fmt.Errorf("parse %s=%q: %w", compilerCgroupGateFDEnv, fdValue, err)
	}
	file := os.NewFile(uintptr(fd), "compiler-cgroup-gate")
	if file == nil {
		return fmt.Errorf("open cgroup gate fd %d", fd)
	}
	defer func() {
		_ = file.Close()
	}()

	gate := []byte{0}
	if _, err := file.Read(gate); err != nil {
		return fmt.Errorf("read cgroup gate: %w", err)
	}
	if gate[0] != 1 {
		return fmt.Errorf("invalid cgroup gate byte %d", gate[0])
	}
	return nil
}

func releaseCompilerCgroupGate(gateWriter *os.File) error {
	defer func() {
		_ = gateWriter.Close()
	}()
	if _, err := gateWriter.Write([]byte{1}); err != nil {
		return fmt.Errorf("write compiler cgroup gate: %w", err)
	}
	return nil
}

func bootstrapCompile(cfg *CompileConfig) {
	if err := cfg.ValidateSandbox(); err != nil {
		fmt.Fprintf(os.Stderr, "compile bootstrap rejected unsafe config: %v\n", err)
		syscall.Exit(compileExitSetupFailure)
	}
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

	controller, err := newCompilerTaskController(cfg)
	if err != nil {
		warnf("setup compiler task cgroup failed: %v", err)
		return &RunResult{Success: false}
	}

	pid := 0
	defer func() {
		cleanupCompilerTaskController(pid, controller)
	}()

	gateReader, gateWriter, err := os.Pipe()
	if err != nil {
		warnf("create compiler cgroup gate failed: %v", err)
		return &RunResult{Success: false}
	}
	defer func() {
		_ = gateReader.Close()
		_ = gateWriter.Close()
	}()

	pid, err = startCompilerBootstrapProcess(cfg, gateReader)
	_ = gateReader.Close()
	if err != nil {
		warnf("start compile bootstrap failed: %v", err)
		return &RunResult{Success: false}
	}
	infof("Child Pid is: %d", pid)

	if err := controller.MovePID(pid); err != nil {
		warnf("move compiler child %d into task cgroup failed: %v", pid, err)
		killCompilerProcessTree(pid, nil)
		waitCompilerChildAfterKill(pid)
		return &RunResult{Success: false}
	}
	if err := releaseCompilerCgroupGate(gateWriter); err != nil {
		warnf("release compiler cgroup gate failed: %v", err)
		killCompilerProcessTree(pid, controller)
		waitCompilerChildAfterKill(pid)
		return &RunResult{Success: false}
	}

	result := RunResult{Success: false}
	timeout := compilerWallClockTimeout(cfg.CPU)
	status, timedOut, err := waitForCompilerChild(pid, timeout, controller)
	if timedOut {
		warnf("compiler wall-clock limit %s exceeded", timeout)
	}
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

type compilerWaitResult struct {
	status syscall.WaitStatus
	err    error
}

func waitForCompilerChild(pid int, timeout time.Duration, controller runner.TaskController) (syscall.WaitStatus, bool, error) {
	waitCh := make(chan compilerWaitResult, 1)
	go func() {
		var status syscall.WaitStatus
		_, err := compilerWait4(pid, &status, 0, nil)
		waitCh <- compilerWaitResult{status: status, err: err}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case waitResult := <-waitCh:
		return waitResult.status, false, waitResult.err
	case <-timer.C:
		killCompilerProcessTree(pid, controller)
		select {
		case waitResult := <-waitCh:
			return waitResult.status, true, waitResult.err
		case <-time.After(compilerKillGraceTimeout):
			return 0, true, fmt.Errorf("compiler child %d did not exit after kill within %s", pid, compilerKillGraceTimeout)
		}
	}
}

func waitCompilerChildAfterKill(pid int) {
	var status syscall.WaitStatus
	if _, err := compilerWait4(pid, &status, 0, nil); err != nil {
		warnf("wait compiler child after kill failed: %v", err)
	}
}

func killCompilerProcessTree(pid int, controller runner.TaskController) {
	killAndDrainCompilerTaskCgroup(controller)
	if pid <= 0 {
		return
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

func killAndDrainCompilerTaskCgroup(controller runner.TaskController) {
	if controller == nil {
		return
	}
	if err := controller.Kill(); err != nil {
		warnf("kill and drain compiler task cgroup failed: %v", err)
	}
}

func cleanupCompilerTaskController(pid int, controller runner.TaskController) {
	if controller == nil {
		return
	}
	killAndDrainCompilerTaskCgroup(controller)
	if err := controller.Cleanup(); err == nil {
		return
	} else {
		warnf("cleanup compiler task cgroup failed: %v", err)
	}

	if pid > 0 {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	if err := controller.Cleanup(); err != nil {
		warnf("cleanup compiler task cgroup after kill failed: %v", err)
	}
}
