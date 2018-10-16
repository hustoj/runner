package runner

// #include "cfork.h"
import "C"
import (
	"github.com/sirupsen/logrus"
	"syscall"
)

func runProcess() int {
	value := C.fork_and_return()
	return int(value)
}

func ChangeRunningUser(user int) {
	err := syscall.Setuid(user)
	if err != nil {
		logrus.Panicf("set running uid failed %v", err)
	}
}
