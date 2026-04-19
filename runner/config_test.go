package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigAllowsWarningsBeforeLoggerInit(t *testing.T) {
	restoreGlobals := preserveConfigTestGlobals()
	defer restoreGlobals()

	runWithTempCaseJSON(t, `{"UseNetNS":true,"RunUID":-1,"RunGID":-1}`, func() {
		log = nil
		setting = nil

		cfg := LoadConfig()
		warnings := cfg.ValidationWarnings()
		if len(warnings) != 1 {
			t.Fatalf("ValidationWarnings() len = %d, want 1", len(warnings))
		}
		if warnings[0] != namespacePrivilegeWarning {
			t.Fatalf("ValidationWarnings() = %q, want %q", warnings[0], namespacePrivilegeWarning)
		}
	})
}

func TestLoadConfigInvalidConfigurationPanicsWithoutLogger(t *testing.T) {
	restoreGlobals := preserveConfigTestGlobals()
	defer restoreGlobals()

	runWithTempCaseJSON(t, `{"RunUID":1000,"RunGID":-1}`, func() {
		log = nil
		setting = nil

		defer func() {
			recovered := recover()
			if recovered == nil {
				t.Fatal("LoadConfig() should panic for invalid configuration")
			}

			message := fmt.Sprint(recovered)
			if !strings.Contains(message, "invalid configuration") {
				t.Fatalf("panic = %q, want invalid configuration message", message)
			}
			if strings.Contains(message, "nil pointer dereference") {
				t.Fatalf("panic = %q, should not be nil pointer dereference", message)
			}
		}()

		LoadConfig()
	})
}

func TestLoadConfigRejectsNegativeResourceLimits(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantMsg string
	}{
		{"CPU", `{"CPU":-1}`, "cpu must be >= 0"},
		{"Memory", `{"Memory":-1}`, "memory must be >= 0"},
		{"MemoryReserve", `{"MemoryReserve":-1}`, "memoryReserve must be >= 0"},
		{"Output", `{"Output":-1}`, "output must be >= 0"},
		{"Stack", `{"Stack":-1}`, "stack must be >= 0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restoreGlobals := preserveConfigTestGlobals()
			defer restoreGlobals()

			runWithTempCaseJSON(t, tt.json, func() {
				log = nil
				setting = nil

				defer func() {
					recovered := recover()
					if recovered == nil {
						t.Fatalf("LoadConfig() should panic for negative %s", tt.name)
					}

					message := fmt.Sprint(recovered)
					if !strings.Contains(message, tt.wantMsg) {
						t.Fatalf("panic = %q, want %q", message, tt.wantMsg)
					}
				}()

				LoadConfig()
			})
		})
	}
}

func preserveConfigTestGlobals() func() {
	previousLog := log
	previousSetting := setting

	return func() {
		log = previousLog
		setting = previousSetting
	}
}

// runWithTempCaseJSON changes the process working directory, so callers must not use t.Parallel.
func runWithTempCaseJSON(t *testing.T, content string, fn func()) {
	t.Helper()

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	tempDir := t.TempDir()
	casePath := filepath.Join(tempDir, "case.json")
	if err := os.WriteFile(casePath, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", casePath, err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("os.Chdir(%q) error = %v", tempDir, err)
	}

	defer func() {
		if err := os.Chdir(previousWD); err != nil {
			t.Fatalf("restore working directory error = %v", err)
		}
	}()

	fn()
}
