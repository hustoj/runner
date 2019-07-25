package runner

import (
	"github.com/koding/multiconfig"
	"strings"
)

type TaskConfig struct {
	CPU    int `default:"3"`
	Memory int `default:"256"`
	Output int `default:"16"`
	Stack  int `default:"8"`

	Command  string `default:"./main"`
	Language int    `default:"2"`

	OneTimeCalls []string `default:"execve"`
	AllowedCalls []string `default:"read,write,brk,fstat,uname,mmap,arch_prctl,exit_group,readlink,access"`
	Verbose      bool     `default:"false"`
	Result       int      `default:"4"`

	LogPath  string `default:"/var/log/runner/runner.log"`
	commands []string
}

func (tc *TaskConfig) GetCommand() string {
	tc.parseCommand()
	return tc.commands[0]
}

func (tc *TaskConfig)parseCommand()  {
	if len(tc.commands) == 0 {
		tc.commands = strings.Split(tc.Command, " ")
	}
}

func (tc *TaskConfig) GetArgs() []string {
	tc.parseCommand()
	return tc.commands
}

func LoadConfig() *TaskConfig {
	m := multiconfig.NewWithPath("case.json")
	setting = new(TaskConfig)
	m.MustLoad(setting)

	return setting
}
