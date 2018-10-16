package runner

// #include "cfork.h"
import "C"
import (
	"github.com/sirupsen/logrus"
	"syscall"
)

func clacDuration(rusage *syscall.Rusage) int64 {
	total := rusage.Utime.Sec*1000 + int64(rusage.Utime.Usec/1000)
	total = total + rusage.Stime.Sec*1000 + int64(rusage.Utime.Usec/1000)

	return total
}

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
