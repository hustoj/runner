//go:build linux

package runner

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestPrepareChildProcessSpecUsesConfiguredResourceLimits(t *testing.T) {
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("os.Chdir(%q) error = %v", tempDir, err)
	}
	defer func() {
		if err := os.Chdir(previousWD); err != nil {
			t.Fatalf("restore working directory error = %v", err)
		}
	}()

	if err := os.WriteFile(filepath.Join(tempDir, "user.in"), []byte(""), 0o600); err != nil {
		t.Fatalf("os.WriteFile(user.in) error = %v", err)
	}

	task := RunningTask{
		setting: &TaskConfig{
			CPU:        2,
			Memory:     64,
			Output:     16,
			Stack:      8,
			Command:    "/bin/true",
			RunUID:     -1,
			RunGID:     -1,
			NoNewPrivs: true,
		},
		memoryLimit: 64 * 1024,
	}

	spec, err := task.prepareChildProcessSpec()
	if err != nil {
		t.Fatalf("prepareChildProcessSpec() error = %v", err)
	}
	defer closeChildIOFiles(spec.io)

	const (
		wantCPULimit    = uint64(3)
		wantStackLimit  = uint64(8) << 20
		wantOutputLimit = uint64(16) << 20
		wantNoFileLimit = uint64(16)
		wantCoreLimit   = uint64(0)
	)

	if spec.cpuLimit.Cur != wantCPULimit || spec.cpuLimit.Max != wantCPULimit {
		t.Fatalf(
			"prepareChildProcessSpec() cpuLimit = {Cur:%d Max:%d}, want {Cur:%d Max:%d}",
			spec.cpuLimit.Cur,
			spec.cpuLimit.Max,
			wantCPULimit,
			wantCPULimit,
		)
	}
	if spec.outputLimit.Cur != wantOutputLimit || spec.outputLimit.Max != wantOutputLimit {
		t.Fatalf(
			"prepareChildProcessSpec() outputLimit = {Cur:%d Max:%d}, want {Cur:%d Max:%d}",
			spec.outputLimit.Cur,
			spec.outputLimit.Max,
			wantOutputLimit,
			wantOutputLimit,
		)
	}
	if spec.stackLimit.Cur != wantStackLimit || spec.stackLimit.Max != wantStackLimit {
		t.Fatalf(
			"prepareChildProcessSpec() stackLimit = {Cur:%d Max:%d}, want {Cur:%d Max:%d}",
			spec.stackLimit.Cur,
			spec.stackLimit.Max,
			wantStackLimit,
			wantStackLimit,
		)
	}
	if spec.noFileLimit.Cur != wantNoFileLimit || spec.noFileLimit.Max != wantNoFileLimit {
		t.Fatalf(
			"prepareChildProcessSpec() noFileLimit = {Cur:%d Max:%d}, want {Cur:%d Max:%d}",
			spec.noFileLimit.Cur,
			spec.noFileLimit.Max,
			wantNoFileLimit,
			wantNoFileLimit,
		)
	}
	if spec.coreLimit.Cur != wantCoreLimit || spec.coreLimit.Max != wantCoreLimit {
		t.Fatalf(
			"prepareChildProcessSpec() coreLimit = {Cur:%d Max:%d}, want {Cur:%d Max:%d}",
			spec.coreLimit.Cur,
			spec.coreLimit.Max,
			wantCoreLimit,
			wantCoreLimit,
		)
	}
}

func TestReleaseChildCgroupGateRetriesEINTR(t *testing.T) {
	original := writeChildCgroupGate
	t.Cleanup(func() {
		writeChildCgroupGate = original
	})

	calls := 0
	writeChildCgroupGate = func(fd int, data []byte) (int, error) {
		calls++
		if calls == 1 {
			return 0, syscall.EINTR
		}
		if fd != -1 {
			t.Fatalf("write fd = %d, want -1 test fd", fd)
		}
		if len(data) != 1 || data[0] != 1 {
			t.Fatalf("gate data = %v, want [1]", data)
		}
		return 1, nil
	}

	if err := releaseChildCgroupGate(-1); err != nil {
		t.Fatalf("releaseChildCgroupGate() error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("write calls = %d, want 2", calls)
	}
}

func TestWaitChildStartupFailureRetriesEINTR(t *testing.T) {
	original := wait4ChildStartup
	t.Cleanup(func() {
		wait4ChildStartup = original
	})

	calls := 0
	wait4ChildStartup = func(pid int, status *syscall.WaitStatus, options int, rusage *syscall.Rusage) (int, error) {
		calls++
		if pid != 1234 {
			t.Fatalf("wait pid = %d, want 1234", pid)
		}
		if options != 0 {
			t.Fatalf("wait options = %d, want 0", options)
		}
		if calls == 1 {
			return 0, syscall.EINTR
		}
		return pid, nil
	}

	waitChildStartupFailure(1234)
	if calls != 2 {
		t.Fatalf("wait calls = %d, want 2", calls)
	}
}

func TestOpenChildIOFilesFailsWhenInputMissing(t *testing.T) {
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("os.Chdir(%q) error = %v", tempDir, err)
	}
	defer func() {
		if err := os.Chdir(previousWD); err != nil {
			t.Fatalf("restore working directory error = %v", err)
		}
	}()

	_, err = openChildIOFiles()
	if err == nil {
		t.Fatal("openChildIOFiles() error = nil, want missing user.in error")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("openChildIOFiles() error = %v, want not-exist", err)
	}
}

func TestOpenChildIOFilesRejectsSymlinkInput(t *testing.T) {
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("os.Chdir(%q) error = %v", tempDir, err)
	}
	defer func() {
		if err := os.Chdir(previousWD); err != nil {
			t.Fatalf("restore working directory error = %v", err)
		}
	}()

	target := filepath.Join(tempDir, "stdin.txt")
	if err := os.WriteFile(target, []byte("hello\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", target, err)
	}
	if err := os.Symlink(target, filepath.Join(tempDir, "user.in")); err != nil {
		t.Fatalf("os.Symlink() error = %v", err)
	}

	_, err = openChildIOFiles()
	if err == nil {
		t.Fatal("openChildIOFiles() error = nil, want symlink rejection")
	}
}

func TestOpenChildIOFilesCreatesSecureOutputFiles(t *testing.T) {
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("os.Chdir(%q) error = %v", tempDir, err)
	}
	defer func() {
		if err := os.Chdir(previousWD); err != nil {
			t.Fatalf("restore working directory error = %v", err)
		}
	}()

	if err := os.WriteFile(filepath.Join(tempDir, "user.in"), []byte("stdin\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(user.in) error = %v", err)
	}

	ioFiles, err := openChildIOFiles()
	if err != nil {
		t.Fatalf("openChildIOFiles() error = %v", err)
	}
	defer closeChildIOFiles(ioFiles)

	for _, name := range []string{"user.out", "user.err"} {
		info, err := os.Stat(filepath.Join(tempDir, name))
		if err != nil {
			t.Fatalf("os.Stat(%q) error = %v", name, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s perms = %o, want 600", name, info.Mode().Perm())
		}
	}
}
