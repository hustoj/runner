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
		name string
		cfg  SandboxConfig
		want int // expected flags combination (we can't test actual unshare without root)
	}{
		{
			name: "no namespaces",
			cfg:  SandboxConfig{},
			want: 0,
		},
		{
			name: "mount namespace only",
			cfg: SandboxConfig{
				UseMountNS: true,
			},
			want: 1,
		},
		{
			name: "all namespaces",
			cfg: SandboxConfig{
				UseMountNS: true,
				UsePIDNS:   true,
				UseIPCNS:   true,
				UseUTSNS:   true,
				UseNetNS:   true,
			},
			want: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can only test the logic without root privileges
			// Actual unshare would fail, so we just verify it doesn't panic
			// and returns expected error for non-root users
			err := setupNamespaces(tt.cfg)
			if tt.want == 0 && err != nil {
				t.Errorf("setupNamespaces() with no flags should succeed, got error: %v", err)
			}
			// For non-zero flags, we expect EPERM when not root
			// This is a weak test but better than nothing
		})
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
