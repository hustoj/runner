package main

import (
	"fmt"
	"github.com/hustoj/runner/runner"
)

func main() {
	setting := runner.LoadConfig()
	runner.InitLogger(setting.LogPath, setting.Verbose)

	task := runner.RunningTask{}
	task.Init(setting)
	task.Run()

	result := task.GetResult()

	if result.RetCode != setting.Result {
		fmt.Printf("%s failed! Result not match!\nexpect: %d, actual: %d\n", setting.Name, setting.Result, result.RetCode)
	} else {
		fmt.Printf("%s passed!\n", setting.Name)
	}
}
