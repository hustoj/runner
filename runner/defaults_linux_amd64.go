//go:build linux && amd64

package runner

func platformDefaultCalls() []string {
	return []string{"arch_prctl", "readlink", "access"}
}
