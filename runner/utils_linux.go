//go:build linux

package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func fileDupErr(f1 *os.File, f2 *os.File) error {
	if err := syscall.Dup3(int(f1.Fd()), int(f2.Fd()), 0); err != nil {
		_ = f1.Close()
		return err
	}
	return f1.Close()
}

var defaultExecEnvKeys = map[string]struct{}{
	"HOME":   {},
	"LANG":   {},
	"LC_ALL": {},
	"PATH":   {},
	"TMPDIR": {},
	"TZ":     {},
}

func BuildMinimalEnv(dropKeys ...string) []string {
	dropSet := make(map[string]struct{}, len(dropKeys))
	for _, key := range dropKeys {
		dropSet[key] = struct{}{}
	}

	env := os.Environ()
	filtered := make([]string, 0, len(defaultExecEnvKeys))
	for _, entry := range env {
		parts := strings.SplitN(entry, "=", 2)
		key := parts[0]
		if _, drop := dropSet[key]; drop {
			continue
		}
		if _, allowed := defaultExecEnvKeys[key]; !allowed {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

// CloseNonStdioFiles closes all file descriptors above stderr (fd > 2).
// This prevents the child process from inheriting open files that belong to the parent.
func CloseNonStdioFiles() error {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return closeNonStdioFilesByLimit()
	}
	for _, entry := range entries {
		fd, err := strconv.Atoi(entry.Name())
		if err != nil || fd <= 2 {
			continue
		}
		if err := syscall.Close(fd); err != nil && err != syscall.EBADF {
			return err
		}
	}
	return nil
}

func closeNonStdioFilesByLimit() error {
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return err
	}
	return closeNonStdioFilesUpTo(limit.Cur)
}

func closeNonStdioFilesUpTo(maxFD uint64) error {
	for fd := uint64(3); fd < maxFD; fd++ {
		if err := syscall.Close(int(fd)); err != nil && err != syscall.EBADF {
			return err
		}
	}
	return nil
}

// StartBootstrapChild launches a new instance of the current binary with a bootstrap marker.
func StartBootstrapChild(markerKey string) (int, error) {
	return StartBootstrapChildWithEnv(markerKey, nil)
}

// StartBootstrapChildWithEnv launches a new instance of the current binary with
// a bootstrap marker plus explicit extra environment entries. Sandbox setup is
// intentionally left to the bootstrap child so callers can read host-side config
// before entering chroot or dropping privileges.
func StartBootstrapChildWithEnv(markerKey string, extraEnv []string) (int, error) {
	self, err := os.Executable()
	if err != nil {
		return 0, err
	}

	dropKeys := []string{markerKey}
	for _, entry := range extraEnv {
		key := strings.SplitN(entry, "=", 2)[0]
		dropKeys = append(dropKeys, key)
	}

	env := append(BuildMinimalEnv(dropKeys...), markerKey+"=1")
	env = append(env, extraEnv...)
	proc, err := os.StartProcess(self, []string{self}, &os.ProcAttr{
		Env: env,
		Files: []*os.File{
			os.Stdin,
			os.Stdout,
			os.Stderr,
		},
		Sys: &syscall.SysProcAttr{Setpgid: true},
	})
	if err != nil {
		return 0, err
	}
	return proc.Pid, nil
}

func openFileNoFollow(filename string, flags int, perm uint32) (*os.File, error) {
	fd, err := syscall.Open(filepath.Clean(filename), flags|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, perm)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), filename)
	if file == nil {
		_ = syscall.Close(fd)
		return nil, fmt.Errorf("new file for %s returned nil", filename)
	}
	return file, nil
}
