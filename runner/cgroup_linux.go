//go:build linux

package runner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const (
	defaultCgroupMountRoot = "/sys/fs/cgroup"
	cgroupMountEnv         = "RUNNER_CGROUP_MOUNT"
	cgroupParentEnv        = "RUNNER_CGROUP_PARENT"
)

func newTaskController(setting *TaskConfig) (taskController, error) {
	return newCgroupTaskController(setting.Memory, setting.MaxProcs)
}

type cgroupTaskController struct {
	path string
}

func newCgroupTaskController(limitMB int, maxProcs int) (*cgroupTaskController, error) {
	mountRoot, err := detectCgroupMountRoot()
	if err != nil {
		return nil, err
	}

	parentPath, err := resolveCgroupParent(mountRoot)
	if err != nil {
		return nil, err
	}

	controller := &cgroupTaskController{
		path: filepath.Join(parentPath, taskCgroupName()),
	}
	if err := os.Mkdir(controller.path, 0o755); err != nil {
		return nil, fmt.Errorf("create task cgroup %q: %w", controller.path, err)
	}
	if err := controller.configure(limitMB, maxProcs); err != nil {
		if cleanupErr := controller.Cleanup(); cleanupErr != nil {
			return nil, errors.Join(err, fmt.Errorf("cleanup task cgroup %q: %w", controller.path, cleanupErr))
		}
		return nil, err
	}

	return controller, nil
}

func detectCgroupMountRoot() (string, error) {
	mountRoot := os.Getenv(cgroupMountEnv)
	if mountRoot == "" {
		mountRoot = defaultCgroupMountRoot
	}
	mountRoot = filepath.Clean(mountRoot)

	var statfs unix.Statfs_t
	if err := unix.Statfs(mountRoot, &statfs); err != nil {
		return "", fmt.Errorf("stat cgroup mount %q: %w", mountRoot, err)
	}
	if uint64(statfs.Type) != uint64(unix.CGROUP2_SUPER_MAGIC) {
		return "", fmt.Errorf("%q is not a cgroup v2 mount", mountRoot)
	}

	return mountRoot, nil
}

func resolveCgroupParent(mountRoot string) (string, error) {
	if configured := os.Getenv(cgroupParentEnv); configured != "" {
		parentPath, err := resolveConfiguredCgroupParent(mountRoot, configured)
		if err != nil {
			return "", err
		}
		if ok, reason, err := isUsableCgroupParent(mountRoot, parentPath); err != nil {
			return "", err
		} else if !ok {
			return "", fmt.Errorf("%s=%q is not usable: %s", cgroupParentEnv, configured, reason)
		}
		return parentPath, nil
	}

	currentPath, err := readSelfCgroupPath()
	if err != nil {
		return "", err
	}

	parentPath, err := findDelegatedCgroupParent(mountRoot, currentPath)
	if err != nil {
		return "", err
	}
	return parentPath, nil
}

func resolveConfiguredCgroupParent(mountRoot string, configured string) (string, error) {
	cleaned := filepath.Clean(configured)
	var parentPath string
	switch {
	case cleaned == mountRoot:
		parentPath = mountRoot
	case strings.HasPrefix(cleaned, mountRoot+string(os.PathSeparator)):
		parentPath = cleaned
	case filepath.IsAbs(cleaned):
		parentPath = filepath.Join(mountRoot, strings.TrimPrefix(cleaned, string(os.PathSeparator)))
	default:
		parentPath = filepath.Join(mountRoot, cleaned)
	}

	if !isWithinPath(mountRoot, parentPath) {
		return "", fmt.Errorf("configured cgroup parent %q escapes mount root %q", configured, mountRoot)
	}

	return parentPath, nil
}

func readSelfCgroupPath() (string, error) {
	content, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", fmt.Errorf("read /proc/self/cgroup: %w", err)
	}

	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "0::") {
			path := strings.TrimSpace(strings.TrimPrefix(line, "0::"))
			if path == "" {
				return "/", nil
			}
			return path, nil
		}
	}

	return "", errors.New("cgroup v2 membership not found in /proc/self/cgroup")
}

func findDelegatedCgroupParent(mountRoot string, currentCgroupPath string) (string, error) {
	currentPath := filepath.Join(mountRoot, strings.TrimPrefix(filepath.Clean(currentCgroupPath), string(os.PathSeparator)))
	for candidate := currentPath; ; candidate = filepath.Dir(candidate) {
		ok, _, err := isUsableCgroupParent(mountRoot, candidate)
		if err == nil && ok {
			return candidate, nil
		}
		if candidate == mountRoot {
			break
		}
	}

	return "", fmt.Errorf(
		"no delegated cgroup parent with required controllers enabled found under %q; configure %s to a writable domain cgroup whose subtree_control includes memory and pids",
		mountRoot,
		cgroupParentEnv,
	)
}

func isUsableCgroupParent(mountRoot string, path string) (bool, string, error) {
	if !isWithinPath(mountRoot, path) {
		return false, "outside cgroup mount", nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, "does not exist", nil
		}
		return false, "", fmt.Errorf("stat %q: %w", path, err)
	}
	if !info.IsDir() {
		return false, "not a directory", nil
	}
	if err := unix.Access(path, unix.W_OK|unix.X_OK); err != nil {
		return false, "directory is not writable", nil
	}

	typePath := filepath.Join(path, "cgroup.type")
	if data, err := os.ReadFile(typePath); err == nil {
		kind := strings.TrimSpace(string(data))
		if kind != "" && kind != "domain" {
			return false, fmt.Sprintf("cgroup.type is %q", kind), nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, "", fmt.Errorf("read %q: %w", typePath, err)
	}

	subtreeControl, err := os.ReadFile(filepath.Join(path, "cgroup.subtree_control"))
	if err != nil {
		return false, "", fmt.Errorf("read subtree control for %q: %w", path, err)
	}
	if !containsWord(string(subtreeControl), "memory") {
		return false, "memory controller is not enabled in subtree_control", nil
	}
	if !containsWord(string(subtreeControl), "pids") {
		return false, "pids controller is not enabled in subtree_control", nil
	}

	if path != mountRoot {
		procs, err := os.ReadFile(filepath.Join(path, "cgroup.procs"))
		if err != nil {
			return false, "", fmt.Errorf("read cgroup.procs for %q: %w", path, err)
		}
		if strings.TrimSpace(string(procs)) != "" {
			return false, "cgroup has internal processes", nil
		}
	}

	return true, "", nil
}

func (controller *cgroupTaskController) configure(limitMB int, maxProcs int) error {
	requiredFiles := []string{
		"cgroup.procs",
		"memory.max",
		"memory.events",
		"memory.peak",
		"memory.oom.group",
		"pids.max",
	}
	for _, name := range requiredFiles {
		if _, err := os.Stat(filepath.Join(controller.path, name)); err != nil {
			return fmt.Errorf("task cgroup missing %s: %w", name, err)
		}
	}

	if err := writeCgroupFile(filepath.Join(controller.path, "memory.oom.group"), "1"); err != nil {
		return fmt.Errorf("write memory.oom.group: %w", err)
	}
	if err := writeCgroupFile(filepath.Join(controller.path, "memory.max"), strconv.FormatUint(uint64(limitMB)<<20, 10)); err != nil {
		return fmt.Errorf("write memory.max: %w", err)
	}
	if err := writeCgroupFile(filepath.Join(controller.path, "pids.max"), strconv.Itoa(maxProcs)); err != nil {
		return fmt.Errorf("write pids.max: %w", err)
	}

	swapMaxPath := filepath.Join(controller.path, "memory.swap.max")
	if _, err := os.Stat(swapMaxPath); err == nil {
		if err := writeCgroupFile(swapMaxPath, "0"); err != nil {
			return fmt.Errorf("write memory.swap.max: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat memory.swap.max: %w", err)
	}

	return nil
}

func (controller *cgroupTaskController) MemoryStatus() (memoryStatus, error) {
	peakBytes, err := readUintFromCgroupFile(filepath.Join(controller.path, "memory.peak"))
	if err != nil {
		return memoryStatus{}, fmt.Errorf("read memory.peak: %w", err)
	}
	events, err := readFlatKeyedCgroupFile(filepath.Join(controller.path, "memory.events"))
	if err != nil {
		return memoryStatus{}, fmt.Errorf("read memory.events: %w", err)
	}

	return memoryStatus{
		PeakMemoryKB: bytesToKB(peakBytes),
		OOMCount:     int64(events["oom"]),
		OOMKillCount: int64(events["oom_kill"]),
	}, nil
}

func (controller *cgroupTaskController) MovePID(pid int) error {
	return writeCgroupFile(filepath.Join(controller.path, "cgroup.procs"), strconv.Itoa(pid))
}

func (controller *cgroupTaskController) Cleanup() error {
	if err := os.Remove(controller.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func readUintFromCgroupFile(path string) (uint64, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	value, err := strconv.ParseUint(strings.TrimSpace(string(content)), 10, 64)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func readFlatKeyedCgroupFile(path string) (map[string]uint64, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	values := make(map[string]uint64)
	for _, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid cgroup file line %q", line)
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse value for %q: %w", fields[0], err)
		}
		values[fields[0]] = value
	}

	return values, nil
}

func writeCgroupFile(path string, content string) error {
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	if _, err := file.WriteString(content); err != nil {
		return err
	}
	return nil
}

func bytesToKB(value uint64) int64 {
	return int64((value + 1023) / 1024)
}

func containsWord(content string, needle string) bool {
	for _, word := range strings.Fields(content) {
		if word == needle {
			return true
		}
	}
	return false
}

func isWithinPath(root string, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func taskCgroupName() string {
	return fmt.Sprintf("runner-%d-%d", os.Getpid(), time.Now().UnixNano())
}
