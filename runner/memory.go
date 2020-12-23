package runner

import (
	"fmt"
	"io/ioutil"
	"strings"
)

func GetProcMemory(pid int) (int64, error) {
	path := fmt.Sprintf("/proc/%d/status", pid)
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, err
	}

	return parseMemory(string(content))
}

func parseMemory(content string) (int64, error) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "VmHWM") {
			_, value := parseLine(line)
			ret, err := parseSize(value)
			return ret, err
		}
	}
	return 0, nil
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
