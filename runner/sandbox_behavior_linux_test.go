//go:build linux

package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

const (
	sandboxBehaviorHelperEnv      = "RUNNER_SANDBOX_BEHAVIOR_HELPER"
	sandboxBehaviorModeEnv        = "RUNNER_SANDBOX_BEHAVIOR_MODE"
	sandboxBehaviorJailEnv        = "RUNNER_SANDBOX_BEHAVIOR_JAIL"
	sandboxBehaviorWorkDirEnv     = "RUNNER_SANDBOX_BEHAVIOR_WORKDIR"
	sandboxBehaviorInsidePathEnv  = "RUNNER_SANDBOX_BEHAVIOR_INSIDE_PATH"
	sandboxBehaviorOutsidePathEnv = "RUNNER_SANDBOX_BEHAVIOR_OUTSIDE_PATH"
	sandboxBehaviorTargetUIDEnv   = "RUNNER_SANDBOX_BEHAVIOR_TARGET_UID"
	sandboxBehaviorTargetGIDEnv   = "RUNNER_SANDBOX_BEHAVIOR_TARGET_GID"

	sandboxBehaviorFailureExitCode = 86
)

type sandboxBehaviorResult struct {
	NoNewPrivs     string   `json:"no_new_privs,omitempty"`
	CWD            string   `json:"cwd,omitempty"`
	InsideFile     string   `json:"inside_file,omitempty"`
	OutsideVisible bool     `json:"outside_visible,omitempty"`
	OutsideError   string   `json:"outside_error,omitempty"`
	UID            int      `json:"uid,omitempty"`
	EUID           int      `json:"euid,omitempty"`
	GID            int      `json:"gid,omitempty"`
	EGID           int      `json:"egid,omitempty"`
	UIDStatus      []string `json:"uid_status,omitempty"`
	GIDStatus      []string `json:"gid_status,omitempty"`
	MountNS        string   `json:"mount_ns,omitempty"`
}

func TestSandboxBehaviorHelperProcess(t *testing.T) {
	if os.Getenv(sandboxBehaviorHelperEnv) != "1" {
		return
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	mode := os.Getenv(sandboxBehaviorModeEnv)
	cfg, err := sandboxBehaviorConfig(mode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sandbox helper config failed: %v\n", err)
		os.Exit(2)
	}

	spec, err := prepareChildSandboxSpec(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepareChildSandboxSpec failed: %v\n", err)
		os.Exit(3)
	}

	if failure := applySandboxInChild(spec); failure.failed() {
		fmt.Fprintf(os.Stderr, "sandbox startup failed at %s: %v\n", failure.stage, failure.errno)
		os.Exit(sandboxBehaviorFailureExitCode)
	}

	result, err := sandboxBehaviorProbe(mode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sandbox helper probe failed: %v\n", err)
		os.Exit(4)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "encode sandbox helper result failed: %v\n", err)
		os.Exit(5)
	}
}

func TestSandboxNoNewPrivsBehavior(t *testing.T) {
	result, output, err := runSandboxBehaviorHelper(t, "no_new_privs", nil)
	if err != nil {
		t.Fatalf("sandbox helper failed: %v\noutput: %s", err, output)
	}
	if result.NoNewPrivs != "1" {
		t.Fatalf("NoNewPrivs = %q, want %q", result.NoNewPrivs, "1")
	}
}

func TestSandboxPrivilegedPathsRequirePrivilegesWhenUnprivileged(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires non-root to validate permission failures")
	}

	t.Run("chroot", func(t *testing.T) {
		tempDir := t.TempDir()
		jailDir := filepath.Join(tempDir, "jail")
		workDir := filepath.Join(jailDir, "work")
		if err := os.MkdirAll(workDir, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v", workDir, err)
		}

		_, output, err := runSandboxBehaviorHelper(t, "chroot_workdir", map[string]string{
			sandboxBehaviorJailEnv:    jailDir,
			sandboxBehaviorWorkDirEnv: "/work",
		})
		if err == nil {
			t.Fatal("sandbox helper unexpectedly succeeded without chroot privilege")
		}
		assertSandboxPermissionFailure(t, output, "chroot")
	})

	t.Run("mount namespace", func(t *testing.T) {
		_, output, err := runSandboxBehaviorHelper(t, "mount_namespace", nil)
		if err == nil {
			t.Fatal("sandbox helper unexpectedly succeeded without namespace privilege")
		}
		assertSandboxPermissionFailure(t, output, "setup namespaces")
	})
}

func TestSandboxChrootAndWorkDirBehavior(t *testing.T) {
	requireSandboxRoot(t)

	tempDir := t.TempDir()
	jailDir := filepath.Join(tempDir, "jail")
	workDir := filepath.Join(jailDir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", workDir, err)
	}

	insidePath := filepath.Join(workDir, "inside.txt")
	if err := os.WriteFile(insidePath, []byte("inside\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", insidePath, err)
	}

	outsidePath := filepath.Join(tempDir, "outside.txt")
	if err := os.WriteFile(outsidePath, []byte("outside\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", outsidePath, err)
	}

	result, output, err := runSandboxBehaviorHelper(t, "chroot_workdir", map[string]string{
		sandboxBehaviorJailEnv:        jailDir,
		sandboxBehaviorWorkDirEnv:     "/work",
		sandboxBehaviorInsidePathEnv:  "/work/inside.txt",
		sandboxBehaviorOutsidePathEnv: outsidePath,
	})
	if err != nil {
		t.Fatalf("sandbox helper failed: %v\noutput: %s", err, output)
	}
	if result.CWD != "/work" {
		t.Fatalf("cwd = %q, want %q", result.CWD, "/work")
	}
	if result.InsideFile != "inside\n" {
		t.Fatalf("inside file = %q, want %q", result.InsideFile, "inside\n")
	}
	if result.OutsideVisible {
		t.Fatalf("outside path remained visible after chroot: %q", outsidePath)
	}
}

func TestSandboxCredentialSwitchBehavior(t *testing.T) {
	requireSandboxRoot(t)

	const targetID = 65534
	result, output, err := runSandboxBehaviorHelper(t, "credentials", map[string]string{
		sandboxBehaviorTargetUIDEnv: strconv.Itoa(targetID),
		sandboxBehaviorTargetGIDEnv: strconv.Itoa(targetID),
	})
	if err != nil {
		t.Fatalf("sandbox helper failed: %v\noutput: %s", err, output)
	}
	if result.UID != targetID || result.EUID != targetID {
		t.Fatalf("uid/euid = (%d,%d), want (%d,%d)", result.UID, result.EUID, targetID, targetID)
	}
	if result.GID != targetID || result.EGID != targetID {
		t.Fatalf("gid/egid = (%d,%d), want (%d,%d)", result.GID, result.EGID, targetID, targetID)
	}
	if !allFieldsEqual(result.UIDStatus, targetID) {
		t.Fatalf("Uid status = %v, want all %d", result.UIDStatus, targetID)
	}
	if !allFieldsEqual(result.GIDStatus, targetID) {
		t.Fatalf("Gid status = %v, want all %d", result.GIDStatus, targetID)
	}
}

func TestSandboxMountNamespaceBehavior(t *testing.T) {
	requireSandboxRoot(t)

	parentMountNS, err := os.Readlink("/proc/self/ns/mnt")
	if err != nil {
		t.Fatalf("os.Readlink(/proc/self/ns/mnt) error = %v", err)
	}

	result, output, err := runSandboxBehaviorHelper(t, "mount_namespace", nil)
	if err != nil {
		if isNamespacePermissionSkip(output) {
			t.Skipf("mount namespace is unavailable in current environment: %s", strings.TrimSpace(string(output)))
		}
		t.Fatalf("sandbox helper failed: %v\noutput: %s", err, output)
	}
	if result.MountNS == "" {
		t.Fatal("mount namespace result is empty")
	}
	if result.MountNS == parentMountNS {
		t.Fatalf("mount namespace = %q, want a different namespace from parent %q", result.MountNS, parentMountNS)
	}
}

func runSandboxBehaviorHelper(t *testing.T, mode string, extraEnv map[string]string) (sandboxBehaviorResult, []byte, error) {
	t.Helper()

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}

	env := append(os.Environ(),
		sandboxBehaviorHelperEnv+"=1",
		sandboxBehaviorModeEnv+"="+mode,
	)
	for key, value := range extraEnv {
		env = append(env, key+"="+value)
	}

	cmd := &exec.Cmd{
		Path: self,
		Args: []string{self, "-test.run=^TestSandboxBehaviorHelperProcess$"},
		Env:  env,
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return sandboxBehaviorResult{}, output, err
	}

	var result sandboxBehaviorResult
	if err := json.Unmarshal(output, &result); err != nil {
		return sandboxBehaviorResult{}, output, fmt.Errorf("decode sandbox helper result: %w", err)
	}
	return result, output, nil
}

func sandboxBehaviorConfig(mode string) (SandboxConfig, error) {
	switch mode {
	case "no_new_privs":
		return SandboxConfig{NoNewPrivs: true, UID: -1, GID: -1}, nil
	case "chroot_workdir":
		return SandboxConfig{
			UID:        -1,
			GID:        -1,
			NoNewPrivs: true,
			ChrootDir:  os.Getenv(sandboxBehaviorJailEnv),
			WorkDir:    os.Getenv(sandboxBehaviorWorkDirEnv),
		}, nil
	case "credentials":
		uid, err := strconv.Atoi(os.Getenv(sandboxBehaviorTargetUIDEnv))
		if err != nil {
			return SandboxConfig{}, fmt.Errorf("parse target uid: %w", err)
		}
		gid, err := strconv.Atoi(os.Getenv(sandboxBehaviorTargetGIDEnv))
		if err != nil {
			return SandboxConfig{}, fmt.Errorf("parse target gid: %w", err)
		}
		return SandboxConfig{
			UID:        uid,
			GID:        gid,
			NoNewPrivs: true,
		}, nil
	case "mount_namespace":
		return SandboxConfig{
			UID:        -1,
			GID:        -1,
			NoNewPrivs: true,
			UseMountNS: true,
		}, nil
	default:
		return SandboxConfig{}, fmt.Errorf("unknown sandbox helper mode %q", mode)
	}
}

func sandboxBehaviorProbe(mode string) (sandboxBehaviorResult, error) {
	switch mode {
	case "no_new_privs":
		fields, err := readProcStatusFields("NoNewPrivs")
		if err != nil {
			return sandboxBehaviorResult{}, err
		}
		return sandboxBehaviorResult{NoNewPrivs: firstField(fields["NoNewPrivs"])}, nil
	case "chroot_workdir":
		cwd, err := os.Getwd()
		if err != nil {
			return sandboxBehaviorResult{}, err
		}
		insideData, err := os.ReadFile(os.Getenv(sandboxBehaviorInsidePathEnv))
		if err != nil {
			return sandboxBehaviorResult{}, err
		}
		_, outsideErr := os.Stat(os.Getenv(sandboxBehaviorOutsidePathEnv))
		return sandboxBehaviorResult{
			CWD:            cwd,
			InsideFile:     string(insideData),
			OutsideVisible: outsideErr == nil,
			OutsideError:   errorString(outsideErr),
		}, nil
	case "credentials":
		fields, err := readProcStatusFields("Uid", "Gid")
		if err != nil {
			return sandboxBehaviorResult{}, err
		}
		return sandboxBehaviorResult{
			UID:       os.Getuid(),
			EUID:      os.Geteuid(),
			GID:       os.Getgid(),
			EGID:      os.Getegid(),
			UIDStatus: fields["Uid"],
			GIDStatus: fields["Gid"],
		}, nil
	case "mount_namespace":
		mountNS, err := os.Readlink("/proc/self/ns/mnt")
		if err != nil {
			return sandboxBehaviorResult{}, err
		}
		return sandboxBehaviorResult{MountNS: mountNS}, nil
	default:
		return sandboxBehaviorResult{}, fmt.Errorf("unknown sandbox helper mode %q", mode)
	}
}

func readProcStatusFields(keys ...string) (map[string][]string, error) {
	data, err := os.ReadFile("/proc/self/status")
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
			return nil, fmt.Errorf("status key %q not found in /proc/self/status", key)
		}
	}
	return result, nil
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

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func requireSandboxRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("requires root on linux to validate real sandbox behavior")
	}
}

func isNamespacePermissionSkip(output []byte) bool {
	if !strings.Contains(string(output), "sandbox startup failed at setup namespaces") {
		return false
	}
	return strings.Contains(string(output), syscall.EPERM.Error())
}

func assertSandboxPermissionFailure(t *testing.T, output []byte, stage string) {
	t.Helper()
	message := string(output)
	if !strings.Contains(message, "sandbox startup failed at "+stage) {
		t.Fatalf("sandbox failure output = %q, want stage %q", message, stage)
	}
	if !strings.Contains(message, syscall.EPERM.Error()) {
		t.Fatalf("sandbox failure output = %q, want %q", message, syscall.EPERM.Error())
	}
}
