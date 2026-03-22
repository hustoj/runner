//go:build darwin

package runner

import "fmt"

func GetProcMemory(pid int) (int64, error) {
	return 0, fmt.Errorf("GetProcMemory is not supported on darwin")
}
