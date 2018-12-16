package main

import (
	"github.com/koding/multiconfig"
	"strings"
)

type CompileConfig struct {
	CPU     int    `default:"3"`
	Memory  int    `default:"128"`
	Output  int    `default:"16"`
	Stack   int    `default:"8"`
	Command string `default:"gcc"`
	Verbose bool   `default:"false"`
	LogPath string `default:"/var/log/runner/compiler.log"`
	Args    string `default:"main.c -o main -O2 -fmax-errors=10 -Wall --static -lm --std=c99"`
}

func (config *CompileConfig) GetArgs() []string {
	return strings.Split(config.Args, " ")
}

func loadConfig() *CompileConfig {
	m := multiconfig.NewWithPath("compile.json")
	compileConfig := new(CompileConfig)
	m.MustLoad(compileConfig)
	return compileConfig
}
