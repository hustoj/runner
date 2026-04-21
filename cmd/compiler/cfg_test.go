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
