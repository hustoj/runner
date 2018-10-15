package runner

import "syscall"

func SetTimeLimit(limit int)  {
	rlim := &syscall.Rlimit{}
	rlim.Cur = uint64(limit)
	rlim.Max = rlim.Cur + 1

	syscall.Setrlimit(syscall.RLIMIT_CPU, rlim)
	//todo: add alarm
	//syscall.Syscall(syscall.SIGALRM, limit * 2 + 3)
}

func SetMemoryLimit(limit int)  {
	rlim := &syscall.Rlimit{}
	rlim.Cur = uint64(limit)
	rlim.Max = rlim.Cur + 1

	syscall.Setrlimit(syscall.RLIMIT_STACK, rlim)
	//todo: add alarm
}
