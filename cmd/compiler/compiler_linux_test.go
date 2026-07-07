//go:build linux

package main

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"

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

func TestHandleRejectsInvalidSandboxConfig(t *testing.T) {
	if _, err := runner.InitLogger("/dev/null", false); err != nil {
		t.Fatalf("InitLogger error = %v", err)
	}

	cfg := &CompileConfig{
		RunUID: 1000,
		RunGID: -1, // mismatched: uid set, gid not set
	}

	result := handle(cfg)
	if result.Success {
		t.Fatal("handle() should return Success=false for invalid sandbox config")
	}
}
