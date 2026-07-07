package runner

import (
	"fmt"
	"os"

	"github.com/hustoj/runner/sec"
)

const allowUnsafeTestModeEnv = "RUNNER_ALLOW_UNSAFE_TEST_MODE"

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
		if os.Getenv(allowUnsafeTestModeEnv) != "1" {
			return fmt.Errorf(
				"AllowPrivilegedChild=true requires %s=1 to acknowledge the risk of running a root child without isolation",
				allowUnsafeTestModeEnv,
			)
		}
		return nil
	}
	return fmt.Errorf("running as root without dropping to unprivileged RunUID/RunGID requires AllowPrivilegedChild=true")
}
