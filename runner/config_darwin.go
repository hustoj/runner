package runner

import "fmt"

// validateSyscallNames is a no-op on non-Linux platforms where the syscall
// table is unavailable.
func validateSyscallNames(_ string, _ []string) error {
	return nil
}

func validateSyscallBackendForPlatform(backend string) error {
	if backend == syscallBackendHybrid {
		return fmt.Errorf("syscallBackend %q is only supported on linux", backend)
	}
	return nil
}
