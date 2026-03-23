//go:build linux

package runner

import (
	"fmt"
	"os"
	"syscall"
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

// applySandbox applies security isolation in a strict order.
// Must be called in the child process BEFORE ptraceTraceme() and exec().
//
// Step order is critical:
// 1. setupNamespaces - must happen before chroot (needs privileges)
// 2. setupRootFS - must happen before dropping privileges (chroot needs CAP_SYS_CHROOT)
// 3. setNoNewPrivs - must happen before dropping privileges (prevents re-gaining privs via setuid binaries)
// 4. dropPrivileges - must be last (after this we cannot do any privileged operations)
//
// This function runs in the bootstrap child process before ptraceTraceme() and
// exec(). Keep it limited to straightforward setup logic so the exec boundary
// stays easy to reason about.
func applySandbox(cfg SandboxConfig) error {
	if err := setupNamespaces(cfg); err != nil {
		return fmt.Errorf("setup namespaces: %w", err)
	}
	if err := setupRootFS(cfg); err != nil {
		return fmt.Errorf("setup rootfs: %w", err)
	}
	if err := setNoNewPrivs(cfg); err != nil {
		return fmt.Errorf("set no_new_privs: %w", err)
	}
	if err := dropPrivileges(cfg); err != nil {
		return fmt.Errorf("drop privileges: %w", err)
	}
	return nil
}

func setupNamespaces(cfg SandboxConfig) error {
	flags := 0
	if cfg.UseMountNS {
		flags |= syscall.CLONE_NEWNS
	}
	if cfg.UsePIDNS {
		flags |= syscall.CLONE_NEWPID
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
	return syscall.Unshare(flags)
}

// setupRootFS changes the root filesystem view for the child process.
// If ChrootDir is set, chroots into that directory (requires CAP_SYS_CHROOT).
// If WorkDir is set, changes to that directory.
// Order: chroot first (changes root), then chdir to working directory.
func setupRootFS(cfg SandboxConfig) error {
	if cfg.ChrootDir == "" {
		if cfg.WorkDir != "" {
			return os.Chdir(cfg.WorkDir)
		}
		return nil
	}

	if err := syscall.Chroot(cfg.ChrootDir); err != nil {
		return err
	}
	if err := os.Chdir("/"); err != nil {
		return err
	}
	if cfg.WorkDir != "" && cfg.WorkDir != "/" {
		return os.Chdir(cfg.WorkDir)
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
		return nil
	}
	if cfg.UID < 0 || cfg.GID < 0 {
		return fmt.Errorf("sandbox uid/gid must be configured together (got uid=%d, gid=%d)", cfg.UID, cfg.GID)
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
