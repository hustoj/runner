package runner

import (
	"github.com/koding/multiconfig"
)

type TaskConfig struct {
	CPU    int `default:"3"`
	Memory int `default:"256"`
	Output int `default:"64"`
	Stack  int `default:"8"`

	Command     string `default:"./main"`
	Language    int `default:"2"`

	OneTimeCalls []string `default:"execve"`
	AllowedCalls []string `default:"read,write,brk,fstat,uname,mmap,arch_prctl,exit_group"`
	Verbose      bool     `default:"false"`
	Result      int `default:"4"`

	LogPath     string `default:"/var/log/runner/runner.log"`
}

func LoadConfig() *TaskConfig {
	m := multiconfig.NewWithPath("case.json")
	setting := new(TaskConfig)
	m.MustLoad(setting)

	return setting
}
