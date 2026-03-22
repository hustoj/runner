package main

import (
	"encoding/json"
	"fmt"
	"go.uber.org/zap"

	"github.com/hustoj/runner/runner"
)

var log *zap.SugaredLogger

type RunResult struct {
	Success bool `json:"success"`
}

func makeArgs(binary string, cfg *CompileConfig) []string {
	args := cfg.GetArgs()
	return append([]string{binary}, args...)
}

func main() {
	m := loadConfig()
	log = runner.InitLogger(m.LogPath, m.Verbose)
	r := handle(m)
	res, _ := json.Marshal(r)
	fmt.Println(string(res))
}
