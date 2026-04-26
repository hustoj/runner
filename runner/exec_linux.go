//go:build linux

package runner

import (
	"encoding/binary"
	"fmt"
	"io"
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
	childStageLimitNProc
	childStageLimitOpenFiles
	childStageLimitCore
	childStageDupStdin
	childStageDupStdout
	childStageDupStderr
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
	childStagePtraceTraceme
	childStageExec
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
	case childStageLimitNProc:
		return "set nproc rlimit"
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
	case childStagePtraceTraceme:
		return "ptrace traceme"
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
	cpuLimit     syscall.Rlimit
	outputLimit  syscall.Rlimit
	stackLimit   syscall.Rlimit
	nProcLimit   syscall.Rlimit
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

func (task *RunningTask) runProcess() error {
	controller, err := newMemoryController(task.setting)
	if err != nil {
		return fmt.Errorf("setup memory controller: %w", err)
	}
	defer func() {
		if controller == nil {
			return
		}
		_ = controller.Cleanup()
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

	task.memoryCtrl = controller
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

	timeLimit := uint64(task.setting.CPU)
	stackLimit := uint64(task.setting.Stack) << 20
	enforcedCPULimit := timeLimit + 1
	enforcedOutputLimit := uint64(task.setting.Output) << 20

	return childProcessSpec{
		io:      ioFiles,
		exec:    execSpec,
		sandbox: sandboxSpec,
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
		nProcLimit: syscall.Rlimit{
			Max: uint64(task.setting.MaxProcs),
			Cur: uint64(task.setting.MaxProcs),
		},
		noFileLimit: syscall.Rlimit{
			Max: 16,
			Cur: 16,
		},
		coreLimit: syscall.Rlimit{
			Max: 0,
			Cur: 0,
		},
		alarmSeconds: timeLimit + 5,
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

	stdout, err := openChildIOFile("user.out", syscall.O_WRONLY|syscall.O_CREAT|syscall.O_TRUNC, 0o600)
	if err != nil {
		closeChildIOFiles(ioFiles)
		return childIOFiles{}, err
	}
	ioFiles.stdout = stdout

	stderr, err := openChildIOFile("user.err", syscall.O_WRONLY|syscall.O_CREAT|syscall.O_TRUNC, 0o600)
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
	_, _ = syscall.Wait4(pid, &status, 0, &rusage)
}

func releaseChildCgroupGate(fd int) error {
	defer func() {
		_ = syscall.Close(fd)
	}()
	_, err := syscall.Write(fd, []byte{1})
	return err
}

func runChildProcess(spec childProcessSpec, startupPipeReadFD, startupPipeWriteFD, gatePipeReadFD, gatePipeWriteFD int) {
	rawClose(startupPipeReadFD)
	rawClose(gatePipeWriteFD)

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
	if errno := setResourceLimit(unix.RLIMIT_NPROC, &spec.nProcLimit); errno != 0 {
		reportChildStartupFailure(startupPipeWriteFD, childStageLimitNProc, errno)
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
	if errno := rawSetpgid(0, 0); errno != 0 {
		reportChildStartupFailure(startupPipeWriteFD, childStageSetProcessGroup, errno)
	}

	if failure := applySandboxInChild(spec.sandbox); failure.failed() {
		reportChildStartupFailure(startupPipeWriteFD, failure.stage, failure.errno)
	}
	if errno := ptraceTraceme(); errno != 0 {
		reportChildStartupFailure(startupPipeWriteFD, childStagePtraceTraceme, errno)
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

func errnoFromError(err error, fallback syscall.Errno) syscall.Errno {
	if err == nil {
		return 0
	}
	if errno, ok := err.(syscall.Errno); ok {
		return errno
	}
	return fallback
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

func reportChildStartupFailure(pipeWriteFD int, stage childStartupStage, errno syscall.Errno) {
	var raw [8]byte
	binary.LittleEndian.PutUint32(raw[:4], uint32(stage))
	binary.LittleEndian.PutUint32(raw[4:], uint32(errno))

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
