package main

import (
	"fmt"
	"hustoj/runner/src"
)

func main() {
	task := runner.RunningTask{}
	task.Init(runner.LoadConfig())
	task.Run()

	result := task.GetResult()
	fmt.Println(result)
}
