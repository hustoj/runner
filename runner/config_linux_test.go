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

	json := `{"OneTimeCalls":["execve"],"AllowedCalls":["read","write","brk"],"AdditionCalls":["mmap"]}`
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

	json := `{"SyscallBackend":"hybrid","NoNewPrivs":true,"OneTimeCalls":["execve"],"AllowedCalls":["read"],"AdditionCalls":["write"]}`
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
