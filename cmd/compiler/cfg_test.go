package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/koding/multiconfig"
)

func TestResolveExecUsesExplicitArgsArray(t *testing.T) {
	wantBinary, err := exec.LookPath("sh")
	if err != nil {
		t.Fatalf("exec.LookPath(sh) error = %v", err)
	}

	cfg := &CompileConfig{
		Command: "sh",
		Args:    newCompileArgs("-c", "printf ok"),
	}

	binary, args, err := cfg.ResolveExec()
	if err != nil {
		t.Fatalf("ResolveExec() error = %v", err)
	}
	if binary != wantBinary {
		t.Fatalf("ResolveExec() binary = %q, want %q", binary, wantBinary)
	}

	wantArgs := []string{wantBinary, "-c", "printf ok"}
	assertStringSliceEqual(t, args, wantArgs, "ResolveExec() args")
}

func TestResolveExecSplitsCommandWhenExplicitArgsOmitted(t *testing.T) {
	wantBinary, err := exec.LookPath("sh")
	if err != nil {
		t.Fatalf("exec.LookPath(sh) error = %v", err)
	}

	cfg := &CompileConfig{Command: `sh -c 'printf ok'`}

	binary, args, err := cfg.ResolveExec()
	if err != nil {
		t.Fatalf("ResolveExec() error = %v", err)
	}
	if binary != wantBinary {
		t.Fatalf("ResolveExec() binary = %q, want %q", binary, wantBinary)
	}

	wantArgs := []string{wantBinary, "-c", "printf ok"}
	assertStringSliceEqual(t, args, wantArgs, "ResolveExec() args")
}

func TestLoadConfigAcceptsLegacyStringArgs(t *testing.T) {
	runWithTempCompileJSON(t, `{"command":"sh","args":"-c 'printf ok'"}`, func(compilePath string) {
		cfg := loadConfigWithLoader(t, compilePath, nil)

		binary, args, err := cfg.ResolveExec()
		if err != nil {
			t.Fatalf("ResolveExec() error = %v", err)
		}

		wantBinary, err := exec.LookPath("sh")
		if err != nil {
			t.Fatalf("exec.LookPath(sh) error = %v", err)
		}
		if binary != wantBinary {
			t.Fatalf("ResolveExec() binary = %q, want %q", binary, wantBinary)
		}

		wantArgs := []string{wantBinary, "-c", "printf ok"}
		assertStringSliceEqual(t, args, wantArgs, "ResolveExec() args")
	})
}

func TestCompileArgsSetPreservesCommaInsideSingleArg(t *testing.T) {
	var args CompileArgs
	if err := args.Set("-Wl,-z,relro"); err != nil {
		t.Fatalf("CompileArgs.Set() error = %v", err)
	}

	want := []string{"-Wl,-z,relro"}
	assertStringSliceEqual(t, args.Values(), want, "CompileArgs.Values()")
}

func TestLoadConfigAcceptsArrayArgs(t *testing.T) {
	runWithTempCompileJSON(t, `{"command":"sh","args":["-c","printf ok"]}`, func(compilePath string) {
		cfg := loadConfigWithLoader(t, compilePath, nil)

		binary, args, err := cfg.ResolveExec()
		if err != nil {
			t.Fatalf("ResolveExec() error = %v", err)
		}

		wantBinary, err := exec.LookPath("sh")
		if err != nil {
			t.Fatalf("exec.LookPath(sh) error = %v", err)
		}
		if binary != wantBinary {
			t.Fatalf("ResolveExec() binary = %q, want %q", binary, wantBinary)
		}

		wantArgs := []string{wantBinary, "-c", "printf ok"}
		assertStringSliceEqual(t, args, wantArgs, "ResolveExec() args")
	})
}

func TestLoadConfigKeepsDefaultArgsWhenArgsOmitted(t *testing.T) {
	runWithTempCompileJSON(t, `{"command":"gcc"}`, func(compilePath string) {
		cfg := loadConfigWithLoader(t, compilePath, nil)
		want := []string{"main.c", "-o", "main", "-O2", "-fmax-errors=10", "-Wall", "--static", "-lm", "--std=c99"}
		assertStringSliceEqual(t, cfg.GetArgs(), want, "GetArgs()")
	})
}

func TestLoadConfigFallsBackToCommandSplitWhenArgsIsNull(t *testing.T) {
	runWithTempCompileJSON(t, `{"command":"sh -c 'printf ok'","args":null}`, func(compilePath string) {
		cfg := loadConfigWithLoader(t, compilePath, nil)

		wantBinary, err := exec.LookPath("sh")
		if err != nil {
			t.Fatalf("exec.LookPath(sh) error = %v", err)
		}
		binary, args, err := cfg.ResolveExec()
		if err != nil {
			t.Fatalf("ResolveExec() error = %v", err)
		}
		if binary != wantBinary {
			t.Fatalf("ResolveExec() binary = %q, want %q", binary, wantBinary)
		}

		wantArgs := []string{wantBinary, "-c", "printf ok"}
		assertStringSliceEqual(t, args, wantArgs, "ResolveExec() args")
	})
}

func TestLoadConfigEnvOverrideAcceptsLegacyShellStyleArgs(t *testing.T) {
	runWithTempCompileJSON(t, `{"command":"sh","args":["-c","printf file"]}`, func(compilePath string) {
		t.Setenv("COMPILECONFIG_ARGS", `-c 'printf env override'`)
		cfg := loadConfigWithLoader(t, compilePath, nil)

		wantBinary, err := exec.LookPath("sh")
		if err != nil {
			t.Fatalf("exec.LookPath(sh) error = %v", err)
		}
		binary, args, err := cfg.ResolveExec()
		if err != nil {
			t.Fatalf("ResolveExec() error = %v", err)
		}
		if binary != wantBinary {
			t.Fatalf("ResolveExec() binary = %q, want %q", binary, wantBinary)
		}

		wantArgs := []string{wantBinary, "-c", "printf env override"}
		assertStringSliceEqual(t, args, wantArgs, "ResolveExec() args")
	})
}

func TestLoadConfigEnvOverridePreservesCommaInsideSingleArg(t *testing.T) {
	runWithTempCompileJSON(t, `{"command":"echo"}`, func(compilePath string) {
		t.Setenv("COMPILECONFIG_ARGS", "-Wl,-z,relro")
		cfg := loadConfigWithLoader(t, compilePath, nil)

		want := []string{"-Wl,-z,relro"}
		assertStringSliceEqual(t, cfg.GetArgs(), want, "GetArgs()")
	})
}

func TestLoadConfigFlagOverrideAcceptsLegacyShellStyleArgs(t *testing.T) {
	runWithTempCompileJSON(t, `{"command":"sh","args":["-c","printf file"]}`, func(compilePath string) {
		cfg := loadConfigWithLoader(t, compilePath, []string{"--args=-c 'printf flag override'"})

		wantBinary, err := exec.LookPath("sh")
		if err != nil {
			t.Fatalf("exec.LookPath(sh) error = %v", err)
		}
		binary, args, err := cfg.ResolveExec()
		if err != nil {
			t.Fatalf("ResolveExec() error = %v", err)
		}
		if binary != wantBinary {
			t.Fatalf("ResolveExec() binary = %q, want %q", binary, wantBinary)
		}

		wantArgs := []string{wantBinary, "-c", "printf flag override"}
		assertStringSliceEqual(t, args, wantArgs, "ResolveExec() args")
	})
}

func TestLoadConfigRejectsInvalidArgsJSONType(t *testing.T) {
	runWithTempCompileJSON(t, `{"command":"sh","args":{"broken":true}}`, func(compilePath string) {
		cfg := new(CompileConfig)
		err := multiconfig.MultiLoader(
			&multiconfig.TagLoader{},
			&multiconfig.JSONLoader{Path: compilePath},
		).Load(cfg)
		if err == nil {
			t.Fatal("load should fail for invalid args json type")
		}
	})
}

func TestValidateSandboxRejectsMismatchedUIDGID(t *testing.T) {
	tests := []struct {
		name    string
		uid     int
		gid     int
		wantErr bool
	}{
		{"both -1", -1, -1, false},
		{"both 0", 0, 0, false},
		{"both positive", 1000, 1000, false},
		{"uid positive gid -1", 1000, -1, true},
		{"uid -1 gid positive", -1, 1000, true},
		{"uid positive gid 0", 1000, 0, true},
		{"uid 0 gid positive", 0, 1000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &CompileConfig{RunUID: tt.uid, RunGID: tt.gid}
			err := cfg.ValidateSandbox()
			if tt.wantErr && err == nil {
				t.Fatal("ValidateSandbox() should return error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ValidateSandbox() error = %v, want nil", err)
			}
		})
	}
}

func TestValidateSandboxRejectsNonexistentChrootDir(t *testing.T) {
	cfg := &CompileConfig{
		RunUID:    -1,
		RunGID:    -1,
		ChrootDir: "/nonexistent/chroot/dir/12345",
	}

	err := cfg.ValidateSandbox()
	if err == nil {
		t.Fatal("ValidateSandbox() should return error for nonexistent ChrootDir")
	}
}

func TestValidateSandboxRejectsChrootDirNotDirectory(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "not_a_dir")
	if err := os.WriteFile(tmpFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	cfg := &CompileConfig{
		RunUID:    -1,
		RunGID:    -1,
		ChrootDir: tmpFile,
	}

	err := cfg.ValidateSandbox()
	if err == nil {
		t.Fatal("ValidateSandbox() should return error when ChrootDir is not a directory")
	}
}

func TestValidateSandboxAcceptsValidChrootDir(t *testing.T) {
	cfg := &CompileConfig{
		RunUID:    -1,
		RunGID:    -1,
		ChrootDir: t.TempDir(),
	}

	if err := cfg.ValidateSandbox(); err != nil {
		t.Fatalf("ValidateSandbox() error = %v, want nil", err)
	}
}

func TestValidateSandboxRejectsRelativeWorkDirWithChroot(t *testing.T) {
	cfg := &CompileConfig{
		RunUID:    -1,
		RunGID:    -1,
		ChrootDir: t.TempDir(),
		WorkDir:   "work",
	}

	if err := cfg.ValidateSandbox(); err == nil {
		t.Fatal("ValidateSandbox() should reject relative WorkDir with ChrootDir")
	}
}

func TestSandboxWorkDirDefaultsToRootWhenChrootSet(t *testing.T) {
	cfg := &CompileConfig{ChrootDir: "/jail"}

	got, err := cfg.sandboxWorkDir()
	if err != nil {
		t.Fatalf("sandboxWorkDir() error = %v", err)
	}
	if got != "/" {
		t.Fatalf("sandboxWorkDir() = %q, want /", got)
	}
}

func TestSandboxWorkDirPreservesInheritedCwdWithoutChroot(t *testing.T) {
	cfg := &CompileConfig{}

	got, err := cfg.sandboxWorkDir()
	if err != nil {
		t.Fatalf("sandboxWorkDir() error = %v", err)
	}
	if got != "" {
		t.Fatalf("sandboxWorkDir() = %q, want empty", got)
	}
}

func TestSandboxWorkDirUsesExplicitWorkDir(t *testing.T) {
	cfg := &CompileConfig{ChrootDir: "/jail", WorkDir: "/work"}

	got, err := cfg.sandboxWorkDir()
	if err != nil {
		t.Fatalf("sandboxWorkDir() error = %v", err)
	}
	if got != "/work" {
		t.Fatalf("sandboxWorkDir() = %q, want /work", got)
	}
}

func TestSandboxWorkDirRejectsRelativeWorkDirWithChroot(t *testing.T) {
	cfg := &CompileConfig{ChrootDir: "/jail", WorkDir: "work"}

	if _, err := cfg.sandboxWorkDir(); err == nil {
		t.Fatal("sandboxWorkDir() should reject relative WorkDir with ChrootDir")
	}
}

func TestLoadBootstrapConfigRequiresEnv(t *testing.T) {
	previous, hadPrevious := os.LookupEnv(compilerBootstrapConfigEnv)
	if err := os.Unsetenv(compilerBootstrapConfigEnv); err != nil {
		t.Fatalf("os.Unsetenv(%s) error = %v", compilerBootstrapConfigEnv, err)
	}
	t.Cleanup(func() {
		if hadPrevious {
			_ = os.Setenv(compilerBootstrapConfigEnv, previous)
			return
		}
		_ = os.Unsetenv(compilerBootstrapConfigEnv)
	})

	if _, err := loadBootstrapConfig(); err == nil {
		t.Fatal("loadBootstrapConfig() should reject missing bootstrap config env")
	}
}

func TestLoadBootstrapConfigRejectsEmptyEnv(t *testing.T) {
	t.Setenv(compilerBootstrapConfigEnv, "")

	if _, err := loadBootstrapConfig(); err == nil {
		t.Fatal("loadBootstrapConfig() should reject empty bootstrap config env")
	}
}

func TestBootstrapConfigRoundTripPreservesFields(t *testing.T) {
	cfg := &CompileConfig{
		CPU:        5,
		Memory:     256,
		Output:     32,
		Stack:      16,
		Command:    "g++",
		Verbose:    true,
		LogPath:    "/tmp/compiler.log",
		Args:       newCompileArgs("main.cpp", "-o", "main", "-O2"),
		RunUID:     1000,
		RunGID:     2000,
		ChrootDir:  "/jail",
		WorkDir:    "/work",
		NoNewPrivs: true,
		UseMountNS: true,
		UseIPCNS:   true,
		UseUTSNS:   true,
		UseNetNS:   true,
	}

	encoded, err := encodeBootstrapConfig(cfg)
	if err != nil {
		t.Fatalf("encodeBootstrapConfig() error = %v", err)
	}
	t.Setenv(compilerBootstrapConfigEnv, encoded)

	got, err := loadBootstrapConfig()
	if err != nil {
		t.Fatalf("loadBootstrapConfig() error = %v", err)
	}

	if got.CPU != 5 || got.Memory != 256 || got.Output != 32 || got.Stack != 16 {
		t.Fatalf("resource fields = cpu:%d memory:%d output:%d stack:%d", got.CPU, got.Memory, got.Output, got.Stack)
	}
	if got.Command != "g++" || !got.Verbose || got.LogPath != "/tmp/compiler.log" {
		t.Fatalf("basic fields = command:%q verbose:%v logPath:%q", got.Command, got.Verbose, got.LogPath)
	}
	assertStringSliceEqual(t, got.Args.Values(), []string{"main.cpp", "-o", "main", "-O2"}, "bootstrap Args")
	if got.RunUID != 1000 || got.RunGID != 2000 {
		t.Fatalf("UID/GID = %d/%d, want 1000/2000", got.RunUID, got.RunGID)
	}
	if got.ChrootDir != "/jail" || got.WorkDir != "/work" || !got.NoNewPrivs {
		t.Fatalf("sandbox paths/flags = chroot:%q work:%q noNewPrivs:%v", got.ChrootDir, got.WorkDir, got.NoNewPrivs)
	}
	if !got.UseMountNS || !got.UseIPCNS || !got.UseUTSNS || !got.UseNetNS {
		t.Fatalf("namespace flags = mount:%v ipc:%v uts:%v net:%v", got.UseMountNS, got.UseIPCNS, got.UseUTSNS, got.UseNetNS)
	}
}

func assertStringSliceEqual(t *testing.T, got []string, want []string, label string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s[%d] = %q, want %q", label, i, got[i], want[i])
		}
	}
}

func loadConfigWithLoader(t *testing.T, compilePath string, flagArgs []string) *CompileConfig {
	t.Helper()

	cfg := new(CompileConfig)
	loader := multiconfig.MultiLoader(
		&multiconfig.TagLoader{},
		&multiconfig.JSONLoader{Path: compilePath},
		&multiconfig.EnvironmentLoader{},
		&multiconfig.FlagLoader{Args: flagArgs},
	)
	if err := loader.Load(cfg); err != nil {
		t.Fatalf("loader.Load() error = %v", err)
	}
	return cfg
}

func runWithTempCompileJSON(t *testing.T, content string, fn func(compilePath string)) {
	t.Helper()

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	tempDir := t.TempDir()
	compilePath := filepath.Join(tempDir, "compile.json")
	if err := os.WriteFile(compilePath, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", compilePath, err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("os.Chdir(%q) error = %v", tempDir, err)
	}

	defer func() {
		if err := os.Chdir(previousWD); err != nil {
			t.Fatalf("restore working directory error = %v", err)
		}
	}()

	fn(compilePath)
}
