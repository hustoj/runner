//go:build darwin

package runner

import "fmt"

// Deprecated: prefer GetProcMemoryInfo to avoid parsing /proc status twice.
func GetProcMemory(pid int) (int64, error) {
	return 0, fmt.Errorf("GetProcMemory is not supported on darwin")
}

// Deprecated: prefer GetProcMemoryInfo to avoid parsing /proc status twice.
func GetProcThreadGroup(pid int) (int, error) {
	return 0, fmt.Errorf("GetProcThreadGroup is not supported on darwin")
}

func GetProcMemoryInfo(pid int) (ProcMemoryInfo, error) {
	return ProcMemoryInfo{}, fmt.Errorf("GetProcMemoryInfo is not supported on darwin")
}
