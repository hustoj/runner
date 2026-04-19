//go:build linux

package runner

import (
	"os"
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
}
