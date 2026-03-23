//go:build darwin

package runner

func GetProcMemory(_ int) (int64, error) {
	return 0, darwinDevelopmentOnlyError("GetProcMemory")
}
