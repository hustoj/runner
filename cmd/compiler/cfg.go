package main

import (
	"errors"
	"os/exec"

	"github.com/google/shlex"
	"github.com/koding/multiconfig"
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
	args    []string
	argsErr error
}

func (config *CompileConfig) GetArgs() []string {
	if err := config.parseArgs(); err != nil {
		return nil
	}
	args := make([]string, len(config.args))
	copy(args, config.args)
	return args
}

func (config *CompileConfig) parseArgs() error {
	if len(config.args) == 0 && config.argsErr == nil {
		config.args, config.argsErr = shlex.Split(config.Args)
	}
	return config.argsErr
}

func (config *CompileConfig) ResolveExec() (string, []string, error) {
	if err := config.parseArgs(); err != nil {
		return "", nil, err
	}
	if config.Command == "" {
		return "", nil, errors.New("empty command")
	}

	binary, err := exec.LookPath(config.Command)
	if err != nil {
		return "", nil, err
	}

	return binary, append([]string{binary}, config.GetArgs()...), nil
}

func loadConfig() *CompileConfig {
	m := multiconfig.NewWithPath("compile.json")
	compileConfig := new(CompileConfig)
	m.MustLoad(compileConfig)
	return compileConfig
}
