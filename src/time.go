package runner

import (
	"fmt"
	"syscall"
)

func ClacUsage(pid int) int64 {
	usage := &syscall.Rusage{}
	err := syscall.Getrusage(pid, usage)
	if err != nil {
		panic(fmt.Sprintf("get usage failed: %v", err))
	}

	total := usage.Utime.Sec * 1000 + int64(usage.Utime.Usec / 1000)
	total = total + usage.Stime.Sec * 1000 + int64(usage.Utime.Usec / 1000)

	return total
}
