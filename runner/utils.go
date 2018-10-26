package runner

import (
	"os"
	"syscall"
)

func fileDup(f1 *os.File, f2 *os.File) {
	syscall.Dup2(int(f1.Fd()), int(f2.Fd()))
	f1.Close()
}

func fork() int {
	r1, _, errno := syscall.Syscall(syscall.SYS_FORK, 0, 0, 0)
	if errno != 0 {
		log.Panic("fork failed", errno)
	}
	return int(r1)
}

func ChangeRunningUser(user int) {
	err := syscall.Setuid(user)
	if err != nil {
		log.Panicf("set running uid failed %v\n", err)
	}
}
