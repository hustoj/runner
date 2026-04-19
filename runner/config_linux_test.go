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
