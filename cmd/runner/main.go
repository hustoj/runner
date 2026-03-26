package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hustoj/runner/runner"
)

func main() {
	if runner.IsBootstrapProcess() {
		runner.BootstrapProcess()
		return
	}

	setting, err := runner.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "runner: %v\n", err)
		os.Exit(1)
	}
	if _, err := runner.InitLogger(setting.LogPath, setting.Verbose); err != nil {
		fmt.Fprintf(os.Stderr, "runner: init logger: %v\n", err)
		os.Exit(1)
	}

	task := runner.RunningTask{}
	task.Init(setting)
	if err := task.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "runner: %v\n", err)
		os.Exit(1)
	}

	result := task.GetResult()
	content, _ := json.Marshal(result)
	fmt.Println(string(content))
}
