//go:build linux

package main

import (
	"strings"
	"syscall"
	"testing"

	"github.com/hustoj/runner/runner"
)

func TestResolveExecIncludesBinary(t *testing.T) {
	cfg := &CompileConfig{
		Command: "/bin/echo",
		Args:    "'hello world'",
	}

	binary, args, err := cfg.ResolveExec()
	if err != nil {
		t.Fatalf("ResolveExec() error = %v", err)
	}
	if binary != "/bin/echo" {
		t.Fatalf("ResolveExec() binary = %q, want %q", binary, "/bin/echo")
	}
	if len(args) != 2 {
		t.Fatalf("ResolveExec() args len = %d, want 2", len(args))
	}
	if args[0] != "/bin/echo" {
		t.Fatalf("ResolveExec() argv[0] = %q, want %q", args[0], "/bin/echo")
	}
	if args[1] != "hello world" {
		t.Fatalf("ResolveExec() argv[1] = %q, want %q", args[1], "hello world")
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

func TestCompileSucceeded(t *testing.T) {
	if !compileSucceeded(syscall.WaitStatus(0)) {
		t.Fatal("compileSucceeded() = false, want true for exit status 0")
	}
	if compileSucceeded(syscall.WaitStatus(1 << 8)) {
		t.Fatal("compileSucceeded() = true, want false for non-zero exit")
	}
	if compileSucceeded(syscall.WaitStatus(9)) {
		t.Fatal("compileSucceeded() = true, want false for signal exit")
	}
}
