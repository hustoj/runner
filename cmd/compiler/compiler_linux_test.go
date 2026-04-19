//go:build linux

package main

import (
	"os/exec"
	"strconv"
	"syscall"
	"testing"
)

func TestResolveExecIncludesBinary(t *testing.T) {
	wantBinary, err := exec.LookPath("sh")
	if err != nil {
		t.Fatalf("exec.LookPath(sh) error = %v", err)
	}

	cfg := &CompileConfig{
		Command: "sh",
		Args:    `-c 'printf ok'`,
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

func TestHandleReturnsFailureWhenCompilerBinaryMissing(t *testing.T) {
	cfg := &CompileConfig{
		Command: "definitely-missing-compiler-binary",
		Args:    "",
	}

	result := handle(cfg)
	if result == nil {
		t.Fatal("handle() returned nil result")
	}
	if result.Success {
		t.Fatal("handle() should report failure when compiler binary is missing")
	}
}
