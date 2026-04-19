//go:build linux

package runner

import (
	"os"
	"syscall"
)

func fileDup(f1 *os.File, f2 *os.File) {
	syscall.Dup2(int(f1.Fd()), int(f2.Fd()))
	f1.Close()
}

func fork() (int, syscall.Errno) {
	r1, _, errno := syscall.RawSyscall(syscall.SYS_FORK, 0, 0, 0)
	if errno != 0 || r1 < 0 {
		return -1, errno
	}
	return int(r1), 0
}

func ChangeRunningUser(user int) {
	err := syscall.Setuid(user)
	if err != nil {
		log.Panicf("set running uid failed %v", err)
	}
}
