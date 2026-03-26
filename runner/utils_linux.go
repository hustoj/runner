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
	if err := syscall.Dup2(int(f1.Fd()), int(f2.Fd())); err != nil {
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
// This prevents the child process from inheriting open files (config, log,
// network sockets, etc.) that belong to the parent.
func CloseNonStdioFiles() error {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return err
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

// StartBootstrapChild launches a new instance of the current binary with a
// bootstrap marker injected into the environment. The child recognises the
// marker and enters its bootstrap path instead of the normal parent flow.
// This is the shared implementation used by both runner and compiler.
func StartBootstrapChild(markerKey string) (int, error) {
	self, err := os.Executable()
	if err != nil {
		return 0, err
	}

	env := append(BuildMinimalEnv(markerKey), markerKey+"=1")
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

func ChangeRunningUser(user int) {
	err := syscall.Setuid(user)
	if err != nil {
		log.Panicf("set running uid failed %v", err)
	}
}
