//go:build linux

package runner

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

type childStartupStage uint32

const (
	childStageLimitCPU childStartupStage = iota + 1
	childStageAlarm
	childStageLimitFileSize
	childStageLimitStack
	childStageLimitOpenFiles
	childStageLimitCore
	childStageDupStdin
	childStageDupStdout
	childStageDupStderr
	childStageCloseInheritedFiles
	childStageSetProcessGroup
	childStageAwaitCgroupJoin
	childStageSandboxNamespaces
	childStageSandboxInvalidCredentials
	childStageSandboxChroot
	childStageSandboxChdirRoot
	childStageSandboxChdirWorkDir
	childStageSandboxNoNewPrivs
	childStageSandboxSetgroups
	childStageSandboxSetgid
	childStageSandboxSetuid
	childStageDropCapabilities
	childStagePtraceTraceme
	childStagePtraceSync
	childStageSeccomp
	childStageExec
)

const (
	cpuLimitBufferSeconds   uint64 = 1
	outputLimitOverflowByte uint64 = 1
	defaultOpenFileLimit    uint64 = 16
	coreDumpDisabled        uint64 = 0
	alarmBufferSeconds      uint64 = 5
)

type childStartupFailure struct {
	stage childStartupStage
	errno syscall.Errno
}

func (f childStartupFailure) failed() bool {
	return f.stage != 0
}

func (s childStartupStage) String() string {
	switch s {
	case childStageLimitCPU:
		return "set cpu rlimit"
	case childStageAlarm:
		return "set alarm"
	case childStageLimitFileSize:
		return "set file-size rlimit"
	case childStageLimitStack:
		return "set stack rlimit"
	case childStageLimitOpenFiles:
		return "set open-file rlimit"
	case childStageLimitCore:
		return "set core-dump rlimit"
	case childStageDupStdin:
		return "dup stdin"
	case childStageDupStdout:
		return "dup stdout"
	case childStageDupStderr:
		return "dup stderr"
	case childStageCloseInheritedFiles:
		return "close inherited files"
	case childStageSetProcessGroup:
		return "set process group"
	case childStageAwaitCgroupJoin:
		return "await parent cgroup join"
	case childStageSandboxNamespaces:
		return "setup namespaces"
	case childStageSandboxInvalidCredentials:
		return "validate sandbox credentials"
	case childStageSandboxChroot:
		return "chroot"
	case childStageSandboxChdirRoot:
		return "chdir /"
	case childStageSandboxChdirWorkDir:
		return "chdir workdir"
	case childStageSandboxNoNewPrivs:
		return "set no_new_privs"
	case childStageSandboxSetgroups:
		return "clear supplementary groups"
	case childStageSandboxSetgid:
		return "setgid"
	case childStageSandboxSetuid:
		return "setuid"
	case childStageDropCapabilities:
		return "drop capabilities"
	case childStagePtraceTraceme:
		return "ptrace traceme"
	case childStagePtraceSync:
		return "ptrace sync"
	case childStageSeccomp:
		return "install seccomp filter"
	case childStageExec:
		return "execve"
	default:
		return "unknown"
	}
}

type childIOFiles struct {
	stdin  int
	stdout int
	stderr int
}

type childExecSpec struct {
	path *byte
	argv []*byte
	env  []*byte
}

type childProcessSpec struct {
	io           childIOFiles
	exec         childExecSpec
	sandbox      childSandboxSpec
	seccomp      childSeccompSpec
	cpuLimit     syscall.Rlimit
	outputLimit  syscall.Rlimit
	stackLimit   syscall.Rlimit
	noFileLimit  syscall.Rlimit
	coreLimit    syscall.Rlimit
	alarmSeconds uint64
}

func openChildIOFile(path string, flags int, perm uint32) (int, error) {
	return syscall.Open(filepath.Clean(path), flags|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, perm)
}

func ptraceTraceme() syscall.Errno {
	_, _, errno := syscall.RawSyscall(syscall.SYS_PTRACE, syscall.PTRACE_TRACEME, 0, 0)
	return errno
}

func stopForPtraceOptions() syscall.Errno {
	pid, _, errno := syscall.RawSyscall(syscall.SYS_GETPID, 0, 0, 0)
	if errno != 0 {
		return errno
	}
	_, _, errno = syscall.RawSyscall(syscall.SYS_KILL, pid, uintptr(syscall.SIGSTOP), 0)
	return errno
}

var (
	writeChildCgroupGate = syscall.Write
	wait4ChildStartup    = syscall.Wait4
	wait4TraceeStop      = syscall.Wait4
)

func setAlarm(seconds uint64) syscall.Errno {
	timer := unix.Itimerval{
		Value: unix.Timeval{Sec: int64(seconds)},
	}
	_, _, errno := syscall.RawSyscall(
		syscall.SYS_SETITIMER,
		uintptr(unix.ITIMER_REAL),
		uintptr(unsafe.Pointer(&timer)),
		0,
	)
	return errno
}

func (task *RunningTask) runProcess() (err error) {
	controller, err := newTaskController(task.setting)
	if err != nil {
		return fmt.Errorf("setup task cgroup: %w", err)
	}
	defer func() {
		if controller == nil {
			return
		}
		_ = controller.Cleanup()
	}()
	defer func() {
		if err != nil {
			task.closeOutputFile()
		}
	}()

	spec, err := task.prepareChildProcessSpec()
	if err != nil {
		return err
	}

	startupPipeFDs := [2]int{-1, -1}
	if err := syscall.Pipe2(startupPipeFDs[:], syscall.O_CLOEXEC); err != nil {
		closeChildIOFiles(spec.io)
		return fmt.Errorf("create child startup pipe: %w", err)
	}

	gatePipeFDs := [2]int{-1, -1}
	if err := syscall.Pipe2(gatePipeFDs[:], syscall.O_CLOEXEC); err != nil {
		closeChildIOFiles(spec.io)
		closePipeFDs(startupPipeFDs)
		return fmt.Errorf("create child cgroup gate pipe: %w", err)
	}

	pid, errno := fork()
	if errno != 0 || pid < 0 {
		closeChildIOFiles(spec.io)
		closePipeFDs(startupPipeFDs)
		closePipeFDs(gatePipeFDs)
		return fmt.Errorf("fork child: %w", errno)
	}

	if pid == 0 {
		runChildProcess(spec, startupPipeFDs[0], startupPipeFDs[1], gatePipeFDs[0], gatePipeFDs[1])
	}

	task.outputFileFD = spec.io.stdout
	task.hasOutputFileFD = true
	spec.io.stdout = -1
	closeChildIOFiles(spec.io)
	_ = syscall.Close(startupPipeFDs[1])
	_ = syscall.Close(gatePipeFDs[0])

	if err := controller.MovePID(pid); err != nil {
		_ = syscall.Close(startupPipeFDs[0])
		_ = syscall.Close(gatePipeFDs[1])
		waitChildStartupFailure(pid)
		return fmt.Errorf("move child %d into task cgroup: %w", pid, err)
	}
	if err := releaseChildCgroupGate(gatePipeFDs[1]); err != nil {
		_ = syscall.Close(startupPipeFDs[0])
		waitChildStartupFailure(pid)
		return fmt.Errorf("release child cgroup gate: %w", err)
	}
	// This hands the child's ptrace state to prepareHybridSeccompTracee.
	// See its doc comment: nothing between releaseChildCgroupGate and this
	// call may wait4/ptraceCont/ptraceSetOptions the child, or the first
	// group-stop it waits for will already be consumed.
	if shouldSyncSeccompTracee(spec.seccomp) {
		if err := prepareHybridSeccompTracee(pid); err != nil {
			_ = syscall.Close(startupPipeFDs[0])
			killChildAfterStartupError(pid)
			return fmt.Errorf("prepare hybrid seccomp tracee: %w", err)
		}
	}

	failure, err := readChildStartupFailure(startupPipeFDs[0])
	_ = syscall.Close(startupPipeFDs[0])
	if err != nil {
		waitChildStartupFailure(pid)
		return fmt.Errorf("read child startup pipe: %w", err)
	}
	if failure.failed() {
		waitChildStartupFailure(pid)
		return fmt.Errorf("child startup failed at %s: %w", failure.stage, failure.errno)
	}

	task.taskCtrl = controller
	controller = nil
	task.process = NewProcess(pid)
	log.Debugf("child pid is %d", pid)
	return nil
}

func (task *RunningTask) prepareChildProcessSpec() (childProcessSpec, error) {
	ioFiles, err := openChildIOFiles()
	if err != nil {
		return childProcessSpec{}, err
	}

	execSpec, err := prepareChildExecSpec(task.setting)
	if err != nil {
		closeChildIOFiles(ioFiles)
		return childProcessSpec{}, err
	}

	sandboxSpec, err := prepareChildSandboxSpec(task.sandboxConfig())
	if err != nil {
		closeChildIOFiles(ioFiles)
		return childProcessSpec{}, err
	}
	seccompSpec, err := prepareChildSeccompSpec(task.setting)
	if err != nil {
		closeChildIOFiles(ioFiles)
		return childProcessSpec{}, err
	}

	cpuTimeLimit := uint64(task.setting.CPU)
	wallClockLimit := uint64(task.setting.effectiveWallClockLimitSeconds())
	stackLimit := uint64(task.setting.Stack) << 20
	enforcedCPULimit := cpuTimeLimit + cpuLimitBufferSeconds
	configuredOutputLimit := uint64(task.setting.Output) << 20
	enforcedOutputLimit := configuredOutputLimit
	if enforcedOutputLimit < math.MaxUint64 {
		enforcedOutputLimit += outputLimitOverflowByte
	}

	return childProcessSpec{
		io:      ioFiles,
		exec:    execSpec,
		sandbox: sandboxSpec,
		seccomp: seccompSpec,
		cpuLimit: syscall.Rlimit{
			Max: enforcedCPULimit,
			Cur: enforcedCPULimit,
		},
		outputLimit: syscall.Rlimit{
			Max: enforcedOutputLimit,
			Cur: enforcedOutputLimit,
		},
		stackLimit: syscall.Rlimit{
			Max: stackLimit,
			Cur: stackLimit,
		},
		noFileLimit: syscall.Rlimit{
			Max: defaultOpenFileLimit,
			Cur: defaultOpenFileLimit,
		},
		coreLimit: syscall.Rlimit{
			Max: coreDumpDisabled,
			Cur: coreDumpDisabled,
		},
		alarmSeconds: wallClockLimit + alarmBufferSeconds,
	}, nil
}

func prepareChildExecSpec(setting *TaskConfig) (childExecSpec, error) {
	binary, args, err := setting.ResolveExec()
	if err != nil {
		return childExecSpec{}, err
	}

	path, err := syscall.BytePtrFromString(binary)
	if err != nil {
		return childExecSpec{}, err
	}

	argv, err := syscall.SlicePtrFromStrings(args)
	if err != nil {
		return childExecSpec{}, err
	}

	env, err := syscall.SlicePtrFromStrings(BuildMinimalEnv())
	if err != nil {
		return childExecSpec{}, err
	}

	return childExecSpec{
		path: path,
		argv: argv,
		env:  env,
	}, nil
}

func openChildIOFiles() (childIOFiles, error) {
	ioFiles := childIOFiles{stdin: -1, stdout: -1, stderr: -1}

	stdin, err := openChildIOFile("user.in", syscall.O_RDONLY, 0)
	if err != nil {
		closeChildIOFiles(ioFiles)
		return childIOFiles{}, err
	}
	ioFiles.stdin = stdin

	stdout, err := openChildIOFile("user.out", syscall.O_WRONLY|syscall.O_CREAT|syscall.O_TRUNC, outputFilePerm)
	if err != nil {
		closeChildIOFiles(ioFiles)
		return childIOFiles{}, err
	}
	ioFiles.stdout = stdout

	stderr, err := openChildIOFile("user.err", syscall.O_WRONLY|syscall.O_CREAT|syscall.O_TRUNC, outputFilePerm)
	if err != nil {
		closeChildIOFiles(ioFiles)
		return childIOFiles{}, err
	}
	ioFiles.stderr = stderr

	return ioFiles, nil
}

func closeChildIOFiles(ioFiles childIOFiles) {
	for _, fd := range []int{ioFiles.stdin, ioFiles.stdout, ioFiles.stderr} {
		if fd >= 0 {
			_ = syscall.Close(fd)
		}
	}
}

func closePipeFDs(pipeFDs [2]int) {
	for _, fd := range pipeFDs {
		if fd >= 0 {
			_ = syscall.Close(fd)
		}
	}
}

// readChildStartupFailure reads the fixed 8-byte child startup failure struct
// from the pipe. The EINTR retry is hand-written on purpose: retryOnEINTR only
// surfaces an error and cannot express "partial read succeeded, keep going until
// all 8 bytes are read", nor the n==0 EOF branching below. So the unified helper
// does not fit this call site.
func readChildStartupFailure(fd int) (childStartupFailure, error) {
	var raw [8]byte
	read := 0

	for read < len(raw) {
		n, err := syscall.Read(fd, raw[read:])
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			return childStartupFailure{}, err
		}
		if n == 0 {
			if read == 0 {
				return childStartupFailure{}, nil
			}
			return childStartupFailure{}, io.ErrUnexpectedEOF
		}
		read += n
	}

	return childStartupFailure{
		stage: childStartupStage(binary.LittleEndian.Uint32(raw[:4])),
		errno: syscall.Errno(binary.LittleEndian.Uint32(raw[4:])),
	}, nil
}

func waitChildStartupFailure(pid int) {
	var status syscall.WaitStatus
	var rusage syscall.Rusage
	_ = retryOnEINTR(func() error {
		_, err := wait4ChildStartup(pid, &status, 0, &rusage)
		return err
	})
}

func shouldSyncSeccompTracee(spec childSeccompSpec) bool {
	return spec.enabled && len(spec.traceSyscalls) > 0
}

// prepareHybridSeccompTracee drives the hybrid seccomp handshake: wait for the
// child's self-SIGSTOP group-stop, arm PTRACE_O_TRACESECCOMP, then wait for the
// execve-triggered seccomp event. It owns the child's ptrace state for the
// duration -- the tracee must arrive at the first group-stop unconsumed, and no
// wait4/ptraceCont/ptraceSetOptions may run against the child between
// releaseChildCgroupGate and this call, nor between its two wait/continue cycles.
func prepareHybridSeccompTracee(pid int) error {
	status, err := waitForTraceeStop(pid)
	if err != nil {
		return fmt.Errorf("wait ptrace sync stop: %w", err)
	}
	if !status.Stopped() || status.StopSignal() != syscall.SIGSTOP {
		return fmt.Errorf("unexpected hybrid ptrace sync status: %s", describeWaitStatus(status))
	}
	if err := setPtraceOptions(pid, true); err != nil {
		return fmt.Errorf("set ptrace options before seccomp install: %w", err)
	}
	if err := ptraceCont(pid, 0); err != nil {
		return fmt.Errorf("continue child to seccomp execve event: %w", err)
	}

	status, err = waitForTraceeStop(pid)
	if err != nil {
		return fmt.Errorf("wait seccomp execve event: %w", err)
	}
	if !isPtraceEvent(status, ptraceEventSeccomp) {
		return fmt.Errorf("unexpected hybrid seccomp event status: %s", describeWaitStatus(status))
	}
	if err := verifyStartupSeccompEvent(pid); err != nil {
		return err
	}
	if err := ptraceCont(pid, 0); err != nil {
		return fmt.Errorf("continue child after seccomp execve event: %w", err)
	}
	return nil
}

func waitForTraceeStop(pid int) (syscall.WaitStatus, error) {
	var status syscall.WaitStatus
	var rusage syscall.Rusage
	var waitedPID int
	if err := retryOnEINTR(func() error {
		var err error
		waitedPID, err = wait4TraceeStop(pid, &status, 0, &rusage)
		return err
	}); err != nil {
		return 0, err
	}
	if waitedPID != pid {
		return 0, fmt.Errorf("wait4 returned pid %d, want %d", waitedPID, pid)
	}
	return status, nil
}

func isPtraceEvent(status syscall.WaitStatus, event int) bool {
	return status.Stopped() && status.StopSignal() == syscall.SIGTRAP && status.TrapCause() == event
}

func verifyStartupSeccompEvent(pid int) error {
	var regs syscall.PtraceRegs
	if err := ptraceGetRegs(pid, &regs); err != nil {
		return fmt.Errorf("read regs for startup seccomp event: %w", err)
	}
	callID := getSyscallNumber(&regs)
	if callID != uint64(syscall.SYS_EXECVE) {
		return fmt.Errorf("unexpected startup seccomp syscall %s(%d), want execve", getName(callID), callID)
	}
	return nil
}

func describeWaitStatus(status syscall.WaitStatus) string {
	switch {
	case status.Stopped():
		return fmt.Sprintf("stopped signal=%v trap=%d", status.StopSignal(), status.TrapCause())
	case status.Exited():
		return fmt.Sprintf("exited code=%d", status.ExitStatus())
	case status.Signaled():
		return fmt.Sprintf("signaled signal=%v", status.Signal())
	default:
		return fmt.Sprintf("raw=%#x", int(status))
	}
}

func killChildAfterStartupError(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = syscall.Kill(pid, syscall.SIGKILL)
	waitChildStartupFailure(pid)
}

func releaseChildCgroupGate(fd int) error {
	defer func() {
		_ = syscall.Close(fd)
	}()
	var n int
	if err := retryOnEINTR(func() error {
		var err error
		n, err = writeChildCgroupGate(fd, []byte{1})
		return err
	}); err != nil {
		return err
	}
	if n != 1 {
		return io.ErrShortWrite
	}
	return nil
}

func runChildProcess(spec childProcessSpec, startupPipeReadFD, startupPipeWriteFD, gatePipeReadFD, gatePipeWriteFD int) {
	// After raw fork, this path must stay limited to fixed control flow over
	// precomputed data plus raw syscalls. Do not add logging, formatting,
	// os package calls, panics, allocation-heavy helpers, or runtime-dependent
	// syscall wrappers here.
	rawClose(startupPipeReadFD)
	rawClose(gatePipeWriteFD)

	// The parent moves this pid into the task cgroup before releasing the gate,
	// so process/thread limits cover the program before it reaches execve.
	if errno := awaitParentCgroupJoin(gatePipeReadFD); errno != 0 {
		reportChildStartupFailure(startupPipeWriteFD, childStageAwaitCgroupJoin, errno)
	}
	rawClose(gatePipeReadFD)

	if errno := setResourceLimit(syscall.RLIMIT_CPU, &spec.cpuLimit); errno != 0 {
		reportChildStartupFailure(startupPipeWriteFD, childStageLimitCPU, errno)
	}
	if errno := setAlarm(spec.alarmSeconds); errno != 0 {
		reportChildStartupFailure(startupPipeWriteFD, childStageAlarm, errno)
	}
	if errno := setResourceLimit(syscall.RLIMIT_FSIZE, &spec.outputLimit); errno != 0 {
		reportChildStartupFailure(startupPipeWriteFD, childStageLimitFileSize, errno)
	}
	if errno := setResourceLimit(syscall.RLIMIT_STACK, &spec.stackLimit); errno != 0 {
		reportChildStartupFailure(startupPipeWriteFD, childStageLimitStack, errno)
	}
	if errno := setResourceLimit(syscall.RLIMIT_NOFILE, &spec.noFileLimit); errno != 0 {
		reportChildStartupFailure(startupPipeWriteFD, childStageLimitOpenFiles, errno)
	}
	if errno := setResourceLimit(syscall.RLIMIT_CORE, &spec.coreLimit); errno != 0 {
		reportChildStartupFailure(startupPipeWriteFD, childStageLimitCore, errno)
	}
	if errno := dupToStandardFD(spec.io.stdin, syscall.Stdin); errno != 0 {
		reportChildStartupFailure(startupPipeWriteFD, childStageDupStdin, errno)
	}
	if errno := dupToStandardFD(spec.io.stdout, syscall.Stdout); errno != 0 {
		reportChildStartupFailure(startupPipeWriteFD, childStageDupStdout, errno)
	}
	if errno := dupToStandardFD(spec.io.stderr, syscall.Stderr); errno != 0 {
		reportChildStartupFailure(startupPipeWriteFD, childStageDupStderr, errno)
	}
	closedStartupPipeWriteFD, closeErrno := closeNonStdioFilesExceptStartupPipe(startupPipeWriteFD)
	startupPipeWriteFD = closedStartupPipeWriteFD
	if closeErrno != 0 {
		reportChildStartupFailure(startupPipeWriteFD, childStageCloseInheritedFiles, closeErrno)
	}
	if errno := rawSetpgid(0, 0); errno != 0 {
		reportChildStartupFailure(startupPipeWriteFD, childStageSetProcessGroup, errno)
	}

	if failure := applySandboxInChild(spec.sandbox); failure.failed() {
		reportChildStartupFailure(startupPipeWriteFD, failure.stage, failure.errno)
	}
	if errno := dropAllCapabilities(); errno != 0 {
		reportChildStartupFailure(startupPipeWriteFD, childStageDropCapabilities, errno)
	}
	if errno := ptraceTraceme(); errno != 0 {
		reportChildStartupFailure(startupPipeWriteFD, childStagePtraceTraceme, errno)
	}
	if shouldSyncSeccompTracee(spec.seccomp) {
		if errno := stopForPtraceOptions(); errno != 0 {
			reportChildStartupFailure(startupPipeWriteFD, childStagePtraceSync, errno)
		}
	}
	// Install after PTRACE_TRACEME so runtime policy does not need to allow ptrace.
	if errno := installSeccompFilter(spec.seccomp); errno != 0 {
		reportChildStartupFailure(startupPipeWriteFD, childStageSeccomp, errno)
	}

	var argvPtr uintptr
	if len(spec.exec.argv) > 0 {
		argvPtr = uintptr(unsafe.Pointer(&spec.exec.argv[0]))
	}
	var envPtr uintptr
	if len(spec.exec.env) > 0 {
		envPtr = uintptr(unsafe.Pointer(&spec.exec.env[0]))
	}

	_, _, errno := syscall.RawSyscall(
		syscall.SYS_EXECVE,
		uintptr(unsafe.Pointer(spec.exec.path)),
		argvPtr,
		envPtr,
	)
	reportChildStartupFailure(startupPipeWriteFD, childStageExec, errno)
}

func setResourceLimit(code int, limit *syscall.Rlimit) syscall.Errno {
	return rawSetrlimit(code, limit)
}

func dupToStandardFD(sourceFD int, targetFD int) syscall.Errno {
	if sourceFD == targetFD {
		return 0
	}
	_, _, errno := syscall.RawSyscall(syscall.SYS_DUP3, uintptr(sourceFD), uintptr(targetFD), 0)
	if errno != 0 {
		return errno
	}
	rawClose(sourceFD)
	return 0
}

// childStartupReportFD is the fixed fd the child moves the startup pipe to
// before closing all other inherited fds via close_range. Chosen as 3 (the
// lowest fd after stdin/stdout/stderr) so dup3 + close_range(4..) is safe.
const childStartupReportFD = 3

func closeNonStdioFilesExceptStartupPipe(startupPipeWriteFD int) (int, syscall.Errno) {
	if startupPipeWriteFD != childStartupReportFD {
		_, _, errno := syscall.RawSyscall(
			syscall.SYS_DUP3,
			uintptr(startupPipeWriteFD),
			uintptr(childStartupReportFD),
			uintptr(syscall.O_CLOEXEC),
		)
		if errno != 0 {
			return startupPipeWriteFD, errno
		}
		rawClose(startupPipeWriteFD)
		startupPipeWriteFD = childStartupReportFD
	}

	if errno := rawCloseRange(childStartupReportFD + 1); errno != 0 {
		return startupPipeWriteFD, errno
	}
	return startupPipeWriteFD, 0
}

func rawCloseRange(firstFD int) syscall.Errno {
	_, _, errno := syscall.RawSyscall(unix.SYS_CLOSE_RANGE, uintptr(firstFD), ^uintptr(0), 0)
	return errno
}

func rawClose(fd int) {
	if fd < 0 {
		return
	}
	_, _, _ = syscall.RawSyscall(syscall.SYS_CLOSE, uintptr(fd), 0, 0)
}

func awaitParentCgroupJoin(fd int) syscall.Errno {
	var gate [1]byte
	for {
		n, _, errno := syscall.RawSyscall(
			syscall.SYS_READ,
			uintptr(fd),
			uintptr(unsafe.Pointer(&gate[0])),
			uintptr(len(gate)),
		)
		if errno == syscall.EINTR {
			continue
		}
		if errno != 0 {
			return errno
		}
		if n == 0 {
			return syscall.EPIPE
		}
		if gate[0] == 1 {
			return 0
		}
		return syscall.EINVAL
	}
}

func rawSetrlimit(resource int, limit *syscall.Rlimit) syscall.Errno {
	_, _, errno := syscall.RawSyscall6(
		syscall.SYS_PRLIMIT64,
		0,
		uintptr(resource),
		uintptr(unsafe.Pointer(limit)),
		0,
		0,
		0,
	)
	return errno
}

func rawSetpgid(pid int, pgid int) syscall.Errno {
	_, _, errno := syscall.RawSyscall(syscall.SYS_SETPGID, uintptr(pid), uintptr(pgid), 0)
	return errno
}

// capabilityCount is the upper bound of capability indices representable in the
// capset ABI (two u32 words = 64 bits). Linux capability numbers are currently
// well below this bound; iterating up to 64 with EINVAL-tolerance covers future
// kernels without an ABI bump.
const capabilityCount = 64

// dropAllCapabilities clears every capability from the bounding set, then
// clears the effective, permitted, and inheritable sets. It is a no-op for
// non-root children because setuid has already cleared effective/permitted and
// non-root cannot drop bounding entries. For root children (the
// AllowPrivilegedChild path) it provides defense-in-depth before execve: the
// child retains no privileges and, combined with NoNewPrivs, cannot regain any
// via setuid binaries or file capabilities.
//
// This runs after applySandboxInChild so that sandbox steps which require
// capabilities (chroot, unshare, setgid, setuid) still see them.
func dropAllCapabilities() syscall.Errno {
	if rawGeteuid() != 0 {
		return 0
	}
	// Drop all capabilities from the bounding set first. This requires
	// CAP_SETPCAP in the effective set; root retains it until we clear
	// effective/permitted below. PR_CAPBSET_DROP only touches the bounding
	// set, so CAP_SETPCAP stays effective across these calls.
	for capability := 0; capability < capabilityCount; capability++ {
		errno := rawPrctl(unix.PR_CAPBSET_DROP, capability, 0, 0, 0)
		if errno == syscall.EINVAL {
			// Capability number not supported on this kernel.
			continue
		}
		if errno != 0 {
			return errno
		}
	}
	// Clear effective, permitted, and inheritable capability sets.
	header := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	data := [2]unix.CapUserData{}
	return rawCapset(&header, &data[0])
}

func rawCapset(header *unix.CapUserHeader, data *unix.CapUserData) syscall.Errno {
	_, _, errno := syscall.RawSyscall(
		syscall.SYS_CAPSET,
		uintptr(unsafe.Pointer(header)),
		uintptr(unsafe.Pointer(data)),
		0,
	)
	return errno
}

func rawGeteuid() uintptr {
	uid, _, _ := syscall.RawSyscall(syscall.SYS_GETEUID, 0, 0, 0)
	return uid
}

func rawPrctl(option int, arg2 int, arg3 int, arg4 int, arg5 int) syscall.Errno {
	_, _, errno := syscall.RawSyscall6(
		syscall.SYS_PRCTL,
		uintptr(option),
		uintptr(arg2),
		uintptr(arg3),
		uintptr(arg4),
		uintptr(arg5),
		0,
	)
	return errno
}

func reportChildStartupFailure(pipeWriteFD int, stage childStartupStage, errno syscall.Errno) {
	var raw [8]byte
	raw[0] = byte(stage)
	raw[1] = byte(stage >> 8)
	raw[2] = byte(stage >> 16)
	raw[3] = byte(stage >> 24)
	raw[4] = byte(errno)
	raw[5] = byte(errno >> 8)
	raw[6] = byte(errno >> 16)
	raw[7] = byte(errno >> 24)

	_, _, _ = syscall.RawSyscall(
		syscall.SYS_WRITE,
		uintptr(pipeWriteFD),
		uintptr(unsafe.Pointer(&raw[0])),
		uintptr(len(raw)),
	)
	rawClose(pipeWriteFD)
	_, _, _ = syscall.RawSyscall(syscall.SYS_EXIT, 127, 0, 0)
	for {
	}
}
