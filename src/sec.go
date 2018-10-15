package runner

import sec "github.com/seccomp/libseccomp-golang"

func getName(syscallID uint64) string {
	name, _ := sec.ScmpSyscall(syscallID).GetName()
	return name
}