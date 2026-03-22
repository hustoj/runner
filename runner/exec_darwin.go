//go:build darwin

package runner

func ptraceTraceme() {
	// Darwin has ptrace, but the syscall number and usage can differ.
	// Providing a stub for now as direct ptrace isn't the focus of this compilation fix.
}

func setAlarm(seconds uint64) {
	// Darwin doesn't have SYS_ALARM in the same way.
}
