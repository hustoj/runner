//go:build darwin

package runner

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
