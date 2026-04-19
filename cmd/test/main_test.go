package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hustoj/runner/runner"
)

func TestMaterializeInputWritesInlineContent(t *testing.T) {
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir tmpDir: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatalf("Restore wd: %v", err)
		}
	}()

	setting := &runner.TaskConfig{Input: "1 2 3\n"}
	if err := materializeInput(setting); err != nil {
		t.Fatalf("materializeInput() error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, inputFileName))
	if err != nil {
		t.Fatalf("ReadFile user.in: %v", err)
	}
	if string(content) != "1 2 3\n" {
		t.Fatalf("user.in content = %q, want %q", string(content), "1 2 3\n")
	}
}

func TestMaterializeInputCreatesEmptyFileWhenNoInputProvided(t *testing.T) {
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir tmpDir: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatalf("Restore wd: %v", err)
		}
	}()

	setting := &runner.TaskConfig{}
	if err := materializeInput(setting); err != nil {
		t.Fatalf("materializeInput() error = %v", err)
	}

	info, err := os.Stat(filepath.Join(tmpDir, inputFileName))
	if err != nil {
		t.Fatalf("Stat user.in: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("user.in size = %d, want 0", info.Size())
	}
}

func TestMaterializeInputReadsFromInputFile(t *testing.T) {
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir tmpDir: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatalf("Restore wd: %v", err)
		}
	}()

	source := filepath.Join(tmpDir, "stdin.txt")
	if err := os.WriteFile(source, []byte("abc\n"), 0600); err != nil {
		t.Fatalf("Write source: %v", err)
	}

	setting := &runner.TaskConfig{InputFile: source}
	if err := materializeInput(setting); err != nil {
		t.Fatalf("materializeInput() error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, inputFileName))
	if err != nil {
		t.Fatalf("ReadFile user.in: %v", err)
	}
	if string(content) != "abc\n" {
		t.Fatalf("user.in content = %q, want %q", string(content), "abc\n")
	}
}
