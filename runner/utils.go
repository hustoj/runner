package runner

import (
	"os"
	"syscall"
	"time"
)

func fileDup(f1 *os.File, f2 *os.File) {
	syscall.Dup2(int(f1.Fd()), int(f2.Fd()))
	f1.Close()
}

func DupFileForRead(filename string, file *os.File) {
	fin, err := os.OpenFile(filename, os.O_RDONLY|os.O_CREATE, 0666)
	checkErr(err)
	fileDup(fin, file)
}

func DupFileForWrite(filename string, file *os.File) {
	fout, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	checkErr(err)
	fileDup(fout, file)
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func fork() int {
	r1, _, errno := syscall.Syscall(syscall.SYS_FORK, 0, 0, 0)
	if errno != 0 || r1 < 0{
		log.Panic("fork failed", errno)
	}
	return int(r1)
}

func getWallDuration(from time.Time) int64 {
	return int64(time.Now().Nanosecond() - from.Nanosecond())
}

func ChangeRunningUser(user int) {
	err := syscall.Setuid(user)
	if err != nil {
		log.Panicf("set running uid failed %v\n", err)
	}
}
