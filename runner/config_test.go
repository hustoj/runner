package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLoadConfigAllowsWarningsBeforeLoggerInit(t *testing.T) {
	restoreGlobals := preserveConfigTestGlobals()
	defer restoreGlobals()

	t.Setenv("RUNNER_ALLOW_UNSAFE_TEST_MODE", "1")
	runWithTempCaseJSON(t, `{"UseNetNS":true,"RunUID":-1,"RunGID":-1,"AllowPrivilegedChild":true}`, func() {
		SetLogger(nil)

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		warnings := cfg.ValidationWarnings()
		if len(warnings) != 2 {
			t.Fatalf("ValidationWarnings() len = %d, want 2", len(warnings))
		}
		if warnings[0] != namespacePrivilegeWarning {
			t.Fatalf("ValidationWarnings()[0] = %q, want %q", warnings[0], namespacePrivilegeWarning)
		}
		if warnings[1] != privilegedChildWarning {
			t.Fatalf("ValidationWarnings()[1] = %q, want %q", warnings[1], privilegedChildWarning)
		}
	})
}

func TestLoadConfigInvalidConfigurationReturnsErrorWithoutLogger(t *testing.T) {
	restoreGlobals := preserveConfigTestGlobals()
	defer restoreGlobals()

	runWithTempCaseJSON(t, `{"RunUID":1000,"RunGID":-1}`, func() {
		SetLogger(nil)

		_, err := LoadConfig()
		if err == nil {
			t.Fatal("LoadConfig() should return an error for invalid configuration")
		}
		message := err.Error()
		if !strings.Contains(message, "invalid configuration") {
			t.Fatalf("error = %q, want invalid configuration message", message)
		}
		if strings.Contains(message, "nil pointer dereference") {
			t.Fatalf("error = %q, should not be nil pointer dereference", message)
		}
	})
}

func TestLoadConfigRejectsRootWithoutPrivilegeDropOrOptIn(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("runtime security checks are linux-only")
	}

	restoreGlobals := preserveConfigTestGlobals()
	defer restoreGlobals()
	effectiveUID = func() int { return 0 }

	runWithTempCaseJSON(t, `{}`, func() {
		SetLogger(nil)

		_, err := LoadConfig()
		if err == nil {
			t.Fatal("LoadConfig() error = nil, want privileged child opt-in rejection")
		}
		message := err.Error()
		if !strings.Contains(message, "unsafe configuration") {
			t.Fatalf("error = %q, want unsafe configuration message", message)
		}
		if !strings.Contains(message, privilegedChildOptInRequiredError) {
			t.Fatalf("error = %q, want %q", message, privilegedChildOptInRequiredError)
		}
	})
}

func TestLoadConfigAllowsRootWithNonRootPrivilegeDrop(t *testing.T) {
	restoreGlobals := preserveConfigTestGlobals()
	defer restoreGlobals()
	effectiveUID = func() int { return 0 }

	runWithTempCaseJSON(t, `{"RunUID":65534,"RunGID":65534}`, func() {
		SetLogger(nil)

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if cfg.RunUID != 65534 || cfg.RunGID != 65534 {
			t.Fatalf("LoadConfig() credentials = (%d,%d), want (65534,65534)", cfg.RunUID, cfg.RunGID)
		}
	})
}

func TestValidateLaunchSafetyRejectsRootTargetCredentialsWithoutOptIn(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("runtime security checks are linux-only")
	}

	restoreGlobals := preserveConfigTestGlobals()
	defer restoreGlobals()
	effectiveUID = func() int { return 0 }

	tests := []struct {
		name string
		uid  int
		gid  int
	}{
		{name: "no privilege drop", uid: -1, gid: -1},
		{name: "root uid", uid: 0, gid: 65534},
		{name: "root gid", uid: 65534, gid: 0},
		{name: "root uid and gid", uid: 0, gid: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &TaskConfig{RunUID: tt.uid, RunGID: tt.gid}
			err := cfg.ValidateLaunchSafety()
			if err == nil {
				t.Fatal("ValidateLaunchSafety() error = nil, want privileged child opt-in rejection")
			}
			if !strings.Contains(err.Error(), privilegedChildOptInRequiredError) {
				t.Fatalf("ValidateLaunchSafety() error = %q, want %q", err, privilegedChildOptInRequiredError)
			}
		})
	}
}

func TestLoadConfigRejectsNegativeResourceLimits(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantMsg string
	}{
		{"CPU", `{"CPU":-1}`, "cpu must be >= 0"},
		{"WallClock", `{"WallClock":-1}`, "wallClock must be >= 0"},
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
				SetLogger(nil)

				_, err := LoadConfig()
				if err == nil {
					t.Fatalf("LoadConfig() should return an error for negative %s", tt.name)
				}

				message := err.Error()
				if !strings.Contains(message, tt.wantMsg) {
					t.Fatalf("error = %q, want %q", message, tt.wantMsg)
				}
			})
		})
	}
}

func TestEffectiveWallClockLimitDefaultsToCPU(t *testing.T) {
	cfg := &TaskConfig{CPU: 3}

	if got := cfg.effectiveWallClockLimitSeconds(); got != 3 {
		t.Fatalf("effectiveWallClockLimitSeconds() = %d, want 3", got)
	}
}

func TestEffectiveWallClockLimitUsesExplicitWallClock(t *testing.T) {
	cfg := &TaskConfig{CPU: 3, WallClock: 7}

	if got := cfg.effectiveWallClockLimitSeconds(); got != 7 {
		t.Fatalf("effectiveWallClockLimitSeconds() = %d, want 7", got)
	}
}

func TestLoadConfigParsesWallClockLimit(t *testing.T) {
	restoreGlobals := preserveConfigTestGlobals()
	defer restoreGlobals()

	runWithTempCaseJSON(t, `{"CPU":2,"WallClock":5}`, func() {
		SetLogger(nil)

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if cfg.WallClock != 5 {
			t.Fatalf("LoadConfig() WallClock = %d, want 5", cfg.WallClock)
		}
		if got := cfg.effectiveWallClockLimitSeconds(); got != 5 {
			t.Fatalf("effectiveWallClockLimitSeconds() = %d, want 5", got)
		}
	})
}

func TestLoadConfigDefaultsWallClockLimitToCPU(t *testing.T) {
	restoreGlobals := preserveConfigTestGlobals()
	defer restoreGlobals()

	runWithTempCaseJSON(t, `{}`, func() {
		SetLogger(nil)

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if cfg.WallClock != 0 {
			t.Fatalf("LoadConfig() WallClock = %d, want 0", cfg.WallClock)
		}
		if got := cfg.effectiveWallClockLimitSeconds(); got != cfg.CPU {
			t.Fatalf("effectiveWallClockLimitSeconds() = %d, want CPU %d", got, cfg.CPU)
		}
	})
}

func TestLoadConfigWarnsOnDeprecatedMemoryReserve(t *testing.T) {
	restoreGlobals := preserveConfigTestGlobals()
	defer restoreGlobals()

	t.Setenv("RUNNER_ALLOW_UNSAFE_TEST_MODE", "1")
	runWithTempCaseJSON(t, `{"MemoryReserve":32,"AllowPrivilegedChild":true}`, func() {
		SetLogger(nil)

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		warnings := cfg.ValidationWarnings()
		if len(warnings) != 2 {
			t.Fatalf("ValidationWarnings() len = %d, want 2", len(warnings))
		}
		if warnings[0] != memoryReserveDeprecatedWarning {
			t.Fatalf("ValidationWarnings()[0] = %q, want %q", warnings[0], memoryReserveDeprecatedWarning)
		}
		if warnings[1] != privilegedChildWarning {
			t.Fatalf("ValidationWarnings()[1] = %q, want %q", warnings[1], privilegedChildWarning)
		}
	})
}

func TestValidationWarningsEmptyForSafeConfig(t *testing.T) {
	cfg := &TaskConfig{RunUID: -1, RunGID: -1}
	if warnings := cfg.ValidationWarnings(); len(warnings) != 0 {
		t.Fatalf("ValidationWarnings() = %v, want empty for safe config", warnings)
	}
}

func TestSyscallBackendDefaultsToPtrace(t *testing.T) {
	cfg := &TaskConfig{}

	if got := cfg.effectiveSyscallBackend(); got != syscallBackendPtrace {
		t.Fatalf("effectiveSyscallBackend() = %q, want %q", got, syscallBackendPtrace)
	}
}

func TestValidateRejectsUnknownSyscallBackend(t *testing.T) {
	cfg := &TaskConfig{
		CPU:            1,
		Memory:         1,
		Output:         1,
		Stack:          1,
		MaxProcs:       1,
		RunUID:         -1,
		RunGID:         -1,
		NoNewPrivs:     true,
		SyscallBackend: "unknown",
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want invalid syscall backend")
	}
	if !strings.Contains(err.Error(), "syscallBackend must be") {
		t.Fatalf("Validate() error = %q, want syscallBackend validation", err)
	}
}

func TestValidateRejectsHybridWithoutNoNewPrivs(t *testing.T) {
	cfg := &TaskConfig{
		CPU:            1,
		Memory:         1,
		Output:         1,
		Stack:          1,
		MaxProcs:       1,
		RunUID:         -1,
		RunGID:         -1,
		NoNewPrivs:     false,
		SyscallBackend: syscallBackendHybrid,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want hybrid without NoNewPrivs rejection")
	}
	if !strings.Contains(err.Error(), "requires NoNewPrivs") {
		t.Fatalf("Validate() error = %q, want NoNewPrivs validation", err)
	}
}

func TestParseCommandFallbackUsesShlex(t *testing.T) {
	tc := &TaskConfig{Command: `  /usr/bin/java   "Main Class"  `}
	if got := tc.GetCommand(); got != "/usr/bin/java" {
		t.Fatalf("GetCommand() = %q, want %q", got, "/usr/bin/java")
	}
	args := tc.GetArgs()
	if len(args) != 1 || args[0] != "Main Class" {
		t.Fatalf("GetArgs() = %v, want [Main Class]", args)
	}
}

func TestParseCommandExplicitArgsTakesPrecedence(t *testing.T) {
	tc := &TaskConfig{Command: "/usr/bin/java", Args: []string{"-Xmx128m", "Main"}}
	if got := tc.GetCommand(); got != "/usr/bin/java" {
		t.Fatalf("GetCommand() = %q, want %q", got, "/usr/bin/java")
	}
	args := tc.GetArgs()
	want := []string{"-Xmx128m", "Main"}
	if len(args) != len(want) {
		t.Fatalf("GetArgs() = %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("GetArgs()[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestParseCommandSimpleDefault(t *testing.T) {
	tc := &TaskConfig{Command: "./main"}
	if got := tc.GetCommand(); got != "./main" {
		t.Fatalf("GetCommand() = %q, want %q", got, "./main")
	}
	if args := tc.GetArgs(); len(args) != 0 {
		t.Fatalf("GetArgs() = %v, want empty", args)
	}
}

func TestResolveExecLooksUpPathAndCanonicalizesArgv0(t *testing.T) {
	wantBinary, err := exec.LookPath("sh")
	if err != nil {
		t.Fatalf("exec.LookPath(sh) error = %v", err)
	}

	tc := &TaskConfig{Command: `sh -c 'printf ok'`}
	binary, args, err := tc.ResolveExec()
	if err != nil {
		t.Fatalf("ResolveExec() error = %v", err)
	}
	if binary != wantBinary {
		t.Fatalf("ResolveExec() binary = %q, want %q", binary, wantBinary)
	}

	wantArgs := []string{wantBinary, "-c", "printf ok"}
	if len(args) != len(wantArgs) {
		t.Fatalf("ResolveExec() args = %v, want %v", args, wantArgs)
	}
	for i := range wantArgs {
		if args[i] != wantArgs[i] {
			t.Fatalf("ResolveExec() args[%d] = %q, want %q", i, args[i], wantArgs[i])
		}
	}
}

func TestResolveExecCommandSkipsPathLookupWhenDisabled(t *testing.T) {
	binary, args, err := resolveExecCommand([]string{"sh", "-c", "printf ok"}, false)
	if err != nil {
		t.Fatalf("resolveExecCommand() error = %v", err)
	}
	if binary != "sh" {
		t.Fatalf("resolveExecCommand() binary = %q, want %q", binary, "sh")
	}

	wantArgs := []string{"sh", "-c", "printf ok"}
	if len(args) != len(wantArgs) {
		t.Fatalf("resolveExecCommand() args = %v, want %v", args, wantArgs)
	}
	for i := range wantArgs {
		if args[i] != wantArgs[i] {
			t.Fatalf("resolveExecCommand() args[%d] = %q, want %q", i, args[i], wantArgs[i])
		}
	}
}

func TestResolveExecPreservesExplicitArgsPrecedence(t *testing.T) {
	tc := &TaskConfig{Command: "/bin/echo", Args: []string{"hello world"}}
	binary, args, err := tc.ResolveExec()
	if err != nil {
		t.Fatalf("ResolveExec() error = %v", err)
	}
	if binary != "/bin/echo" {
		t.Fatalf("ResolveExec() binary = %q, want %q", binary, "/bin/echo")
	}

	wantArgs := []string{"/bin/echo", "hello world"}
	if len(args) != len(wantArgs) {
		t.Fatalf("ResolveExec() args = %v, want %v", args, wantArgs)
	}
	for i := range wantArgs {
		if args[i] != wantArgs[i] {
			t.Fatalf("ResolveExec() args[%d] = %q, want %q", i, args[i], wantArgs[i])
		}
	}
}

func preserveConfigTestGlobals() func() {
	previousLog := log
	previousEffectiveUID := effectiveUID
	effectiveUID = func() int { return 1000 }

	return func() {
		SetLogger(previousLog)
		effectiveUID = previousEffectiveUID
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
