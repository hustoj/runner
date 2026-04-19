package runner

// validateSyscallNames is a no-op on non-Linux platforms where the syscall
// table is unavailable.
func validateSyscallNames(_ string, _ []string) error {
	return nil
}
