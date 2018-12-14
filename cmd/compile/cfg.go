package main

import (
	"github.com/koding/multiconfig"
)

type CompileConfig struct {
	CPU     int    `default:"3"`
	Memory  int    `default:"128"`
	Output  int    `default:"16"`
	Stack   int    `default:"8"`
	Command string `default:"/usr/bin/gcc"`
	Verbose bool   `default:"false"`
}

func loadConfig() *CompileConfig {
	m := multiconfig.New()
	compileConfig := new(CompileConfig)
	m.MustLoad(compileConfig)
	return compileConfig
}
