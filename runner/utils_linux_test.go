//go:build linux

package runner

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
)

func TestBuildMinimalEnvKeepsWhitelistedKeys(t *testing.T) {
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("HOME", "/root")

	env := BuildMinimalEnv()
	found := map[string]bool{}
	for _, entry := range env {
		parts := strings.SplitN(entry, "=", 2)
		found[parts[0]] = true
	}
	if !found["PATH"] {
		t.Fatal("BuildMinimalEnv() dropped PATH")
	}
	if !found["HOME"] {
		t.Fatal("BuildMinimalEnv() dropped HOME")
	}
}

func TestBuildMinimalEnvRemovesDropKeysAndUnsafeEntries(t *testing.T) {
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("RUNNER_BOOTSTRAP", "1")
	t.Setenv("LD_PRELOAD", "bad.so")

	for _, entry := range BuildMinimalEnv("RUNNER_BOOTSTRAP") {
		if strings.HasPrefix(entry, "RUNNER_BOOTSTRAP=") {
			t.Fatalf("BuildMinimalEnv() kept dropped key: %q", entry)
		}
		if strings.HasPrefix(entry, "LD_PRELOAD=") {
			t.Fatalf("BuildMinimalEnv() kept unsafe env: %q", entry)
		}
	}
}

func TestCloseNonStdioFiles(t *testing.T) {
	if os.Getenv("TEST_CLOSE_FDS") == "1" {
		f, err := os.Open("/dev/null")
		if err != nil {
			fmt.Fprintf(os.Stderr, "open: %v", err)
			os.Exit(2)
		}
		fd := int(f.Fd())
		if fd <= 2 {
			os.Exit(3)
		}
		if err := CloseNonStdioFiles(); err != nil {
			fmt.Fprintf(os.Stderr, "close: %v", err)
			os.Exit(4)
		}
		var stat syscall.Stat_t
		if syscall.Fstat(fd, &stat) == nil {
			os.Exit(5)
		}
		os.Exit(0)
	}

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}

	cmd := &exec.Cmd{
		Path: self,
		Args: []string{self, "-test.run=^TestCloseNonStdioFiles$"},
		Env:  append(os.Environ(), "TEST_CLOSE_FDS=1"),
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CloseNonStdioFiles subprocess failed: %v\noutput: %s", err, out)
	}
}
