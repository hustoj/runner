package main

import (
	"encoding/json"
	"fmt"
	"hustoj/runner/runner"
	"os"
)

func main() {
	value, exist := os.LookupEnv("JUDGE_DEBUG")
	if exist && value == "true" {
		runner.Debug()
	}
	task := runner.RunningTask{}
	task.Init(runner.LoadConfig())
	task.Run()

	result := task.GetResult()
	content, _ := json.Marshal(result)
	fmt.Println(string(content))
}
