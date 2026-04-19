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
	if cfg.UsePIDNS {
		return childSandboxSpec{}, fmt.Errorf("UsePIDNS is not supported by the current launcher: PID namespaces require an extra fork after unshare(CLONE_NEWPID)")
	}

	spec := childSandboxSpec{
		uid:        cfg.UID,
		gid:        cfg.GID,
		noNewPrivs: cfg.NoNewPrivs,
	}

	if cfg.UseMountNS {
		spec.namespaceFlags |= syscall.CLONE_NEWNS
	}
	if cfg.UseIPCNS {
		spec.namespaceFlags |= syscall.CLONE_NEWIPC
	}
	if cfg.UseUTSNS {
		spec.namespaceFlags |= syscall.CLONE_NEWUTS
	}
	if cfg.UseNetNS {
		spec.namespaceFlags |= syscall.CLONE_NEWNET
	}

	if cfg.ChrootDir != "" {
		path, err := syscall.BytePtrFromString(cfg.ChrootDir)
		if err != nil {
			return childSandboxSpec{}, err
		}
		spec.chrootDir = path
	}

	if cfg.WorkDir != "" {
		path, err := syscall.BytePtrFromString(cfg.WorkDir)
		if err != nil {
			return childSandboxSpec{}, err
		}
		spec.workDir = path
	}

	return spec, nil
}

func applySandboxInChild(spec childSandboxSpec) childStartupFailure {
	if spec.namespaceFlags != 0 {
		if errno := rawUnshare(spec.namespaceFlags); errno != 0 {
			return childStartupFailure{stage: childStageSandboxNamespaces, errno: errno}
		}
	}
	if spec.uid >= 0 || spec.gid >= 0 {
		if spec.uid < 0 || spec.gid < 0 {
			return childStartupFailure{stage: childStageSandboxInvalidCredentials, errno: syscall.EINVAL}
		}
	}

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
	} else if spec.workDir != nil {
		if errno := rawChdir(spec.workDir); errno != 0 {
			return childStartupFailure{stage: childStageSandboxChdirWorkDir, errno: errno}
		}
	}

	if spec.noNewPrivs {
		_, _, errno := syscall.RawSyscall6(syscall.SYS_PRCTL, uintptr(prSetNoNewPrivs), 1, 0, 0, 0, 0)
		if errno != 0 {
			return childStartupFailure{stage: childStageSandboxNoNewPrivs, errno: errno}
		}
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

func setupNamespaces(cfg SandboxConfig) error {
	if cfg.UsePIDNS {
		return fmt.Errorf("UsePIDNS is not supported by the current launcher: PID namespaces require an extra fork after unshare(CLONE_NEWPID)")
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
	if flags == 0 {
		return nil
	}
	if errno := rawUnshare(flags); errno != 0 {
		return errno
	}
	return nil
}

// setupRootFS changes the root filesystem view for the child process.
// If ChrootDir is set, chroots into that directory (requires CAP_SYS_CHROOT).
// If WorkDir is set, changes to that directory.
// Order: chroot first (changes root), then chdir to working directory.
func setupRootFS(cfg SandboxConfig) error {
	if cfg.ChrootDir == "" {
		if cfg.WorkDir != "" {
			return syscall.Chdir(cfg.WorkDir)
		}
		return nil
	}

	if err := syscall.Chroot(cfg.ChrootDir); err != nil {
		return err
	}
	if err := syscall.Chdir("/"); err != nil {
		return err
	}
	if cfg.WorkDir != "" && cfg.WorkDir != "/" {
		return syscall.Chdir(cfg.WorkDir)
	}
	return nil
}

// setNoNewPrivs prevents the process and its children from gaining new privileges.
// This blocks execve() of setuid/setgid binaries and similar privilege escalation vectors.
// PR_SET_NO_NEW_PRIVS is one-way and inherited by children.
// Must be called BEFORE dropping privileges (as it's irreversible).
func setNoNewPrivs(cfg SandboxConfig) error {
	if !cfg.NoNewPrivs {
		return nil
	}
	_, _, errno := syscall.Syscall6(syscall.SYS_PRCTL, uintptr(prSetNoNewPrivs), 1, 0, 0, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

// dropPrivileges drops to a non-privileged user account.
// Order matters: setgroups → setgid → setuid (must be last as it cannot be reversed).
// After setuid, the process cannot regain privileges.
func dropPrivileges(cfg SandboxConfig) error {
	if cfg.UID < 0 && cfg.GID < 0 {
		// No privilege dropping requested
		return nil
	}
	if cfg.UID < 0 || cfg.GID < 0 {
		return fmt.Errorf("sandbox uid/gid must be configured together (got uid=%d, gid=%d)", cfg.UID, cfg.GID)
	}
	if cfg.UID < 0 || cfg.GID < 0 {
		return fmt.Errorf("sandbox uid/gid cannot be negative (got uid=%d, gid=%d)", cfg.UID, cfg.GID)
	}
	// Clear supplementary groups first
	if err := syscall.Setgroups([]int{}); err != nil {
		return fmt.Errorf("setgroups: %w", err)
	}
	// Set GID before UID (setuid is irreversible)
	if err := syscall.Setgid(cfg.GID); err != nil {
		return fmt.Errorf("setgid(%d): %w", cfg.GID, err)
	}
	// Set UID last - after this we lose all privileges
	if err := syscall.Setuid(cfg.UID); err != nil {
		return fmt.Errorf("setuid(%d): %w", cfg.UID, err)
	}
	return nil
}
