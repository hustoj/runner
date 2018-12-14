package main

import (
	"fmt"
	"hustoj/runner/runner"
)

func main() {
	setting := runner.LoadConfig()
	runner.InitLogger(setting.LogPath, setting.Verbose)

	task := runner.RunningTask{}
	task.Init(setting)
	task.Run()

	result := task.GetResult()

	if result.RetCode != setting.Result {
		fmt.Printf("Result not match, expect: %d, actual: %d\n", setting.Result, result.RetCode)
	} else {
		fmt.Printf("Case passed! expect: %d, actual: %d\n", setting.Result, result.RetCode)
	}
}

