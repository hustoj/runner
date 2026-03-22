//go:build darwin

package main

import (
	"fmt"
)

func handle(cfg *CompileConfig) *RunResult {
	fmt.Println("handle is not implemented on macOS")
	return &RunResult{Success: false}
}
