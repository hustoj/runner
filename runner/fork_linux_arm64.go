//go:build linux && arm64

package runner

import "syscall"

func fork() (int, syscall.Errno) {
	r1, _, errno := syscall.RawSyscall6(
		syscall.SYS_CLONE,
		uintptr(syscall.SIGCHLD),
		0,
		0,
		0,
		0,
		0,
	)
	if errno != 0 {
		return -1, errno
	}
	return int(r1), 0
}
