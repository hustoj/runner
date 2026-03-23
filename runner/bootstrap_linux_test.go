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

func closeTestFile(t *testing.T, file *os.File) {
	t.Helper()
	if err := file.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func cleanupRemoveFile(t *testing.T, path string) {
	t.Helper()
	t.Cleanup(func() {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Fatalf("Remove %q: %v", path, err)
		}
	})
}

func TestResolveExecIncludesArgv0(t *testing.T) {
	task := &TaskConfig{Command: "/bin/echo 'hello world'"}

	binary, args, err := task.ResolveExec()
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

func TestResolveExecEmptyCommand(t *testing.T) {
	task := &TaskConfig{Command: ""}
	_, _, err := task.ResolveExec()
	if err == nil {
		t.Fatal("ResolveExec() should fail for empty command")
	}
}

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

func TestBuildMinimalEnvRemovesBootstrapMarker(t *testing.T) {
	t.Setenv(runnerBootstrapEnv, "1")
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("LD_PRELOAD", "bad.so")

	for _, entry := range BuildMinimalEnv(runnerBootstrapEnv) {
		if strings.HasPrefix(entry, runnerBootstrapEnv+"=") {
			t.Fatalf("BuildMinimalEnv() kept bootstrap marker: %q", entry)
		}
		if strings.HasPrefix(entry, "LD_PRELOAD=") {
			t.Fatalf("BuildMinimalEnv() kept unsafe env: %q", entry)
		}
	}
}

func TestCloseNonStdioFiles(t *testing.T) {
	// We run CloseNonStdioFiles in a subprocess because it closes ALL fds > 2,
	// which would break the test framework's own internal file descriptors.
	if os.Getenv("TEST_CLOSE_FDS") == "1" {
		// Open a file to get fd > 2, then close all non-stdio fds
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
		// Verify the fd is now closed
		var stat syscall.Stat_t
		if syscall.Fstat(fd, &stat) == nil {
			os.Exit(5) // still open = failure
		}
		os.Exit(0) // success
	}

	// Parent: launch ourselves as a subprocess with the marker set
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("Executable: %v", err)
	}
	cmd := &exec.Cmd{
		Path: self,
		Args: []string{self, "-test.run=^TestCloseNonStdioFiles$"},
		Env:  append(os.Environ(), "TEST_CLOSE_FDS=1"),
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("subprocess failed: %v\noutput: %s", err, out)
	}
}

func TestOpenFileNoFollow(t *testing.T) {
	tmp, err := os.CreateTemp("", "nofollow-test")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	closeTestFile(t, tmp)
	cleanupRemoveFile(t, tmp.Name())

	f, err := openFileNoFollow(tmp.Name(), syscall.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("openFileNoFollow() error = %v", err)
	}
	closeTestFile(t, f)
}

func TestOpenFileNoFollowRejectsSymlink(t *testing.T) {
	tmp, err := os.CreateTemp("", "nofollow-target")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	closeTestFile(t, tmp)
	cleanupRemoveFile(t, tmp.Name())

	link := tmp.Name() + ".link"
	if err := os.Symlink(tmp.Name(), link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	cleanupRemoveFile(t, link)

	_, err = openFileNoFollow(link, syscall.O_RDONLY, 0)
	if err == nil {
		t.Fatal("openFileNoFollow() should reject symlink, but succeeded")
	}
}
