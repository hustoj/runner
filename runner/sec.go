package runner

import sec "github.com/seccomp/libseccomp-golang"

func getName(syscallID uint64) string {
	name, _ := sec.ScmpSyscall(syscallID).GetName()
	return name
}

func allowSyscall(scs []string) {
	filter, _ := sec.NewFilter(sec.ActTrap)
	for _, sc := range scs {
		id, _ := sec.GetSyscallFromName(sc)
		filter.AddRule(id, sec.ActAllow)
	}
	filter.Load()
}