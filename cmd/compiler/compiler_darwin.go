//go:build darwin

package main

import (
	"fmt"
)

func initLog(_ *CompileConfig) {
	fmt.Println("initLog is not implemented on macOS")
}

func handle(cfg *CompileConfig) *RunResult {
	fmt.Println("handle is not implemented on macOS")
	return &RunResult{Success: false}
}
