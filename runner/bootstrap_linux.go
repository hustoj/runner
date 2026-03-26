//go:build linux

package runner

import (
	"fmt"
	"os"
	"syscall"
)

const runnerBootstrapEnv = "RUNNER_BOOTSTRAP"

// IsBootstrapProcess reports whether this process was re-execed as the
// dedicated bootstrap stage.
//
// We intentionally do not use raw fork() and then keep running Go code in the
// child. In Go, raw fork only clones the calling thread, while the runtime may
// still have locks, GC state, and scheduler state owned by other threads.
// Continuing to execute Go code in that half-copied runtime can deadlock or
// crash. Re-execing the current binary gives us a fresh process with a fully
// initialized runtime, and only then do we perform sandbox setup before the
// final execve of the user program.
func IsBootstrapProcess() bool {
	return os.Getenv(runnerBootstrapEnv) == "1"
}

// BootstrapProcess runs the minimal pre-exec setup in a clean helper process.
//
// This stage exists to solve the "raw fork + keep running Go" problem. The
// parent process starts a brand new copy of the runner binary, that copy loads
// configuration, applies limits/sandbox/ptrace, and then immediately execs the
// target program. The result is that all preparatory Go code runs in a normal
// Go process instead of in a post-fork runtime state.
func BootstrapProcess() {
	setting, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap config: %v\n", err)
		syscall.Exit(1)
	}
	if _, err := InitLogger(setting.LogPath, setting.Verbose); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap logger: %v\n", err)
		syscall.Exit(1)
	}

	task := &RunningTask{
		setting:     setting,
		memoryLimit: int64(setting.Memory) * 1024,
	}
	if err := task.execBootstrap(); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap exec failed: %v\n", err)
		syscall.Exit(1)
	}
}

func (task *RunningTask) startBootstrapProcess() (int, error) {
	return StartBootstrapChild(runnerBootstrapEnv)
}

func (task *RunningTask) execBootstrap() error {
	// Order matters and is deliberate:
	// 1. limitResource  — set rlimits while we still have CAP_SYS_RESOURCE
	// 2. redirectIO      — open user.in/out/err before a potential chroot
	//                      makes them unreachable
	// 3. CloseNonStdioFiles — seal off inherited FDs after IO is wired up
	// 4. applySandbox    — namespaces, chroot, no_new_privs, drop privileges
	// 5. ptraceTraceme   — signal the parent tracer we're ready
	// 6. exec            — replace this process with the user program
	if err := task.limitResource(); err != nil {
		return err
	}
	if err := task.redirectIO(); err != nil {
		return err
	}
	if err := CloseNonStdioFiles(); err != nil {
		return err
	}
	if err := applySandbox(task.sandboxConfig()); err != nil {
		return err
	}
	if err := ptraceTraceme(); err != nil {
		return err
	}

	binary, args, err := task.setting.ResolveExec()
	if err != nil {
		return err
	}
	log.Debugf("Command is %s, Args is %v", binary, args)

	// The final exec sees the same minimized environment, but without the
	// bootstrap marker so the user program cannot observe or reuse the internal
	// hand-off flag.
	return syscall.Exec(binary, args, BuildMinimalEnv(runnerBootstrapEnv))
}
