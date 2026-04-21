package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/google/shlex"
	"github.com/koding/multiconfig"
)

type CompileArgs struct {
	values []string
}

func newCompileArgs(values ...string) *CompileArgs {
	return &CompileArgs{values: append([]string(nil), values...)}
}

func (args CompileArgs) Values() []string {
	values := make([]string, len(args.values))
	copy(values, args.values)
	return values
}

func (args CompileArgs) Len() int {
	return len(args.values)
}

func (args CompileArgs) String() string {
	return strings.Join(args.values, " ")
}

func (args *CompileArgs) Set(value string) error {
	parsed, err := parseCompileArgsValue(value)
	if err != nil {
		return err
	}
	args.values = append(args.values[:0], parsed...)
	return nil
}

func (args *CompileArgs) UnmarshalJSON(data []byte) error {
	switch trimmed := strings.TrimSpace(string(data)); trimmed {
	case "null":
		args.values = nil
		return nil
	default:
		var values []string
		if err := json.Unmarshal(data, &values); err == nil {
			args.values = append(args.values[:0], values...)
			return nil
		}

		var legacy string
		if err := json.Unmarshal(data, &legacy); err == nil {
			return args.Set(legacy)
		}
	}

	return fmt.Errorf("compile args must be a JSON string or array of strings")
}

func parseCompileArgsValue(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	return shlex.Split(value)
}

type CompileConfig struct {
	CPU     int          `default:"3"`
	Memory  int          `default:"128"`
	Output  int          `default:"16"`
	Stack   int          `default:"8"`
	Command string       `default:"gcc"`
	Verbose bool         `default:"false"`
	LogPath string       `default:"/var/log/runner/compiler.log"`
	Args    *CompileArgs `default:"main.c -o main -O2 -fmax-errors=10 -Wall --static -lm --std=c99"`

	commands []string
	parseErr error
}

func (config *CompileConfig) GetArgs() []string {
	if err := config.parseCommand(); err != nil || len(config.commands) <= 1 {
		return nil
	}
	args := make([]string, len(config.commands)-1)
	copy(args, config.commands[1:])
	return args
}

func (config *CompileConfig) parseCommand() error {
	if len(config.commands) > 0 || config.parseErr != nil {
		return config.parseErr
	}

	switch {
	case config.Args != nil && config.Args.Len() > 0:
		config.commands = append([]string{config.Command}, config.Args.Values()...)
		return nil
	default:
		config.commands, config.parseErr = shlex.Split(config.Command)
		if len(config.commands) == 0 {
			config.commands = []string{config.Command}
		}
		return config.parseErr
	}
}

func (config *CompileConfig) ResolveExec() (string, []string, error) {
	if err := config.parseCommand(); err != nil {
		return "", nil, err
	}
	if len(config.commands) == 0 || config.commands[0] == "" {
		return "", nil, errors.New("empty command")
	}

	command := config.commands[0]
	binary := command
	if !strings.Contains(command, "/") {
		resolved, err := exec.LookPath(command)
		if err != nil {
			return "", nil, err
		}
		binary = resolved
	}

	args := make([]string, 0, len(config.commands))
	args = append(args, binary)
	args = append(args, config.commands[1:]...)

	return binary, args, nil
}

func loadConfig() *CompileConfig {
	m := multiconfig.NewWithPath("compile.json")
	compileConfig := new(CompileConfig)
	m.MustLoad(compileConfig)
	return compileConfig
}
