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
