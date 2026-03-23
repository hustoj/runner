package runner

import (
	"os"
	"syscall"
)

// DupFileForRead opens filename read-only and redirects file to it.
// The caller is responsible for error handling in the bootstrap path.
func DupFileForRead(filename string, file *os.File) {
	if err := dupFileForRead(filename, file); err != nil {
		panic(err)
	}
}

// DupFileForWrite creates/truncates filename and redirects file to it.
func DupFileForWrite(filename string, file *os.File) {
	if err := dupFileForWrite(filename, file); err != nil {
		panic(err)
	}
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func dupFileForRead(filename string, file *os.File) error {
	fin, err := openFileNoFollow(filename, syscall.O_RDONLY, 0)
	if err != nil {
		if os.IsNotExist(err) {
			// Tests and some local runs rely on missing stdin meaning empty input.
			// Redirect to /dev/null instead of recreating user.in on disk.
			fin, err = os.Open("/dev/null")
		}
		if err != nil {
			return err
		}
	}
	return fileDupErr(fin, file)
}

func dupFileForWrite(filename string, file *os.File) error {
	fout, err := openFileNoFollow(filename, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	return fileDupErr(fout, file)
}
