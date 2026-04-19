//go:build linux && amd64

package runner

import "syscall"

func fork() (int, syscall.Errno) {
	r1, _, errno := syscall.RawSyscall(syscall.SYS_FORK, 0, 0, 0)
	if errno != 0 {
		return -1, errno
	}
	return int(r1), 0
}
