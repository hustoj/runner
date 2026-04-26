//go:build linux

package runner

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func TestRunProcessSetsNoNewPrivsBeforeTraceLoop(t *testing.T) {
	runInSandboxWorkspace(t, func(_ string) {
		task, cleanup, err := startSandboxedTask(t, sandboxRunProcessConfig("/bin/true"))
		if err != nil {
			t.Fatalf("runProcess() error = %v", err)
		}
		defer cleanup()

		status, err := readProcStatusFields(task.process.Pid, "NoNewPrivs")
		if err != nil {
			t.Fatalf("readProcStatusFields(NoNewPrivs) error = %v", err)
		}
		if firstField(status["NoNewPrivs"]) != "1" {
			t.Fatalf("NoNewPrivs = %q, want %q", firstField(status["NoNewPrivs"]), "1")
		}
	})
}

func TestRunProcessPlacesChildInDedicatedTaskCgroup(t *testing.T) {
	runInSandboxWorkspace(t, func(_ string) {
		cfg := sandboxRunProcessConfig("/bin/true")

		task, cleanup, err := startSandboxedTask(t, cfg)
		if err != nil {
			t.Fatalf("runProcess() error = %v", err)
		}
		defer cleanup()

		controller, ok := task.taskCtrl.(*cgroupTaskController)
		if !ok {
			t.Fatalf("task controller = %T, want *cgroupTaskController", task.taskCtrl)
		}

		mountRoot, err := detectCgroupMountRoot()
		if err != nil {
			t.Fatalf("detectCgroupMountRoot() error = %v", err)
		}
		gotCgroupPath, err := readProcCgroupV2Path(task.process.Pid)
		if err != nil {
			t.Fatalf("readProcCgroupV2Path() error = %v", err)
		}
		wantCgroupPath := "/" + strings.TrimPrefix(strings.TrimPrefix(controller.path, mountRoot), string(os.PathSeparator))
		if gotCgroupPath != wantCgroupPath {
			t.Fatalf("child cgroup path = %q, want %q", gotCgroupPath, wantCgroupPath)
		}

		content, err := os.ReadFile(filepath.Join(controller.path, "memory.max"))
		if err != nil {
			t.Fatalf("os.ReadFile(memory.max) error = %v", err)
		}
		wantLimit := strconv.FormatUint(uint64(cfg.Memory)<<20, 10)
		if strings.TrimSpace(string(content)) != wantLimit {
			t.Fatalf("memory.max = %q, want %q", strings.TrimSpace(string(content)), wantLimit)
		}

		pidsContent, err := os.ReadFile(filepath.Join(controller.path, "pids.max"))
		if err != nil {
			t.Fatalf("os.ReadFile(pids.max) error = %v", err)
		}
		if strings.TrimSpace(string(pidsContent)) != strconv.Itoa(cfg.MaxProcs) {
			t.Fatalf("pids.max = %q, want %q", strings.TrimSpace(string(pidsContent)), strconv.Itoa(cfg.MaxProcs))
		}
	})
}

func TestRunProcessPropagatesSandboxPermissionFailures(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires non-root to validate privileged sandbox failure propagation")
	}

	t.Run("chroot", func(t *testing.T) {
		runInSandboxWorkspace(t, func(tempDir string) {
			cfg := sandboxRunProcessConfig("/bin/true")
			cfg.ChrootDir = filepath.Join(tempDir, "jail")
			if err := os.MkdirAll(cfg.ChrootDir, 0o755); err != nil {
				t.Fatalf("os.MkdirAll(%q) error = %v", cfg.ChrootDir, err)
			}

			_, _, err := startSandboxedTask(t, cfg)
			assertSandboxStartupError(t, err, "chroot", syscall.EPERM)
		})
	})

	t.Run("mount namespace", func(t *testing.T) {
		runInSandboxWorkspace(t, func(_ string) {
			cfg := sandboxRunProcessConfig("/bin/true")
			cfg.UseMountNS = true

			_, _, err := startSandboxedTask(t, cfg)
			assertSandboxStartupError(t, err, "setup namespaces", syscall.EPERM)
		})
	})
}

func TestRunProcessSetsCredentialsBeforeTraceLoop(t *testing.T) {
	requireSandboxRoot(t)

	runInSandboxWorkspace(t, func(_ string) {
		const targetID = 65534

		cfg := sandboxRunProcessConfig("/bin/true")
		cfg.RunUID = targetID
		cfg.RunGID = targetID

		task, cleanup, err := startSandboxedTask(t, cfg)
		if err != nil {
			t.Fatalf("runProcess() error = %v", err)
		}
		defer cleanup()

		status, err := readProcStatusFields(task.process.Pid, "Uid", "Gid")
		if err != nil {
			t.Fatalf("readProcStatusFields(Uid,Gid) error = %v", err)
		}
		if !allFieldsEqual(status["Uid"], targetID) {
			t.Fatalf("Uid status = %v, want all %d", status["Uid"], targetID)
		}
		if !allFieldsEqual(status["Gid"], targetID) {
			t.Fatalf("Gid status = %v, want all %d", status["Gid"], targetID)
		}
	})
}

func TestRunProcessCreatesMountNamespaceBeforeTraceLoop(t *testing.T) {
	requireSandboxRoot(t)

	parentMountNS, err := os.Readlink("/proc/self/ns/mnt")
	if err != nil {
		t.Fatalf("os.Readlink(/proc/self/ns/mnt) error = %v", err)
	}

	runInSandboxWorkspace(t, func(_ string) {
		cfg := sandboxRunProcessConfig("/bin/true")
		cfg.UseMountNS = true

		task, cleanup, err := startSandboxedTask(t, cfg)
		if err != nil {
			if isSandboxStartupPermissionError(err, "setup namespaces", syscall.EPERM) {
				t.Skipf("mount namespace is unavailable in current environment: %v", err)
			}
			t.Fatalf("runProcess() error = %v", err)
		}
		defer cleanup()

		mountNS, err := readProcLink(task.process.Pid, "ns/mnt")
		if err != nil {
			t.Fatalf("readProcLink(ns/mnt) error = %v", err)
		}
		if mountNS == parentMountNS {
			t.Fatalf("mount namespace = %q, want different from parent %q", mountNS, parentMountNS)
		}
	})
}

func TestRunProcessAppliesChrootAndWorkDirBeforeTraceLoop(t *testing.T) {
	requireSandboxRoot(t)

	runInSandboxWorkspace(t, func(tempDir string) {
		jailDir := filepath.Join(tempDir, "jail")
		workDir := filepath.Join(jailDir, "work")
		if err := os.MkdirAll(workDir, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v", workDir, err)
		}

		self, err := os.Executable()
		if err != nil {
			t.Fatalf("os.Executable() error = %v", err)
		}
		jailBinary := filepath.Join(jailDir, "helper")
		if err := copyExecutable(self, jailBinary); err != nil {
			t.Fatalf("copyExecutable(%q,%q) error = %v", self, jailBinary, err)
		}

		cfg := sandboxRunProcessConfig("/helper")
		cfg.ChrootDir = jailDir
		cfg.WorkDir = "/work"

		task, cleanup, err := startSandboxedTask(t, cfg)
		if err != nil {
			if isSandboxStartupExecUnavailable(err) {
				t.Skipf("copied test binary is not executable inside chroot jail: %v", err)
			}
			t.Fatalf("runProcess() error = %v", err)
		}
		defer cleanup()

		rootPath, err := readProcLink(task.process.Pid, "root")
		if err != nil {
			t.Fatalf("readProcLink(root) error = %v", err)
		}
		if rootPath != jailDir {
			t.Fatalf("proc root = %q, want %q", rootPath, jailDir)
		}

		cwdPath, err := readProcLink(task.process.Pid, "cwd")
		if err != nil {
			t.Fatalf("readProcLink(cwd) error = %v", err)
		}
		wantCWD := filepath.Join(jailDir, "work")
		if cwdPath != wantCWD {
			t.Fatalf("proc cwd = %q, want %q", cwdPath, wantCWD)
		}
	})
}

func sandboxRunProcessConfig(command string) *TaskConfig {
	return &TaskConfig{
		CPU:        1,
		Memory:     64,
		Output:     16,
		Stack:      8,
		MaxProcs:   16,
		Command:    command,
		RunUID:     -1,
		RunGID:     -1,
		NoNewPrivs: true,
	}
}

func startSandboxedTask(t *testing.T, cfg *TaskConfig) (*RunningTask, func(), error) {
	t.Helper()

	if _, err := InitLogger("/dev/null", false); err != nil {
		t.Fatalf("InitLogger(/dev/null) error = %v", err)
	}

	task := &RunningTask{}
	task.Init(cfg)
	if err := task.runProcess(); err != nil {
		if isTaskCgroupSetupError(err) {
			t.Skipf("task cgroup backend unavailable: %v", err)
		}
		return nil, func() {}, err
	}

	cleanup := func() {
		if task.process != nil {
			task.process.Kill()
		}
		task.cleanupRuntimeResources()
	}
	return task, cleanup, nil
}

func runInSandboxWorkspace(t *testing.T, fn func(tempDir string)) {
	t.Helper()

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

	if err := os.WriteFile("user.in", []byte(""), 0o600); err != nil {
		t.Fatalf("os.WriteFile(user.in) error = %v", err)
	}

	fn(tempDir)
}

func readProcStatusFields(pid int, keys ...string) (map[string][]string, error) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "status"))
	if err != nil {
		return nil, err
	}

	result := make(map[string][]string, len(keys))
	for _, key := range keys {
		prefix := key + ":"
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, prefix) {
				continue
			}
			result[key] = strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, prefix)))
			break
		}
		if len(result[key]) == 0 {
			return nil, fmt.Errorf("status key %q not found in /proc/%d/status", key, pid)
		}
	}
	return result, nil
}

func readProcLink(pid int, name string) (string, error) {
	return os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), name))
}

func readProcCgroupV2Path(pid int) (string, error) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cgroup"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "0::") {
			return strings.TrimSpace(strings.TrimPrefix(line, "0::")), nil
		}
	}
	return "", fmt.Errorf("cgroup v2 path not found in /proc/%d/cgroup", pid)
}

func firstField(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func allFieldsEqual(values []string, want int) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		got, err := strconv.Atoi(value)
		if err != nil || got != want {
			return false
		}
	}
	return true
}

func copyExecutable(src string, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func requireSandboxRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("requires root on linux to validate privileged sandbox behavior")
	}
}

func assertSandboxStartupError(t *testing.T, err error, stage string, errno syscall.Errno) {
	t.Helper()
	if err == nil {
		t.Fatalf("runProcess() unexpectedly succeeded, want child startup failure at %s", stage)
	}
	if !isSandboxStartupPermissionError(err, stage, errno) {
		t.Fatalf("runProcess() error = %v, want child startup failure at %s: %v", err, stage, errno)
	}
}

func isSandboxStartupPermissionError(err error, stage string, errno syscall.Errno) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "child startup failed at "+stage) && strings.Contains(message, errno.Error())
}

func isSandboxStartupExecUnavailable(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	if !strings.Contains(message, "child startup failed at execve") {
		return false
	}
	return strings.Contains(message, syscall.ENOENT.Error()) || strings.Contains(message, syscall.ENOEXEC.Error())
}

func isTaskCgroupSetupError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "setup task cgroup")
}
