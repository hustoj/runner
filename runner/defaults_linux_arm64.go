//go:build linux && arm64

package runner

// platformDefaultCalls returns ARM64-specific syscalls that should be
// included in the default allowed list.
func platformDefaultCalls() []string {
	return nil
}
