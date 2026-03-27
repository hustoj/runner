//go:build !(linux && (amd64 || arm64))

package sec

// syscallEntries returns nil on unsupported platforms.
// SCTable will be empty; GetName/GetID will return errors.
func syscallEntries() map[string]int {
	return nil
}
