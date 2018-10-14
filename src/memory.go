package runner

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
)

func GetProcMemory(pid int) (int, error) {
	path := fmt.Sprintf("/proc/%d/status", pid)
	content, err := ioutil.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("open status file[%s] failed!, %v", path, err))
	}

	return parseMemory(string(content))
}

func parseMemory(content string) (int, error) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		prefix, value := parseLine(line)
		if prefix == "VmHWM" {
			ret, err := parseSize(value)
			if err != nil {
				return ret, err
			}
			return ret, nil
		}
	}
	return 0, errors.New("parse memory failed")
}

func parseLine(line string) (string, string) {
	segments := strings.Split(line, ":")
	if len(segments) != 2 {
		return "", ""
	}

	return strings.Trim(segments[0], " "), strings.Trim(segments[1], " ")
}

func parseSize(info string) (int, error) {
	var size int
	_, err := fmt.Sscanf(info, "%d kB", &size)

	return size, err
}
