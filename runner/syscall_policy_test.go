package runner

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSyscallPolicyConfigJSONContractUsesDocumentedFieldNames(t *testing.T) {
	policy := SyscallPolicyConfig{
		Allow: []string{"read"},
		Deny:  []string{"ptrace"},
		Trace: []string{"clone3"},
		Audit: []string{"getpid"},
	}

	data, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("json.Marshal(SyscallPolicyConfig) error = %v", err)
	}
	want := `{"Allow":["read"],"Deny":["ptrace"],"Trace":["clone3"],"Audit":["getpid"]}`
	if string(data) != want {
		t.Fatalf("json.Marshal(SyscallPolicyConfig) = %s, want %s", data, want)
	}

	var decoded SyscallPolicyConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(SyscallPolicyConfig) error = %v", err)
	}
	assertStringSliceEqual(t, decoded.Allow, policy.Allow)
	assertStringSliceEqual(t, decoded.Deny, policy.Deny)
	assertStringSliceEqual(t, decoded.Trace, policy.Trace)
	assertStringSliceEqual(t, decoded.Audit, policy.Audit)
}

func TestEffectiveSyscallPolicyMergesLegacyAndStructuredPolicy(t *testing.T) {
	cfg := &TaskConfig{
		OneTimeCalls:  []string{"execve", "execve"},
		AllowedCalls:  []string{"read", "write"},
		AdditionCalls: []string{"mmap", "read"},
		SyscallPolicy: SyscallPolicyConfig{
			Allow: []string{"fstat", "write"},
			Deny:  []string{"ptrace"},
			Trace: []string{"clone3"},
			Audit: []string{"getpid"},
		},
	}

	policy := cfg.effectiveSyscallPolicy()

	assertStringSliceEqual(t, policy.OneTime, []string{"execve"})
	assertStringSliceEqual(t, policy.Allow, []string{"read", "write", "mmap", "fstat"})
	assertStringSliceEqual(t, policy.Deny, []string{"ptrace"})
	assertStringSliceEqual(t, policy.Trace, []string{"clone3"})
	assertStringSliceEqual(t, policy.Audit, []string{"getpid"})
	assertStringSliceEqual(t, policy.ptraceAllowedCalls(), []string{"read", "write", "mmap", "fstat", "clone3", "getpid"})
	assertStringSliceEqual(t, policy.seccompAllowedCalls(), []string{"read", "write", "mmap", "fstat"})
	assertStringSliceEqual(t, policy.seccompTracedCalls(), []string{"execve", "clone3", "getpid"})
}

func TestEffectiveSyscallPolicyDenyOverridesLegacyAndStructuredAllow(t *testing.T) {
	cfg := &TaskConfig{
		CPU:           1,
		Memory:        1,
		Output:        1,
		Stack:         1,
		MaxProcs:      1,
		RunUID:        -1,
		RunGID:        -1,
		NoNewPrivs:    true,
		OneTimeCalls:  []string{"execve"},
		AllowedCalls:  []string{"read", "write"},
		AdditionCalls: []string{"mmap"},
		SyscallPolicy: SyscallPolicyConfig{
			Allow: []string{"fstat"},
			Deny:  []string{"read", "fstat"},
		},
	}

	policy := cfg.effectiveSyscallPolicy()

	assertStringSliceEqual(t, policy.Allow, []string{"write", "mmap"})
	assertStringSliceEqual(t, policy.Deny, []string{"read", "fstat"})
	assertStringSliceEqual(t, policy.ptraceAllowedCalls(), []string{"write", "mmap"})
	assertStringSliceEqual(t, policy.seccompAllowedCalls(), []string{"write", "mmap"})
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want Deny to override allow rules", err)
	}
}

func TestHybridPolicyTracesArgumentFilteredAllowedCalls(t *testing.T) {
	cfg := &TaskConfig{
		OneTimeCalls: []string{"execve"},
		AllowedCalls: []string{"read", "prlimit64"},
	}

	policy := cfg.effectiveSyscallPolicy()

	assertStringSliceEqual(t, policy.ptraceAllowedCalls(), []string{"read", "prlimit64"})
	assertStringSliceEqual(t, policy.seccompAllowedCalls(), []string{"read"})
	assertStringSliceEqual(t, policy.seccompTracedCalls(), []string{"execve", "prlimit64"})
}

func TestCompileSyscallPolicyProducesBackendInputs(t *testing.T) {
	cfg := &TaskConfig{
		CPU:           1,
		Memory:        1,
		Output:        1,
		Stack:         1,
		MaxProcs:      1,
		RunUID:        -1,
		RunGID:        -1,
		NoNewPrivs:    true,
		OneTimeCalls:  []string{"execve"},
		AllowedCalls:  []string{"read", "write"},
		AdditionCalls: []string{"mmap"},
		SyscallPolicy: SyscallPolicyConfig{
			Allow: []string{"fstat"},
			Deny:  []string{"read", "fstat"},
			Trace: []string{"clone3"},
			Audit: []string{"getpid"},
		},
	}

	policy, err := cfg.compileSyscallPolicy()
	if err != nil {
		t.Fatalf("compileSyscallPolicy() error = %v", err)
	}

	assertStringSliceEqual(t, policy.Ptrace.OneTimeCalls, []string{"execve"})
	assertStringSliceEqual(t, policy.Ptrace.AllowedCalls, []string{"write", "mmap", "clone3", "getpid"})
	assertStringSliceEqual(t, policy.Ptrace.AuditCalls, []string{"getpid"})
	assertStringSliceEqual(t, policy.SeccompAllowedCalls, []string{"write", "mmap", "close", "exit", "exit_group"})
	assertStringSliceEqual(t, policy.SeccompTracedCalls, []string{"execve", "clone3", "getpid"})
}

func TestValidateRejectsSyscallPolicyOwnershipOverlap(t *testing.T) {
	tests := []struct {
		name    string
		config  TaskConfig
		wantMsg string
	}{
		{
			name: "one-time overlaps structured allow",
			config: TaskConfig{
				CPU:            1,
				Memory:         1,
				Output:         1,
				Stack:          1,
				MaxProcs:       1,
				RunUID:         -1,
				RunGID:         -1,
				NoNewPrivs:     true,
				OneTimeCalls:   []string{"execve"},
				AllowedCalls:   []string{"read"},
				SyscallBackend: syscallBackendPtrace,
				SyscallPolicy:  SyscallPolicyConfig{Allow: []string{"execve"}},
			},
			wantMsg: `syscall "execve" is assigned to both oneTimeCalls and runtime allowlist`,
		},
		{
			name: "legacy one-time overlaps allowed",
			config: TaskConfig{
				CPU:            1,
				Memory:         1,
				Output:         1,
				Stack:          1,
				MaxProcs:       1,
				RunUID:         -1,
				RunGID:         -1,
				NoNewPrivs:     true,
				OneTimeCalls:   []string{"execve"},
				AllowedCalls:   []string{"read", "execve"},
				SyscallBackend: syscallBackendPtrace,
			},
			wantMsg: `syscall "execve" is assigned to both oneTimeCalls and runtime allowlist`,
		},
		{
			name: "legacy one-time overlaps addition",
			config: TaskConfig{
				CPU:            1,
				Memory:         1,
				Output:         1,
				Stack:          1,
				MaxProcs:       1,
				RunUID:         -1,
				RunGID:         -1,
				NoNewPrivs:     true,
				OneTimeCalls:   []string{"execve"},
				AllowedCalls:   []string{"read"},
				AdditionCalls:  []string{"execve"},
				SyscallBackend: syscallBackendPtrace,
			},
			wantMsg: `syscall "execve" is assigned to both oneTimeCalls and runtime allowlist`,
		},
		{
			name: "deny overlaps audit",
			config: TaskConfig{
				CPU:            1,
				Memory:         1,
				Output:         1,
				Stack:          1,
				MaxProcs:       1,
				RunUID:         -1,
				RunGID:         -1,
				NoNewPrivs:     true,
				OneTimeCalls:   []string{"execve"},
				AllowedCalls:   []string{"read"},
				SyscallBackend: syscallBackendPtrace,
				SyscallPolicy: SyscallPolicyConfig{
					Audit: []string{"getpid"},
					Deny:  []string{"getpid"},
				},
			},
			wantMsg: `syscall "getpid" cannot be both denied and syscallPolicy.audit`,
		},
		{
			name: "trace overlaps allow",
			config: TaskConfig{
				CPU:            1,
				Memory:         1,
				Output:         1,
				Stack:          1,
				MaxProcs:       1,
				RunUID:         -1,
				RunGID:         -1,
				NoNewPrivs:     true,
				OneTimeCalls:   []string{"execve"},
				AllowedCalls:   []string{"read"},
				SyscallBackend: syscallBackendPtrace,
				SyscallPolicy:  SyscallPolicyConfig{Trace: []string{"read"}},
			},
			wantMsg: `syscall "read" is assigned to both runtime allowlist and syscallPolicy.trace`,
		},
		{
			name: "audit overlaps trace",
			config: TaskConfig{
				CPU:            1,
				Memory:         1,
				Output:         1,
				Stack:          1,
				MaxProcs:       1,
				RunUID:         -1,
				RunGID:         -1,
				NoNewPrivs:     true,
				OneTimeCalls:   []string{"execve"},
				AllowedCalls:   []string{"read"},
				SyscallBackend: syscallBackendPtrace,
				SyscallPolicy: SyscallPolicyConfig{
					Trace: []string{"clone3"},
					Audit: []string{"clone3"},
				},
			},
			wantMsg: `syscall "clone3" is assigned to both syscallPolicy.trace and syscallPolicy.audit`,
		},
		{
			name: "one-time overlaps trace",
			config: TaskConfig{
				CPU:            1,
				Memory:         1,
				Output:         1,
				Stack:          1,
				MaxProcs:       1,
				RunUID:         -1,
				RunGID:         -1,
				NoNewPrivs:     true,
				OneTimeCalls:   []string{"execve"},
				AllowedCalls:   []string{"read"},
				SyscallBackend: syscallBackendPtrace,
				SyscallPolicy:  SyscallPolicyConfig{Trace: []string{"execve"}},
			},
			wantMsg: `syscall "execve" is assigned to both oneTimeCalls and syscallPolicy.trace`,
		},
		{
			name: "one-time overlaps deny",
			config: TaskConfig{
				CPU:            1,
				Memory:         1,
				Output:         1,
				Stack:          1,
				MaxProcs:       1,
				RunUID:         -1,
				RunGID:         -1,
				NoNewPrivs:     true,
				OneTimeCalls:   []string{"execve"},
				AllowedCalls:   []string{"read"},
				SyscallBackend: syscallBackendPtrace,
				SyscallPolicy:  SyscallPolicyConfig{Deny: []string{"execve"}},
			},
			wantMsg: `syscall "execve" cannot be both denied and oneTimeCalls`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want ownership overlap rejection")
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("Validate() error = %q, want %q", err, tt.wantMsg)
			}
		})
	}
}

func TestValidateHybridSyscallPolicyRejectsReservedStartupProtocolCalls(t *testing.T) {
	tests := []struct {
		name    string
		policy  EffectiveSyscallPolicy
		wantMsg string
	}{
		{
			name:    "deny",
			policy:  EffectiveSyscallPolicy{Deny: []string{"write"}},
			wantMsg: `syscallPolicy.deny cannot include "write"`,
		},
		{
			name:    "trace",
			policy:  EffectiveSyscallPolicy{Trace: []string{"close"}},
			wantMsg: `syscallPolicy.trace cannot include "close"`,
		},
		{
			name:    "audit",
			policy:  EffectiveSyscallPolicy{Audit: []string{"exit"}},
			wantMsg: `syscallPolicy.audit cannot include "exit"`,
		},
		{
			name:    "one-time",
			policy:  EffectiveSyscallPolicy{OneTime: []string{"exit_group"}},
			wantMsg: `oneTimeCalls cannot include "exit_group"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHybridSyscallPolicy(tt.policy)
			if err == nil {
				t.Fatal("validateHybridSyscallPolicy() error = nil, want startup protocol rejection")
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("validateHybridSyscallPolicy() error = %q, want %q", err, tt.wantMsg)
			}
			if !strings.Contains(err.Error(), "hybrid startup protocol") {
				t.Fatalf("validateHybridSyscallPolicy() error = %q, want startup protocol context", err)
			}
		})
	}
}

func assertStringSliceEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("slice = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slice[%d] = %q, want %q; full=%v", i, got[i], want[i], got)
		}
	}
}
