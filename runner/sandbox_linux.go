//go:build linux

package runner

import (
	"fmt"
	"syscall"
	"unsafe"
)

const prSetNoNewPrivs = 38

// SandboxConfig defines the security isolation parameters for child processes.
// It controls privilege dropping, filesystem isolation, and namespace creation.
type SandboxConfig struct {
	UID        int
	GID        int
	ChrootDir  string
	WorkDir    string
	NoNewPrivs bool
	UseMountNS bool
	UsePIDNS   bool
	UseIPCNS   bool
	UseUTSNS   bool
	UseNetNS   bool
}

func (task *RunningTask) sandboxConfig() SandboxConfig {
	return SandboxConfig{
		UID:        task.setting.RunUID,
		GID:        task.setting.RunGID,
		ChrootDir:  task.setting.ChrootDir,
		WorkDir:    task.setting.WorkDir,
		NoNewPrivs: task.setting.NoNewPrivs,
		UseMountNS: task.setting.UseMountNS,
		UsePIDNS:   task.setting.UsePIDNS,
		UseIPCNS:   task.setting.UseIPCNS,
		UseUTSNS:   task.setting.UseUTSNS,
		UseNetNS:   task.setting.UseNetNS,
	}
}

type childSandboxSpec struct {
	uid            int
	gid            int
	chrootDir      *byte
	workDir        *byte
	noNewPrivs     bool
	namespaceFlags int
}

func prepareChildSandboxSpec(cfg SandboxConfig) (childSandboxSpec, error) {
	if err := validateSandboxCredentialConfig(cfg.UID, cfg.GID); err != nil {
		return childSandboxSpec{}, err
	}

	namespaceFlags, err := namespaceFlagsForConfig(cfg)
	if err != nil {
		return childSandboxSpec{}, err
	}

	spec := childSandboxSpec{
		uid:            cfg.UID,
		gid:            cfg.GID,
		noNewPrivs:     cfg.NoNewPrivs,
		namespaceFlags: namespaceFlags,
	}

	spec.chrootDir, err = bytePtrOrNil(cfg.ChrootDir)
	if err != nil {
		return childSandboxSpec{}, err
	}

	spec.workDir, err = bytePtrOrNil(cfg.WorkDir)
	if err != nil {
		return childSandboxSpec{}, err
	}

	return spec, nil
}

func applySandboxInChild(spec childSandboxSpec) childStartupFailure {
	if failure := applySandboxNamespaces(spec); failure.failed() {
		return failure
	}
	if failure := applySandboxRootFS(spec); failure.failed() {
		return failure
	}
	if failure := applySandboxNoNewPrivs(spec); failure.failed() {
		return failure
	}
	if failure := applySandboxCredentials(spec); failure.failed() {
		return failure
	}

	return childStartupFailure{}
}

func validateSandboxCredentialConfig(uid, gid int) error {
	if uid < 0 && gid < 0 {
		return nil
	}
	if uid < 0 || gid < 0 {
		return fmt.Errorf("sandbox uid/gid must be configured together (got uid=%d, gid=%d)", uid, gid)
	}
	return nil
}

func namespaceFlagsForConfig(cfg SandboxConfig) (int, error) {
	if cfg.UsePIDNS {
		return 0, fmt.Errorf("UsePIDNS is not supported by the current launcher: PID namespaces require an extra fork after unshare(CLONE_NEWPID)")
	}

	flags := 0
	if cfg.UseMountNS {
		flags |= syscall.CLONE_NEWNS
	}
	if cfg.UseIPCNS {
		flags |= syscall.CLONE_NEWIPC
	}
	if cfg.UseUTSNS {
		flags |= syscall.CLONE_NEWUTS
	}
	if cfg.UseNetNS {
		flags |= syscall.CLONE_NEWNET
	}
	return flags, nil
}

func bytePtrOrNil(value string) (*byte, error) {
	if value == "" {
		return nil, nil
	}
	return syscall.BytePtrFromString(value)
}

func applySandboxNamespaces(spec childSandboxSpec) childStartupFailure {
	if spec.namespaceFlags == 0 {
		return childStartupFailure{}
	}
	if errno := rawUnshare(spec.namespaceFlags); errno != 0 {
		return childStartupFailure{stage: childStageSandboxNamespaces, errno: errno}
	}
	return childStartupFailure{}
}

func applySandboxRootFS(spec childSandboxSpec) childStartupFailure {
	if spec.chrootDir != nil {
		if errno := rawChroot(spec.chrootDir); errno != 0 {
			return childStartupFailure{stage: childStageSandboxChroot, errno: errno}
		}
		if errno := rawChdir(rootDirPtr); errno != 0 {
			return childStartupFailure{stage: childStageSandboxChdirRoot, errno: errno}
		}
		if spec.workDir != nil && spec.workDir != rootDirPtr {
			if errno := rawChdir(spec.workDir); errno != 0 {
				return childStartupFailure{stage: childStageSandboxChdirWorkDir, errno: errno}
			}
		}
		return childStartupFailure{}
	}

	if spec.workDir != nil {
		if errno := rawChdir(spec.workDir); errno != 0 {
			return childStartupFailure{stage: childStageSandboxChdirWorkDir, errno: errno}
		}
	}
	return childStartupFailure{}
}

func applySandboxNoNewPrivs(spec childSandboxSpec) childStartupFailure {
	if !spec.noNewPrivs {
		return childStartupFailure{}
	}
	_, _, errno := syscall.RawSyscall6(syscall.SYS_PRCTL, uintptr(prSetNoNewPrivs), 1, 0, 0, 0, 0)
	if errno != 0 {
		return childStartupFailure{stage: childStageSandboxNoNewPrivs, errno: errno}
	}
	return childStartupFailure{}
}

func applySandboxCredentials(spec childSandboxSpec) childStartupFailure {
	if err := validateSandboxCredentialConfig(spec.uid, spec.gid); err != nil {
		return childStartupFailure{stage: childStageSandboxInvalidCredentials, errno: syscall.EINVAL}
	}
	if spec.uid < 0 && spec.gid < 0 {
		return childStartupFailure{}
	}
	if errno := rawClearGroups(); errno != 0 {
		return childStartupFailure{stage: childStageSandboxSetgroups, errno: errno}
	}
	// Do not use syscall.Setgid/Setuid here: on Linux they go through
	// AllThreadsSyscall, which is unsafe after raw fork in a Go program.
	if errno := rawSetgid(spec.gid); errno != 0 {
		return childStartupFailure{stage: childStageSandboxSetgid, errno: errno}
	}
	if errno := rawSetuid(spec.uid); errno != 0 {
		return childStartupFailure{stage: childStageSandboxSetuid, errno: errno}
	}
	return childStartupFailure{}
}

var rootDir = [...]byte{'/', 0}

var rootDirPtr = &rootDir[0]

func rawChroot(path *byte) syscall.Errno {
	_, _, errno := syscall.RawSyscall(syscall.SYS_CHROOT, uintptr(unsafe.Pointer(path)), 0, 0)
	return errno
}

func rawChdir(path *byte) syscall.Errno {
	_, _, errno := syscall.RawSyscall(syscall.SYS_CHDIR, uintptr(unsafe.Pointer(path)), 0, 0)
	return errno
}

func rawClearGroups() syscall.Errno {
	_, _, errno := syscall.RawSyscall(syscall.SYS_SETGROUPS, 0, 0, 0)
	return errno
}

func rawUnshare(flags int) syscall.Errno {
	_, _, errno := syscall.RawSyscall(syscall.SYS_UNSHARE, uintptr(flags), 0, 0)
	return errno
}

func rawSetgid(gid int) syscall.Errno {
	_, _, errno := syscall.RawSyscall(syscall.SYS_SETGID, uintptr(gid), 0, 0)
	return errno
}

func rawSetuid(uid int) syscall.Errno {
	_, _, errno := syscall.RawSyscall(syscall.SYS_SETUID, uintptr(uid), 0, 0)
	return errno
}
