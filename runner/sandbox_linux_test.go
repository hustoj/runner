//go:build linux

package runner

import (
	"strings"
	"syscall"
	"testing"
)

func TestValidateSandboxCredentialConfig(t *testing.T) {
	tests := []struct {
		name    string
		uid     int
		gid     int
		wantErr bool
		errMsg  string
	}{
		{
			name:    "both negative disables privilege drop",
			uid:     -1,
			gid:     -1,
			wantErr: false,
		},
		{
			name:    "only uid set",
			uid:     1000,
			gid:     -1,
			wantErr: true,
			errMsg:  "must be configured together",
		},
		{
			name:    "only gid set",
			uid:     -1,
			gid:     1000,
			wantErr: true,
			errMsg:  "must be configured together",
		},
		{
			name:    "both set",
			uid:     1000,
			gid:     1000,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSandboxCredentialConfig(tt.uid, tt.gid)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateSandboxCredentialConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Fatalf("validateSandboxCredentialConfig() error = %v, want %q", err, tt.errMsg)
			}
		})
	}
}

func TestNamespaceFlagsForConfig(t *testing.T) {
	tests := []struct {
		name      string
		cfg       SandboxConfig
		wantFlags int
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "no namespaces",
			cfg:       SandboxConfig{},
			wantFlags: 0,
			wantErr:   false,
		},
		{
			name: "combined flags",
			cfg: SandboxConfig{
				UseMountNS: true,
				UseIPCNS:   true,
				UseUTSNS:   true,
				UseNetNS:   true,
			},
			wantFlags: syscall.CLONE_NEWNS | syscall.CLONE_NEWIPC | syscall.CLONE_NEWUTS | syscall.CLONE_NEWNET,
			wantErr:   false,
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
			flags, err := namespaceFlagsForConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("namespaceFlagsForConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Fatalf("namespaceFlagsForConfig() error = %v, want %q", err, tt.errMsg)
				}
				return
			}
			if flags != tt.wantFlags {
				t.Fatalf("namespaceFlagsForConfig() flags = %#x, want %#x", flags, tt.wantFlags)
			}
		})
	}
}

func TestBytePtrOrNil(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantNil bool
		wantErr bool
	}{
		{
			name:    "empty string",
			value:   "",
			wantNil: true,
			wantErr: false,
		},
		{
			name:    "normal path",
			value:   "/tmp",
			wantNil: false,
			wantErr: false,
		},
		{
			name:    "nul byte rejected",
			value:   "/tmp\x00broken",
			wantNil: true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ptr, err := bytePtrOrNil(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("bytePtrOrNil() error = %v, wantErr %v", err, tt.wantErr)
			}
			if (ptr == nil) != tt.wantNil {
				t.Fatalf("bytePtrOrNil() nil = %v, wantNil %v", ptr == nil, tt.wantNil)
			}
		})
	}
}

func TestPrepareChildSandboxSpec(t *testing.T) {
	cfg := SandboxConfig{
		UID:        1000,
		GID:        1001,
		ChrootDir:  "/srv/root",
		WorkDir:    "/work",
		NoNewPrivs: true,
		UseMountNS: true,
		UseIPCNS:   true,
	}

	spec, err := prepareChildSandboxSpec(cfg)
	if err != nil {
		t.Fatalf("prepareChildSandboxSpec() error = %v", err)
	}

	if spec.uid != cfg.UID || spec.gid != cfg.GID {
		t.Fatalf("prepareChildSandboxSpec() credentials = (%d,%d), want (%d,%d)", spec.uid, spec.gid, cfg.UID, cfg.GID)
	}
	if !spec.noNewPrivs {
		t.Fatal("prepareChildSandboxSpec() should preserve no_new_privs")
	}
	wantFlags := syscall.CLONE_NEWNS | syscall.CLONE_NEWIPC
	if spec.namespaceFlags != wantFlags {
		t.Fatalf("prepareChildSandboxSpec() flags = %#x, want %#x", spec.namespaceFlags, wantFlags)
	}
	if spec.chrootDir == nil {
		t.Fatal("prepareChildSandboxSpec() should populate chrootDir")
	}
	if spec.workDir == nil {
		t.Fatal("prepareChildSandboxSpec() should populate workDir")
	}
}

func TestPrepareChildSandboxSpecRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		cfg    SandboxConfig
		errMsg string
	}{
		{
			name: "pid namespace unsupported",
			cfg: SandboxConfig{
				UsePIDNS: true,
			},
			errMsg: "UsePIDNS is not supported",
		},
		{
			name: "credential mismatch",
			cfg: SandboxConfig{
				UID: 1000,
				GID: -1,
			},
			errMsg: "must be configured together",
		},
		{
			name: "invalid chroot path",
			cfg: SandboxConfig{
				ChrootDir: "/root\x00bad",
			},
			errMsg: "invalid argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := prepareChildSandboxSpec(tt.cfg)
			if err == nil {
				t.Fatal("prepareChildSandboxSpec() should fail")
			}
			if !strings.Contains(err.Error(), tt.errMsg) {
				t.Fatalf("prepareChildSandboxSpec() error = %v, want %q", err, tt.errMsg)
			}
		})
	}
}

func TestApplySandboxCredentialsRejectsInvalidSpec(t *testing.T) {
	failure := applySandboxCredentials(childSandboxSpec{uid: 1000, gid: -1})
	if !failure.failed() {
		t.Fatal("applySandboxCredentials() should reject invalid credential spec")
	}
	if failure.stage != childStageSandboxInvalidCredentials {
		t.Fatalf("applySandboxCredentials() stage = %v, want %v", failure.stage, childStageSandboxInvalidCredentials)
	}
	if failure.errno != syscall.EINVAL {
		t.Fatalf("applySandboxCredentials() errno = %v, want %v", failure.errno, syscall.EINVAL)
	}
}
