//go:build linux

package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindDelegatedCgroupParentFindsNearestWritableAncestor(t *testing.T) {
	mountRoot := t.TempDir()
	parent := filepath.Join(mountRoot, "user.slice", "user-1000.slice", "user@1000.service", "app.slice")
	current := filepath.Join(parent, "app-terminal.scope")
	for _, dir := range []string{parent, current} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v", dir, err)
		}
	}

	writeTestCgroupFile(t, filepath.Join(parent, "cgroup.type"), "domain\n")
	writeTestCgroupFile(t, filepath.Join(parent, "cgroup.subtree_control"), "memory pids\n")
	writeTestCgroupFile(t, filepath.Join(parent, "cgroup.procs"), "")
	writeTestCgroupFile(t, filepath.Join(current, "cgroup.type"), "domain\n")
	writeTestCgroupFile(t, filepath.Join(current, "cgroup.subtree_control"), "")
	writeTestCgroupFile(t, filepath.Join(current, "cgroup.procs"), "100\n101\n")

	parentPath, err := findDelegatedCgroupParent(
		mountRoot,
		"/user.slice/user-1000.slice/user@1000.service/app.slice/app-terminal.scope",
	)
	if err != nil {
		t.Fatalf("findDelegatedCgroupParent() error = %v", err)
	}
	if parentPath != parent {
		t.Fatalf("findDelegatedCgroupParent() = %q, want %q", parentPath, parent)
	}
}

func TestResolveConfiguredCgroupParentKeepsMountBoundaries(t *testing.T) {
	mountRoot := t.TempDir()

	tests := []struct {
		name       string
		configured string
		want       string
	}{
		{
			name:       "mount-relative absolute path",
			configured: "/runner",
			want:       filepath.Join(mountRoot, "runner"),
		},
		{
			name:       "filesystem absolute path",
			configured: filepath.Join(mountRoot, "runner"),
			want:       filepath.Join(mountRoot, "runner"),
		},
		{
			name:       "relative path",
			configured: "delegated/runner",
			want:       filepath.Join(mountRoot, "delegated", "runner"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveConfiguredCgroupParent(mountRoot, tt.configured)
			if err != nil {
				t.Fatalf("resolveConfiguredCgroupParent() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolveConfiguredCgroupParent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveConfiguredCgroupParentRejectsEscapes(t *testing.T) {
	mountRoot := t.TempDir()

	if _, err := resolveConfiguredCgroupParent(mountRoot, "../../etc"); err == nil {
		t.Fatal("resolveConfiguredCgroupParent() error = nil, want path escape rejection")
	}
}

func TestCgroupMemoryControllerStatusUsesPeakAndEvents(t *testing.T) {
	controller := &cgroupMemoryController{path: t.TempDir()}
	writeTestCgroupFile(t, filepath.Join(controller.path, "memory.peak"), "8193\n")
	writeTestCgroupFile(t, filepath.Join(controller.path, "memory.events"), "low 0\nhigh 0\nmax 3\noom 1\noom_kill 0\n")

	status, err := controller.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.PeakMemoryKB != 9 {
		t.Fatalf("Status() peak = %d, want 9", status.PeakMemoryKB)
	}
	if !status.Exceeded() {
		t.Fatal("Status() Exceeded() = false, want true when oom counter is non-zero")
	}
}

func writeTestCgroupFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
}
