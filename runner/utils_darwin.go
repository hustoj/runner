//go:build darwin

package runner

import (
	"fmt"
	"os"
)

const darwinDevelopmentOnlyMessage = "darwin is supported only for development, type-checking, and builds; runtime execution requires linux"

func darwinDevelopmentOnlyError(feature string) error {
	return fmt.Errorf("%s is unavailable on darwin: %s", feature, darwinDevelopmentOnlyMessage)
}

func panicDarwinDevelopmentOnly(feature string) {
	panic(darwinDevelopmentOnlyError(feature))
}

func openFileNoFollow(filename string, flag int, perm uint32) (*os.File, error) {
	return os.OpenFile(filename, flag, os.FileMode(perm))
}

func fileDupErr(_ *os.File, _ *os.File) error {
	return darwinDevelopmentOnlyError("fileDupErr")
}
