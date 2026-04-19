package runner

import (
	"fmt"
	"strconv"
	"strings"
)

type ProcMemoryInfo struct {
	ThreadGroup int
	PeakMemory  int64
}

func parseMemory(content string) (int64, error) {
	value, ok := parseStatusField(content, "VmHWM")
	if !ok {
		return 0, nil
	}
	return parseSize(value)
}

func parseThreadGroupID(content string) (int, error) {
	value, ok := parseStatusField(content, "Tgid")
	if !ok {
		return 0, fmt.Errorf("tgid not found")
	}
	return strconv.Atoi(strings.TrimSpace(value))
}

func parseMemoryInfo(content string) (ProcMemoryInfo, error) {
	threadGroup, err := parseThreadGroupID(content)
	if err != nil {
		return ProcMemoryInfo{}, err
	}
	peakMemory, err := parseMemory(content)
	if err != nil {
		return ProcMemoryInfo{}, err
	}
	return ProcMemoryInfo{
		ThreadGroup: threadGroup,
		PeakMemory:  peakMemory,
	}, nil
}

func parseStatusField(content string, field string) (string, bool) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, field+":") {
			_, value := parseLine(line)
			return value, true
		}
	}
	return "", false
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
