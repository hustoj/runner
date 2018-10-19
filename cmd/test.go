package main

import (
	"fmt"
	"hustoj/runner/src"
	"os"
	"strconv"
)

func main() {
	setting := &runner.Setting{}
	if len(os.Args) < 3 {
		panic("argument should be time, memory, retcode")
	}

	setting.TimeLimit = parseArg(1)
	setting.MemoryLimit = parseArg(2)

	retCode := parseArg(3)

	task := runner.RunningTask{}
	task.Init(setting)
	task.Run()

	result := task.GetResult()

	if result.RetCode != retCode {
		fmt.Printf("retcode not match, expect: %d, actual: %d\n", retCode, result.RetCode)
	}
}

func parseArg(index int) int {
	ret, err := strconv.Atoi(os.Args[index])
	if err != nil {
		panic(err)
	}
	return ret
}
