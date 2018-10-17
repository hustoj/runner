package runner

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
)

type Setting struct {
	TimeLimit   int
	MemoryLimit int
	Language    int // language
}

func LoadConfig() *Setting {
	contents, err := ioutil.ReadFile("case.conf")
	if err != nil {
		panic(fmt.Sprintf("read solution config failed: %v", err))
	}
	return ParseSettingContent(string(contents))
}

func ParseSettingContent(content string) *Setting {
	lines := strings.Split(content, "\n")
	if len(lines) != 3 {
		msg := fmt.Sprintf("solution config format invalid, %v", lines)
		panic(msg)
	}
	setting := &Setting{
		TimeLimit:   parseInt(lines[0]),
		MemoryLimit: parseInt(lines[1]),
		Language:    parseInt(lines[2]),
	}

	return setting
}

func parseInt(content string) int {
	number, err := strconv.Atoi(content)
	if err != nil {
		panic("parse int failed")
	}

	return number
}
