//go:build darwin

package runner

import (
	"os"
)

func fileDup(f1 *os.File, f2 *os.File) {
	panic("fileDup is not supported on darwin")
}

func ChangeRunningUser(user int) {
	panic("ChangeRunningUser is not supported on darwin")
}
