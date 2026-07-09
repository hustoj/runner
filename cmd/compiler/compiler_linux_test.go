//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/hustoj/runner/runner"
	"golang.org/x/sys/unix"
)

func TestResolveExecIncludesBinary(t *testing.T) {
	wantBinary, err := exec.LookPath("sh")
	if err != nil {
		t.Fatalf("exec.LookPath(sh) error = %v", err)
	}

	cfg := &CompileConfig{
		Command: "sh",
		Args:    newCompileArgs("-c", "printf ok"),
	}

	binary, args, err := cfg.ResolveExec()
	if err != nil {
		t.Fatalf("ResolveExec() error = %v", err)
	}
	if binary != wantBinary {
		t.Fatalf("ResolveExec() binary = %q, want %q", binary, wantBinary)
	}

	wantArgs := []string{wantBinary, "-c", "printf ok"}
	if len(args) != len(wantArgs) {
		t.Fatalf("ResolveExec() args = %v, want %v", args, wantArgs)
	}
	for i := range wantArgs {
		if args[i] != wantArgs[i] {
			t.Fatalf("ResolveExec() args[%d] = %q, want %q", i, args[i], wantArgs[i])
		}
	}
}

func TestBuildMinimalEnvRemovesBootstrapMarker(t *testing.T) {
	t.Setenv(compilerBootstrapEnv, "1")
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("LD_PRELOAD", "bad.so")

	for _, entry := range runner.BuildMinimalEnv(compilerBootstrapEnv) {
		if strings.HasPrefix(entry, compilerBootstrapEnv+"=") {
			t.Fatalf("BuildMinimalEnv() kept bootstrap marker: %q", entry)
		}
		if strings.HasPrefix(entry, "LD_PRELOAD=") {
			t.Fatalf("BuildMinimalEnv() kept unsafe env: %q", entry)
		}
	}
}

func waitStatusForCommand(t *testing.T, command string) syscall.WaitStatus {
	t.Helper()

	cmd := exec.Command("sh", "-c", command)
	// These helper commands intentionally exercise non-zero exits and signals;
	// we only need the resulting ProcessState / WaitStatus.
	_ = cmd.Run()
	if cmd.ProcessState == nil {
		t.Fatal("process state is nil")
	}

	status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
	if !ok {
		t.Fatalf("unexpected process state type: %T", cmd.ProcessState.Sys())
	}
	return status
}

func waitStatusForExit(t *testing.T, code int) syscall.WaitStatus {
	t.Helper()
	return waitStatusForCommand(t, "exit "+strconv.Itoa(code))
}

func TestCompileSucceeded(t *testing.T) {
	tests := []struct {
		name   string
		status syscall.WaitStatus
		want   bool
	}{
		{
			name:   "zero exit succeeds",
			status: waitStatusForExit(t, 0),
			want:   true,
		},
		{
			name:   "non-zero exit fails",
			status: waitStatusForExit(t, 1),
			want:   false,
		},
		{
			name:   "signal termination fails",
			status: waitStatusForCommand(t, "kill -KILL $$"),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compileSucceeded(tt.status); got != tt.want {
				t.Fatalf("compileSucceeded() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompileFailureReason(t *testing.T) {
	tests := []struct {
		name   string
		status syscall.WaitStatus
		want   string
	}{
		{
			name:   "dup failure exit code",
			status: waitStatusForExit(t, compileExitDupFailure),
			want:   "compiler child failed while redirecting output",
		},
		{
			name:   "setup failure exit code",
			status: waitStatusForExit(t, compileExitSetupFailure),
			want:   "compiler child failed while applying startup limits",
		},
		{
			name:   "exec failure exit code",
			status: waitStatusForExit(t, compileExitExecFailure),
			want:   "compiler child failed to exec compiler binary",
		},
		{
			name:   "signal termination",
			status: waitStatusForCommand(t, "kill -KILL $$"),
			want:   "compiler terminated by signal 9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compileFailureReason(tt.status); got != tt.want {
				t.Fatalf("compileFailureReason() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveExecReturnsErrorWhenCompilerBinaryMissing(t *testing.T) {
	cfg := &CompileConfig{
		Command: "definitely-missing-compiler-binary",
	}

	_, _, err := cfg.ResolveExec()
	if err == nil {
		t.Fatal("ResolveExec() should fail when compiler binary is missing")
	}
}

func TestStartCompileBootstrapProcessRejectsOversizedConfig(t *testing.T) {
	oversized := strings.Repeat("x", compilerBootstrapConfigMaxBytes+1)
	cfg := &CompileConfig{
		Command:    "gcc",
		Args:       newCompileArgs(oversized),
		NoNewPrivs: true,
		MaxProcs:   32,
	}

	pid, err := startCompileBootstrapProcess(cfg, nil)
	if err == nil {
		t.Fatal("startCompileBootstrapProcess() should fail for oversized config")
	}
	if pid != 0 {
		// 防御：encode 失败应早于 StartProcess，不应有子进程残留
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Fatalf("startCompileBootstrapProcess() pid = %d, want 0 (no child should be started)", pid)
	}
	if !strings.Contains(err.Error(), "encode bootstrap config") {
		t.Fatalf("error %q should originate from encode bootstrap config", err.Error())
	}
}

func TestSetCompileAlarmPropagatesSetitimerError(t *testing.T) {
	oldSetitimer := compilerSetitimer
	compilerSetitimer = func(which unix.ItimerWhich, it unix.Itimerval) (unix.Itimerval, error) {
		return unix.Itimerval{}, syscall.EINVAL
	}
	defer func() {
		compilerSetitimer = oldSetitimer
	}()

	err := setCompileAlarm(1)
	if !errors.Is(err, syscall.EINVAL) {
		t.Fatalf("setCompileAlarm() error = %v, want %v", err, syscall.EINVAL)
	}
}

func TestCompilerWallClockTimeoutSeconds(t *testing.T) {
	tests := []struct {
		name string
		cpu  int
		want int
	}{
		{name: "negative cpu uses buffer only", cpu: -1, want: 2},
		{name: "zero cpu uses buffer only", cpu: 0, want: 2},
		{name: "positive cpu uses multiplier plus buffer", cpu: 5, want: 17},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compilerWallClockTimeoutSeconds(tt.cpu); got != tt.want {
				t.Fatalf("compilerWallClockTimeoutSeconds(%d) = %d, want %d", tt.cpu, got, tt.want)
			}
		})
	}
}

func TestSetNoNewPrivsSucceeds(t *testing.T) {
	// prctl(PR_SET_NO_NEW_PRIVS, 1) should succeed for any process.
	errno := setNoNewPrivs()
	if errno != 0 {
		t.Fatalf("setNoNewPrivs() errno = %v, want 0", errno)
	}
}

func TestCompilerNamespaceFlags(t *testing.T) {
	cfg := &CompileConfig{
		UseMountNS: true,
		UseIPCNS:   true,
		UseUTSNS:   true,
		UseNetNS:   true,
	}

	want := unix.CLONE_NEWNS | unix.CLONE_NEWIPC | unix.CLONE_NEWUTS | unix.CLONE_NEWNET
	if got := compilerNamespaceFlags(cfg); got != want {
		t.Fatalf("compilerNamespaceFlags() = %#x, want %#x", got, want)
	}
}

func TestApplyCompilerRootFSPreservesCwdWithoutSandboxWorkDir(t *testing.T) {
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	if err := applyCompilerRootFS(&CompileConfig{}); err != nil {
		t.Fatalf("applyCompilerRootFS() error = %v", err)
	}

	currentWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	if currentWD != previousWD {
		t.Fatalf("cwd = %q, want %q", currentWD, previousWD)
	}
}

func TestApplyCompilerRootFSUsesExplicitWorkDirWithoutChroot(t *testing.T) {
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(previousWD); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()

	workDir := t.TempDir()
	if err := applyCompilerRootFS(&CompileConfig{WorkDir: workDir}); err != nil {
		t.Fatalf("applyCompilerRootFS() error = %v", err)
	}

	currentWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	if currentWD != workDir {
		t.Fatalf("cwd = %q, want %q", currentWD, workDir)
	}
}

func TestApplyCompilerRootFSRejectsRelativeWorkDirBeforeChroot(t *testing.T) {
	err := applyCompilerRootFS(&CompileConfig{
		ChrootDir: t.TempDir(),
		WorkDir:   "work",
	})
	if err == nil {
		t.Fatal("applyCompilerRootFS() should reject relative WorkDir with ChrootDir")
	}
	if !strings.Contains(err.Error(), "must be absolute") {
		t.Fatalf("applyCompilerRootFS() error = %v, want absolute WorkDir error", err)
	}
}

func TestDropCompilerCredentialsRejectsMismatchedUIDGID(t *testing.T) {
	err := dropCompilerCredentials(&CompileConfig{RunUID: 1000, RunGID: -1})
	if err == nil {
		t.Fatal("dropCompilerCredentials() should reject mismatched UID/GID")
	}
}

func TestDropCompilerCredentialsNoopsWhenAlreadyTargetIdentity(t *testing.T) {
	uid := os.Geteuid()
	gid := os.Getegid()
	if uid <= 0 || gid <= 0 {
		t.Skip("requires a non-root test process")
	}

	if err := dropCompilerCredentials(&CompileConfig{RunUID: uid, RunGID: gid}); err != nil {
		t.Fatalf("dropCompilerCredentials() error = %v, want nil", err)
	}
}

func TestHandleRejectsInvalidSandboxConfig(t *testing.T) {
	if _, err := runner.InitLogger("/dev/null", false); err != nil {
		t.Fatalf("InitLogger error = %v", err)
	}

	cfg := &CompileConfig{
		RunUID:   1000,
		RunGID:   -1, // mismatched: uid set, gid not set
		MaxProcs: 32,
	}

	result := handle(cfg)
	if result.Success {
		t.Fatal("handle() should return Success=false for invalid sandbox config")
	}
}

func TestCompilerCgroupCleanupKillsDetachedHelper(t *testing.T) {
	requireCompilerCgroupBackend(t)
	requireCommand(t, "sh")
	requireCommand(t, "setsid")
	requireCommand(t, "sleep")

	compilerPath := filepath.Join(t.TempDir(), "compiler")
	build := exec.Command("go", "build", "-buildvcs=false", "-o", compilerPath, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build compiler error = %v\n%s", err, output)
	}

	workDir := t.TempDir()
	if err := os.Chmod(workDir, 0o777); err != nil {
		t.Fatalf("chmod work dir: %v", err)
	}

	sleepArg := strconv.Itoa(50000 + os.Getpid()%10000)
	if pids := sleepProcessPIDsWithArg(t, sleepArg); len(pids) > 0 {
		t.Skipf("sleep %s already running as pids %v", sleepArg, pids)
	}
	t.Cleanup(func() {
		killSleepProcessesWithArg(t, sleepArg)
	})

	compileConfig := fmt.Sprintf(`{
	"CPU": 1,
	"Memory": 128,
	"Output": 16,
	"Stack": 8,
	"MaxProcs": 32,
	"Command": "sh",
	"Args": ["-c", "setsid sleep %s & exit 0"],
	"LogPath": "/dev/stderr"%s
}
`, sleepArg, compilerCleanupRunIdentityJSON())
	if err := os.WriteFile(filepath.Join(workDir, "compile.json"), []byte(compileConfig), 0o644); err != nil {
		t.Fatalf("write compile.json: %v", err)
	}

	cmd := exec.Command(compilerPath)
	cmd.Dir = workDir
	cmd.Env = compilerCleanupEnv()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiler error = %v\n%s", err, output)
	}
	if !strings.Contains(string(output), `"success":true`) {
		t.Fatalf("compiler output = %s, want success=true", output)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if pids := sleepProcessPIDsWithArg(t, sleepArg); len(pids) == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("residual detached sleep process found with arg %s", sleepArg)
}

func TestHandleMovesCompilerIntoTaskController(t *testing.T) {
	withCompilerEffectiveUID(t, 1000)

	fakeController := &fakeCompilerTaskController{}
	withCompilerTaskController(t, func(cfg *CompileConfig) (runner.TaskController, error) {
		if cfg.MaxProcs != 32 {
			t.Fatalf("MaxProcs = %d, want 32", cfg.MaxProcs)
		}
		return fakeController, nil
	})
	gateCh := make(chan byte, 1)
	withCompilerBootstrapStart(t, func(cfg *CompileConfig, gateReader *os.File) (int, error) {
		readCompilerGateInTest(t, gateReader, gateCh)
		return 4321, nil
	})

	successStatus := waitStatusForExit(t, 0)
	withCompilerWait4(t, func(pid int, status *syscall.WaitStatus, options int, rusage *syscall.Rusage) (int, error) {
		*status = successStatus
		return pid, nil
	})

	result := handle(validCompilerConfig())
	if !result.Success {
		t.Fatal("handle() Success = false, want true")
	}
	if fakeController.movedPID != 4321 {
		t.Fatalf("MovePID pid = %d, want 4321", fakeController.movedPID)
	}
	if got := <-gateCh; got != 1 {
		t.Fatalf("cgroup gate byte = %d, want 1", got)
	}
	if fakeController.cleanupCalls == 0 {
		t.Fatal("Cleanup was not called")
	}
	if !fakeController.killed {
		t.Fatal("Kill should drain the compiler task cgroup before cleanup")
	}
	wantCalls := []string{"kill", "cleanup"}
	if len(fakeController.calls) != len(wantCalls) {
		t.Fatalf("controller calls = %v, want %v", fakeController.calls, wantCalls)
	}
	for i := range wantCalls {
		if fakeController.calls[i] != wantCalls[i] {
			t.Fatalf("controller calls = %v, want %v", fakeController.calls, wantCalls)
		}
	}
}

func TestWaitForCompilerChildKillsOnTimeout(t *testing.T) {
	controller := &fakeCompilerTaskController{killCh: make(chan struct{})}
	killedStatus := waitStatusForCommand(t, "kill -KILL $$")

	withCompilerWait4(t, func(pid int, status *syscall.WaitStatus, options int, rusage *syscall.Rusage) (int, error) {
		<-controller.killCh
		*status = killedStatus
		return pid, nil
	})

	status, timedOut, err := waitForCompilerChild(4321, time.Millisecond, controller)
	if err != nil {
		t.Fatalf("waitForCompilerChild() error = %v", err)
	}
	if !timedOut {
		t.Fatal("timedOut = false, want true")
	}
	if !controller.killed {
		t.Fatal("Kill was not called on timeout")
	}
	if !status.Signaled() || status.Signal() != syscall.SIGKILL {
		t.Fatalf("status = %v, want SIGKILL", status)
	}
}

type fakeCompilerTaskController struct {
	movedPID     int
	cleanupCalls int
	killed       bool
	calls        []string
	killCh       chan struct{}
	killOnce     sync.Once
}

func (controller *fakeCompilerTaskController) MovePID(pid int) error {
	controller.movedPID = pid
	return nil
}

func (controller *fakeCompilerTaskController) Kill() error {
	controller.killed = true
	controller.calls = append(controller.calls, "kill")
	if controller.killCh != nil {
		controller.killOnce.Do(func() { close(controller.killCh) })
	}
	return nil
}

func (controller *fakeCompilerTaskController) Cleanup() error {
	controller.cleanupCalls++
	controller.calls = append(controller.calls, "cleanup")
	return nil
}

func validCompilerConfig() *CompileConfig {
	return &CompileConfig{
		CPU:        1,
		Memory:     128,
		Output:     16,
		Stack:      8,
		MaxProcs:   32,
		NoNewPrivs: true,
		RunUID:     -1,
		RunGID:     -1,
	}
}

func withCompilerTaskController(t *testing.T, factory func(*CompileConfig) (runner.TaskController, error)) {
	t.Helper()

	previous := newCompilerTaskController
	newCompilerTaskController = factory
	t.Cleanup(func() {
		newCompilerTaskController = previous
	})
}

func readCompilerGateInTest(t *testing.T, gateReader *os.File, gateCh chan<- byte) {
	t.Helper()

	dupFD, err := syscall.Dup(int(gateReader.Fd()))
	if err != nil {
		t.Fatalf("dup gate fd: %v", err)
	}
	dupFile := os.NewFile(uintptr(dupFD), "test-compiler-cgroup-gate")
	go func() {
		defer func() {
			_ = dupFile.Close()
		}()
		buf := []byte{0}
		_, _ = dupFile.Read(buf)
		gateCh <- buf[0]
	}()
}

func withCompilerBootstrapStart(t *testing.T, start func(*CompileConfig, *os.File) (int, error)) {
	t.Helper()

	previous := startCompilerBootstrapProcess
	startCompilerBootstrapProcess = start
	t.Cleanup(func() {
		startCompilerBootstrapProcess = previous
	})
}

func withCompilerWait4(t *testing.T, wait4 func(int, *syscall.WaitStatus, int, *syscall.Rusage) (int, error)) {
	t.Helper()

	previous := compilerWait4
	compilerWait4 = wait4
	t.Cleanup(func() {
		compilerWait4 = previous
	})
}

func requireCompilerCgroupBackend(t *testing.T) {
	t.Helper()

	controller, err := runner.NewCgroupTaskController(128, 32)
	if err != nil {
		t.Skipf("compiler cgroup backend unavailable: %v", err)
	}
	if err := controller.Cleanup(); err != nil {
		t.Fatalf("cleanup compiler cgroup precheck: %v", err)
	}
}

func requireCommand(t *testing.T, name string) {
	t.Helper()

	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s unavailable: %v", name, err)
	}
}

func compilerCleanupRunIdentityJSON() string {
	if os.Geteuid() != 0 {
		return ""
	}
	uid, gid := compilerCleanupRunIdentity()
	return fmt.Sprintf(",\n\t\"RunUID\": %d,\n\t\"RunGID\": %d", uid, gid)
}

func compilerCleanupRunIdentity() (int, int) {
	if uid, gid, ok := positiveEnvUIDGID("COMPILECONFIG_RUNUID", "COMPILECONFIG_RUNGID"); ok {
		return uid, gid
	}
	if uid, gid, ok := positiveEnvUIDGID("SUDO_UID", "SUDO_GID"); ok {
		return uid, gid
	}
	return 65534, 65534
}

func positiveEnvUIDGID(uidKey string, gidKey string) (int, int, bool) {
	uid, uidOK := positiveEnvInt(uidKey)
	gid, gidOK := positiveEnvInt(gidKey)
	return uid, gid, uidOK && gidOK
}

func positiveEnvInt(key string) (int, bool) {
	value := os.Getenv(key)
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return parsed, true
}

func compilerCleanupEnv() []string {
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "COMPILECONFIG_") {
			continue
		}
		switch key {
		case compilerBootstrapEnv, compilerBootstrapConfigEnv, compilerCgroupGateFDEnv:
			continue
		default:
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func sleepProcessPIDsWithArg(t *testing.T, arg string) []int {
	t.Helper()

	entries, err := os.ReadDir("/proc")
	if err != nil {
		t.Skipf("/proc unavailable: %v", err)
	}

	var pids []int
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		comm, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "comm"))
		if err != nil || strings.TrimSpace(string(comm)) != "sleep" {
			continue
		}
		cmdline, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil {
			continue
		}
		if sleepCmdlineHasArg(cmdline, arg) {
			pids = append(pids, pid)
		}
	}
	return pids
}

func sleepCmdlineHasArg(cmdline []byte, arg string) bool {
	fields := strings.Split(strings.TrimRight(string(cmdline), "\x00"), "\x00")
	if len(fields) < 2 {
		return false
	}
	return filepath.Base(fields[0]) == "sleep" && fields[1] == arg
}

func killSleepProcessesWithArg(t *testing.T, arg string) {
	t.Helper()

	for _, pid := range sleepProcessPIDsWithArg(t, arg) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}
