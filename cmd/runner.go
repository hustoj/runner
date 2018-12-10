package main

import (
	"encoding/json"
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
	content, _ := json.Marshal(result)
	fmt.Println(string(content))
}
