//go:build darwin

package runner

import "fmt"

// SandboxConfig defines the security isolation parameters for child processes.
// On Darwin, most sandbox features are not available or require different APIs.
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

var (
	_ = (*RunningTask).sandboxConfig
	_ = applySandbox
)

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

// applySandbox is a no-op stub on Darwin.
// Real sandboxing requires the macOS sandbox API which is different from Linux.
func applySandbox(cfg SandboxConfig) error {
	// Darwin does not support the same sandboxing primitives as Linux
	// Real implementation would require sandbox-exec or App Sandbox APIs
	if cfg.UID >= 0 || cfg.GID >= 0 {
		return fmt.Errorf("privilege dropping not implemented on darwin")
	}
	if cfg.ChrootDir != "" {
		return fmt.Errorf("chroot not supported on darwin")
	}
	if cfg.UseMountNS || cfg.UsePIDNS || cfg.UseIPCNS || cfg.UseUTSNS || cfg.UseNetNS {
		return fmt.Errorf("namespace isolation not available on darwin")
	}
	// NoNewPrivs and WorkDir could be partially supported but are skipped for simplicity
	return nil
}
