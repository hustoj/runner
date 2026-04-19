//go:build linux

package runner

import (
	"testing"
)

func TestSandboxConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     SandboxConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "both uid and gid negative - skip privileges drop",
			cfg: SandboxConfig{
				UID: -1,
				GID: -1,
			},
			wantErr: false,
		},
		{
			name: "only uid set - error",
			cfg: SandboxConfig{
				UID: 1000,
				GID: -1,
			},
			wantErr: true,
			errMsg:  "must be configured together",
		},
		{
			name: "only gid set - error",
			cfg: SandboxConfig{
				UID: -1,
				GID: 1000,
			},
			wantErr: true,
			errMsg:  "must be configured together",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := dropPrivileges(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("dropPrivileges() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("dropPrivileges() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestSetupNamespaces_FlagCombination(t *testing.T) {
	tests := []struct {
		name    string
		cfg     SandboxConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "no namespaces",
			cfg:     SandboxConfig{},
			wantErr: false,
		},
		{
			name: "mount namespace only",
			cfg: SandboxConfig{
				UseMountNS: true,
			},
			wantErr: true,
		},
		{
			name: "pid namespace unsupported",
			cfg: SandboxConfig{
				UsePIDNS: true,
			},
			wantErr: true,
			errMsg:  "UsePIDNS is not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := setupNamespaces(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("setupNamespaces() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
				t.Errorf("setupNamespaces() error = %v, want error containing %q", err, tt.errMsg)
			}
		})
	}
}

func TestPrepareChildSandboxSpec_RejectsPIDNamespace(t *testing.T) {
	_, err := prepareChildSandboxSpec(SandboxConfig{UsePIDNS: true})
	if err == nil {
		t.Fatal("prepareChildSandboxSpec() should reject UsePIDNS")
	}
	if !contains(err.Error(), "UsePIDNS is not supported") {
		t.Fatalf("prepareChildSandboxSpec() error = %v, want unsupported pid namespace message", err)
	}
}

func TestSetupRootFS_PathHandling(t *testing.T) {
	tests := []struct {
		name      string
		cfg       SandboxConfig
		wantChdir bool
	}{
		{
			name:      "no chroot, no workdir",
			cfg:       SandboxConfig{},
			wantChdir: false,
		},
		{
			name: "no chroot, with workdir",
			cfg: SandboxConfig{
				WorkDir: "/tmp",
			},
			wantChdir: true,
		},
		{
			name: "with chroot, no workdir",
			cfg: SandboxConfig{
				ChrootDir: "/some/dir",
			},
			wantChdir: false, // would chdir to / after chroot, but chroot will fail
		},
		{
			name: "with chroot and workdir",
			cfg: SandboxConfig{
				ChrootDir: "/some/dir",
				WorkDir:   "/app",
			},
			wantChdir: false, // chroot will fail without root
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Similar to namespace test, we can only verify logic not actual syscalls
			// setupRootFS will fail if it tries to chroot without privileges
			err := setupRootFS(tt.cfg)
			if tt.cfg.ChrootDir != "" {
				// Expect error when trying to chroot without root
				if err == nil {
					t.Error("setupRootFS() should fail when trying to chroot without root privileges")
				}
			}
		})
	}
}

func TestSetNoNewPrivs(t *testing.T) {
	tests := []struct {
		name    string
		cfg     SandboxConfig
		wantErr bool
	}{
		{
			name: "disabled",
			cfg: SandboxConfig{
				NoNewPrivs: false,
			},
			wantErr: false,
		},
		{
			name: "enabled",
			cfg: SandboxConfig{
				NoNewPrivs: true,
			},
			wantErr: false, // PR_SET_NO_NEW_PRIVS doesn't require privileges
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := setNoNewPrivs(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("setNoNewPrivs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
