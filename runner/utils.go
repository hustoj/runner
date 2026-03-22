package runner

import (
	"os"
	"time"
)

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

func getWallDuration(from time.Time) int64 {
	return time.Since(from).Nanoseconds()
}
