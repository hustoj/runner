package runner

import (
	"strings"
	"testing"
)

func TestValidateRejectsInvalidSyscallName(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantMsg string
	}{
		{"OneTimeCalls", `{"OneTimeCalls":["not_a_real_syscall"]}`, `invalid syscall in oneTimeCalls: "not_a_real_syscall"`},
		{"AllowedCalls", `{"AllowedCalls":["bogus_call"]}`, `invalid syscall in allowedCalls: "bogus_call"`},
		{"AdditionCalls", `{"AdditionCalls":["fake_syscall"]}`, `invalid syscall in additionCalls: "fake_syscall"`},
		{"SyscallPolicyAllow", `{"SyscallPolicy":{"Allow":["fake_syscall"]}}`, `invalid syscall in syscallPolicy.allow: "fake_syscall"`},
		{"SyscallPolicyDeny", `{"SyscallPolicy":{"Deny":["fake_syscall"]}}`, `invalid syscall in syscallPolicy.deny: "fake_syscall"`},
		{"SyscallPolicyTrace", `{"SyscallPolicy":{"Trace":["fake_syscall"]}}`, `invalid syscall in syscallPolicy.trace: "fake_syscall"`},
		{"SyscallPolicyAudit", `{"SyscallPolicy":{"Audit":["fake_syscall"]}}`, `invalid syscall in syscallPolicy.audit: "fake_syscall"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restoreGlobals := preserveConfigTestGlobals()
			defer restoreGlobals()

			runWithTempCaseJSON(t, tt.json, func() {
				SetLogger(nil)

				_, err := LoadConfig()
				if err == nil {
					t.Fatalf("LoadConfig() should return an error for invalid syscall name")
				}

				message := err.Error()
				if !strings.Contains(message, tt.wantMsg) {
					t.Fatalf("error = %q, want %q", message, tt.wantMsg)
				}
			})
		})
	}
}

func TestValidateAcceptsValidSyscallNames(t *testing.T) {
	restoreGlobals := preserveConfigTestGlobals()
	defer restoreGlobals()

	json := `{"OneTimeCalls":["execve"],"AllowedCalls":["read","write","brk"],"AdditionCalls":["mmap"],"AllowPrivilegedChild":true}`
	runWithTempCaseJSON(t, json, func() {
		SetLogger(nil)

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() unexpected error: %v", err)
		}
	})
}

func TestValidateAcceptsHybridSyscallBackend(t *testing.T) {
	restoreGlobals := preserveConfigTestGlobals()
	defer restoreGlobals()

	json := `{"SyscallBackend":"hybrid","NoNewPrivs":true,"OneTimeCalls":["execve"],"AllowedCalls":["read"],"AdditionCalls":["write"],"AllowPrivilegedChild":true}`
	runWithTempCaseJSON(t, json, func() {
		SetLogger(nil)

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if got := cfg.effectiveSyscallBackend(); got != syscallBackendHybrid {
			t.Fatalf("effectiveSyscallBackend() = %q, want %q", got, syscallBackendHybrid)
		}
	})
}

func TestValidateRuntimeSecurityRejectsRootWithoutPrivilegeDropOrOptIn(t *testing.T) {
	tests := []struct {
		name   string
		config TaskConfig
	}{
		{name: "unset", config: TaskConfig{RunUID: -1, RunGID: -1}},
		{name: "zero uid/gid keeps root", config: TaskConfig{RunUID: 0, RunGID: 0}},
		{name: "zero uid", config: TaskConfig{RunUID: 0, RunGID: 1000}},
		{name: "zero gid", config: TaskConfig{RunUID: 1000, RunGID: 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validateRuntimeSecurity(0)
			if err == nil {
				t.Fatal("validateRuntimeSecurity() error = nil, want privileged child rejection")
			}
			if !strings.Contains(err.Error(), "AllowPrivilegedChild=true") {
				t.Fatalf("validateRuntimeSecurity() error = %q, want explicit opt-in guidance", err)
			}
		})
	}
}

func TestValidateRuntimeSecurityAcceptsNonRootOrPrivilegeDropOrOptIn(t *testing.T) {
	tests := []struct {
		name   string
		config TaskConfig
		euid   int
	}{
		{name: "non-root", config: TaskConfig{RunUID: -1, RunGID: -1}, euid: 1000},
		{name: "credential drop", config: TaskConfig{RunUID: 1000, RunGID: 1000}, euid: 0},
		{name: "explicit opt-in", config: TaskConfig{RunUID: -1, RunGID: -1, AllowPrivilegedChild: true}, euid: 0},
		{name: "zero uid/gid with explicit opt-in", config: TaskConfig{RunUID: 0, RunGID: 0, AllowPrivilegedChild: true}, euid: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.config.validateRuntimeSecurity(tt.euid); err != nil {
				t.Fatalf("validateRuntimeSecurity() error = %v", err)
			}
		})
	}
}

func TestValidateRejectsHybridDenyOfStartupProtocolCalls(t *testing.T) {
	restoreGlobals := preserveConfigTestGlobals()
	defer restoreGlobals()

	json := `{"SyscallBackend":"hybrid","NoNewPrivs":true,"OneTimeCalls":["execve"],"AllowedCalls":["read"],"SyscallPolicy":{"Deny":["write"]}}`
	runWithTempCaseJSON(t, json, func() {
		SetLogger(nil)

		_, err := LoadConfig()
		if err == nil {
			t.Fatal("LoadConfig() error = nil, want startup protocol denial rejection")
		}
		if !strings.Contains(err.Error(), `syscallPolicy.deny cannot include "write"`) {
			t.Fatalf("LoadConfig() error = %q, want denied startup syscall name", err)
		}
		if !strings.Contains(err.Error(), "hybrid startup protocol") {
			t.Fatalf("LoadConfig() error = %q, want startup protocol context", err)
		}
	})
}
