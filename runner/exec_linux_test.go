//go:build linux

package runner

import (
	"os"
	"path/filepath"
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
			CPU:           2,
			Memory:        64,
			MemoryReserve: 32,
			Output:        16,
			Stack:         8,
			Command:       "/bin/true",
			RunUID:        -1,
			RunGID:        -1,
			NoNewPrivs:    true,
		},
		memoryLimit: 64 * 1024,
	}

	spec, err := task.prepareChildProcessSpec()
	if err != nil {
		t.Fatalf("prepareChildProcessSpec() error = %v", err)
	}
	defer closeChildIOFiles(spec.io)

	const (
		wantStackLimit      = uint64(8) << 20
		wantHardMemoryLimit = uint64(96) << 20
		wantNoFileLimit     = uint64(16)
		wantCoreLimit       = uint64(0)
	)

	if spec.stackLimit.Cur != wantStackLimit || spec.stackLimit.Max != wantStackLimit {
		t.Fatalf(
			"prepareChildProcessSpec() stackLimit = {Cur:%d Max:%d}, want {Cur:%d Max:%d}",
			spec.stackLimit.Cur,
			spec.stackLimit.Max,
			wantStackLimit,
			wantStackLimit,
		)
	}
	if spec.hardMemoryLimit.Cur != wantHardMemoryLimit || spec.hardMemoryLimit.Max != wantHardMemoryLimit {
		t.Fatalf(
			"prepareChildProcessSpec() hardMemoryLimit = {Cur:%d Max:%d}, want {Cur:%d Max:%d}",
			spec.hardMemoryLimit.Cur,
			spec.hardMemoryLimit.Max,
			wantHardMemoryLimit,
			wantHardMemoryLimit,
		)
	}
	if spec.hardMemoryLimit.Cur == (uint64(64) << 20) {
		t.Fatalf("prepareChildProcessSpec() hardMemoryLimit should include MemoryReserve, got %d", spec.hardMemoryLimit.Cur)
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
