//go:build darwin

package runner

import "fmt"

func GetProcMemoryInfo(pid int) (ProcMemoryInfo, error) {
	return ProcMemoryInfo{}, fmt.Errorf("GetProcMemoryInfo is not supported on darwin")
}
