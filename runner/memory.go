package runner

import (
	"errors"
	"fmt"
	"strings"
)

// ErrVmHWMNotFound is returned when /proc/<pid>/status does not contain
// a VmHWM line (e.g. kernel thread or process already exited).
var ErrVmHWMNotFound = errors.New("VmHWM not found in proc status")

func parseMemory(content string) (int64, error) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "VmHWM") {
			_, value := parseLine(line)
			ret, err := parseSize(value)
			return ret, err
		}
	}
	return 0, ErrVmHWMNotFound
}

func parseLine(line string) (string, string) {
	segments := strings.Split(line, ":")
	if len(segments) != 2 {
		return "", ""
	}

	return strings.Trim(segments[0], " "), strings.Trim(segments[1], " ")
}

func parseSize(info string) (int64, error) {
	var size int64
	_, err := fmt.Sscanf(info, "%d kB", &size)

	return size, err
}
