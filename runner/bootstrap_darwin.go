//go:build darwin

// Darwin stubs: bootstrap requires Linux ptrace. These exist only so
// cmd/runner compiles on macOS for development. They are never called.

package runner

func IsBootstrapProcess() bool {
	return false
}

func BootstrapProcess() {
	panicDarwinDevelopmentOnly("BootstrapProcess")
}
