//go:build linux && amd64

package runner

// platformDefaultCalls returns x86_64-specific syscalls that should be
// included in the default allowed list. arch_prctl is needed by Go and
// glibc on x86_64 for thread-local storage setup.
func platformDefaultCalls() []string {
	return []string{"arch_prctl"}
}
