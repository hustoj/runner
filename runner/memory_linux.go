//go:build linux

package runner

import (
	"fmt"
	"os"
)

func GetProcMemory(pid int) (int64, error) {
	path := fmt.Sprintf("/proc/%d/status", pid)
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	return parseMemory(string(content))
}
