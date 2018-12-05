package runner

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type Setting struct {
	TimeLimit   int `json:"time_limit"`
	MemoryLimit int `json:"memory_limit"`
	Language    int `json:"language"`
}

func LoadConfig() *Setting {
	contents, err := ioutil.ReadFile("case.conf")
	if err != nil {
		log.Panicln(fmt.Sprintf("read solution config failed: %v", err))
	}
	return ParseSettingContent(contents)
}

func ParseSettingContent(content []byte) *Setting {
	conf := &Setting{}
	err := json.Unmarshal(content, conf)
	if err != nil {
		log.Panicf("parse case config failed : %v", err)
	}

	return conf
}
