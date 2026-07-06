package runner

import (
	"fmt"

	"github.com/hustoj/runner/sec"
)

func validateSyscallNames(field string, names []string) error {
	for _, name := range names {
		if len(name) == 0 {
			continue
		}
		if _, err := sec.SCTbl.GetID(name); err != nil {
			return fmt.Errorf("invalid syscall in %s: %q", field, name)
		}
	}
	return nil
}

func validateSyscallBackendForPlatform(_ string) error {
	return nil
}

func validateRuntimeSecurityForPlatform(config *TaskConfig, euid int) error {
	if euid != 0 {
		return nil
	}
	if config.RunUID > 0 && config.RunGID > 0 {
		return nil
	}
	if config.AllowPrivilegedChild {
		return nil
	}
	return fmt.Errorf("running as root without dropping to unprivileged RunUID/RunGID requires AllowPrivilegedChild=true")
}
