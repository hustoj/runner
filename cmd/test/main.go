package main

import (
	"fmt"
	"os"

	"github.com/hustoj/runner/runner"
)

const inputFileName = "user.in"

func materializeInput(setting *runner.TaskConfig) error {
	content := []byte(setting.Input)
	if setting.InputFile != "" {
		data, err := os.ReadFile(setting.InputFile)
		if err != nil {
			return err
		}
		content = data
	}
	return os.WriteFile(inputFileName, content, 0600)
}

func main() {
	if runner.IsBootstrapProcess() {
		runner.BootstrapProcess()
		return
	}

	setting := runner.LoadConfig()
	runner.InitLogger(setting.LogPath, setting.Verbose)
	if err := materializeInput(setting); err != nil {
		fmt.Printf("prepare input failed: %v\n", err)
		os.Exit(1)
	}

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
