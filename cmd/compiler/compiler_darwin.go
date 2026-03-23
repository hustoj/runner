//go:build darwin

package main

import (
	"fmt"
	"os"
)

const compilerBootstrapEnv = "RUNNER_COMPILER_BOOTSTRAP"
const compilerDarwinDevelopmentOnlyMessage = "darwin is supported only for development, type-checking, and builds; compiler runtime requires linux"

func compilerDarwinUnavailable(feature string) string {
	return fmt.Sprintf("%s is unavailable on darwin: %s", feature, compilerDarwinDevelopmentOnlyMessage)
}

func initLog(_ *CompileConfig) {
	fmt.Println(compilerDarwinUnavailable("initLog"))
}

func isCompilerBootstrapProcess() bool {
	return os.Getenv(compilerBootstrapEnv) == "1"
}

func bootstrapCompile(_ *CompileConfig) {
	fmt.Fprintln(os.Stderr, compilerDarwinUnavailable("bootstrapCompile"))
	os.Exit(1)
}

func handle(_ *CompileConfig) *RunResult {
	fmt.Println(compilerDarwinUnavailable("handle"))
	return &RunResult{Success: false}
}
