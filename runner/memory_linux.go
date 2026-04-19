//go:build linux

package runner

import (
	"fmt"
	"os"
)

func readProcStatus(pid int) ([]byte, error) {
	path := fmt.Sprintf("/proc/%d/status", pid)
	return os.ReadFile(path)
}

func GetProcMemoryInfo(pid int) (ProcMemoryInfo, error) {
	content, err := readProcStatus(pid)
	if err != nil {
		return ProcMemoryInfo{}, err
	}

	return parseMemoryInfo(string(content))
}

// Deprecated: prefer GetProcMemoryInfo to avoid parsing /proc status twice.
func GetProcMemory(pid int) (int64, error) {
	info, err := GetProcMemoryInfo(pid)
	if err != nil {
		return 0, err
	}
	return info.PeakMemory, nil
}

// Deprecated: prefer GetProcMemoryInfo to avoid parsing /proc status twice.
func GetProcThreadGroup(pid int) (int, error) {
	info, err := GetProcMemoryInfo(pid)
	if err != nil {
		return 0, err
	}

	return info.ThreadGroup, nil
}
