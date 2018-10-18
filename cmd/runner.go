package main

import (
	"hustoj/runner/src"
)

func main() {
	task := runner.RunningTask{}
	runner.set
	task.Init(runner.LoadConfig())
	task.Run()

	task.GetResult()

}
