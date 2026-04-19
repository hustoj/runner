package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hustoj/runner/runner"
)

func TestMaterializeInputWritesInlineContent(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("os.Chdir(%q) error = %v", tempDir, err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working directory error = %v", err)
		}
	}()

	setting := &runner.TaskConfig{Input: "1 2 3\n"}
	if err := materializeInput(setting); err != nil {
		t.Fatalf("materializeInput() error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tempDir, inputFileName))
	if err != nil {
		t.Fatalf("os.ReadFile(user.in) error = %v", err)
	}
	if string(content) != "1 2 3\n" {
		t.Fatalf("user.in content = %q, want %q", string(content), "1 2 3\n")
	}
}

func TestMaterializeInputCreatesEmptyFileWhenNoInputProvided(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("os.Chdir(%q) error = %v", tempDir, err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working directory error = %v", err)
		}
	}()

	setting := &runner.TaskConfig{}
	if err := materializeInput(setting); err != nil {
		t.Fatalf("materializeInput() error = %v", err)
	}

	info, err := os.Stat(filepath.Join(tempDir, inputFileName))
	if err != nil {
		t.Fatalf("os.Stat(user.in) error = %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("user.in size = %d, want 0", info.Size())
	}
}

func TestMaterializeInputReadsFromInputFile(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("os.Chdir(%q) error = %v", tempDir, err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working directory error = %v", err)
		}
	}()

	source := filepath.Join(tempDir, "stdin.txt")
	if err := os.WriteFile(source, []byte("abc\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", source, err)
	}

	setting := &runner.TaskConfig{InputFile: source}
	if err := materializeInput(setting); err != nil {
		t.Fatalf("materializeInput() error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tempDir, inputFileName))
	if err != nil {
		t.Fatalf("os.ReadFile(user.in) error = %v", err)
	}
	if string(content) != "abc\n" {
		t.Fatalf("user.in content = %q, want %q", string(content), "abc\n")
	}
}
