//go:build linux

package runner

import (
	"encoding/binary"
	"io"
	"os"
	"syscall"
	"unsafe"
)

type childStartupStage uint32

const (
	childStageLimitCPU childStartupStage = iota + 1
	childStageAlarm
	childStageLimitFileSize
	childStageLimitStack
	childStageLimitData
	childStageLimitAddressSpace
	childStageDupStdin
	childStageDupStdout
	childStageDupStderr
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
	case childStageLimitData:
		return "set data rlimit"
	case childStageLimitAddressSpace:
		return "set address-space rlimit"
	case childStageDupStdin:
		return "dup stdin"
	case childStageDupStdout:
		return "dup stdout"
	case childStageDupStderr:
		return "dup stderr"
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
	memoryLimit  syscall.Rlimit
	alarmSeconds uint64
}

func ptraceTraceme() syscall.Errno {
	_, _, errno := syscall.RawSyscall(syscall.SYS_PTRACE, syscall.PTRACE_TRACEME, 0, 0)
	return errno
}

func setAlarm(seconds uint64) syscall.Errno {
	_, _, errno := syscall.RawSyscall(syscall.SYS_ALARM, uintptr(seconds), 0, 0)
	return errno
}

func (task *RunningTask) runProcess() bool {
	spec, err := task.prepareChildProcessSpec()
	if err != nil {
		log.Infof("prepare child process failed: %v", err)
		task.Result.RetCode = RUNTIME_ERROR
		return false
	}

	pipeFDs := [2]int{-1, -1}
	if err := syscall.Pipe2(pipeFDs[:], syscall.O_CLOEXEC); err != nil {
		closeChildIOFiles(spec.io)
		log.Panicf("create child startup pipe failed: %v", err)
	}

	pid, errno := fork()
	if errno != 0 || pid < 0 {
		closeChildIOFiles(spec.io)
		closePipeFDs(pipeFDs)
		log.Panicf("fork child failed: %v", errno)
	}

	if pid == 0 {
		runChildProcess(spec, pipeFDs[0], pipeFDs[1])
	}

	closeChildIOFiles(spec.io)
	_ = syscall.Close(pipeFDs[1])

	failure, err := readChildStartupFailure(pipeFDs[0])
	_ = syscall.Close(pipeFDs[0])
	if err != nil {
		log.Infof("read child startup pipe failed: %v", err)
		task.Result.RetCode = RUNTIME_ERROR
		waitChildStartupFailure(pid)
		return false
	}
	if failure.failed() {
		log.Infof("child startup failed at %s: %v", failure.stage, failure.errno)
		task.Result.RetCode = RUNTIME_ERROR
		waitChildStartupFailure(pid)
		return false
	}

	task.process = &Process{Pid: pid}
	log.Debugf("child pid is %d", pid)
	return true
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
	memoryLimit := uint64(task.memoryLimit<<10) * 4

	return childProcessSpec{
		io:      ioFiles,
		exec:    execSpec,
		sandbox: sandboxSpec,
		cpuLimit: syscall.Rlimit{
			Max: timeLimit + 3,
			Cur: timeLimit + 1,
		},
		outputLimit: syscall.Rlimit{
			Max: uint64(task.setting.Output << 21),
			Cur: uint64(task.setting.Output << 20),
		},
		memoryLimit: syscall.Rlimit{
			Max: memoryLimit + 5<<20,
			Cur: memoryLimit,
		},
		alarmSeconds: timeLimit + 5,
	}, nil
}

func prepareChildExecSpec(setting *TaskConfig) (childExecSpec, error) {
	path, err := syscall.BytePtrFromString(setting.GetCommand())
	if err != nil {
		return childExecSpec{}, err
	}

	argv, err := syscall.SlicePtrFromStrings(setting.GetArgs())
	if err != nil {
		return childExecSpec{}, err
	}

	env, err := syscall.SlicePtrFromStrings(os.Environ())
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

	stdin, err := syscall.Open("user.in", syscall.O_RDONLY|syscall.O_CREAT, 0666)
	if err != nil {
		closeChildIOFiles(ioFiles)
		return childIOFiles{}, err
	}
	ioFiles.stdin = stdin

	stdout, err := syscall.Open("user.out", syscall.O_WRONLY|syscall.O_CREAT|syscall.O_TRUNC, 0666)
	if err != nil {
		closeChildIOFiles(ioFiles)
		return childIOFiles{}, err
	}
	ioFiles.stdout = stdout

	stderr, err := syscall.Open("user.err", syscall.O_WRONLY|syscall.O_CREAT|syscall.O_TRUNC, 0666)
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

func runChildProcess(spec childProcessSpec, pipeReadFD, pipeWriteFD int) {
	rawClose(pipeReadFD)

	if errno := setResourceLimit(syscall.RLIMIT_CPU, &spec.cpuLimit); errno != 0 {
		reportChildStartupFailure(pipeWriteFD, childStageLimitCPU, errno)
	}
	if errno := setAlarm(spec.alarmSeconds); errno != 0 {
		reportChildStartupFailure(pipeWriteFD, childStageAlarm, errno)
	}
	if errno := setResourceLimit(syscall.RLIMIT_FSIZE, &spec.outputLimit); errno != 0 {
		reportChildStartupFailure(pipeWriteFD, childStageLimitFileSize, errno)
	}
	if errno := setResourceLimit(syscall.RLIMIT_STACK, &spec.memoryLimit); errno != 0 {
		reportChildStartupFailure(pipeWriteFD, childStageLimitStack, errno)
	}
	if errno := setResourceLimit(syscall.RLIMIT_DATA, &spec.memoryLimit); errno != 0 {
		reportChildStartupFailure(pipeWriteFD, childStageLimitData, errno)
	}
	if errno := setResourceLimit(syscall.RLIMIT_AS, &spec.memoryLimit); errno != 0 {
		reportChildStartupFailure(pipeWriteFD, childStageLimitAddressSpace, errno)
	}
	if errno := dupToStandardFD(spec.io.stdin, syscall.Stdin); errno != 0 {
		reportChildStartupFailure(pipeWriteFD, childStageDupStdin, errno)
	}
	if errno := dupToStandardFD(spec.io.stdout, syscall.Stdout); errno != 0 {
		reportChildStartupFailure(pipeWriteFD, childStageDupStdout, errno)
	}
	if errno := dupToStandardFD(spec.io.stderr, syscall.Stderr); errno != 0 {
		reportChildStartupFailure(pipeWriteFD, childStageDupStderr, errno)
	}

	if failure := applySandboxInChild(spec.sandbox); failure.failed() {
		reportChildStartupFailure(pipeWriteFD, failure.stage, failure.errno)
	}
	if errno := ptraceTraceme(); errno != 0 {
		reportChildStartupFailure(pipeWriteFD, childStagePtraceTraceme, errno)
	}

	_, _, errno := syscall.RawSyscall(
		syscall.SYS_EXECVE,
		uintptr(unsafe.Pointer(spec.exec.path)),
		uintptr(unsafe.Pointer(&spec.exec.argv[0])),
		uintptr(unsafe.Pointer(&spec.exec.env[0])),
	)
	reportChildStartupFailure(pipeWriteFD, childStageExec, errno)
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
	_, _, errno := syscall.RawSyscall(syscall.SYS_DUP2, uintptr(sourceFD), uintptr(targetFD), 0)
	if errno != 0 {
		return errno
	}
	if sourceFD != targetFD {
		rawClose(sourceFD)
	}
	return 0
}

func rawClose(fd int) {
	if fd < 0 {
		return
	}
	_, _, _ = syscall.RawSyscall(syscall.SYS_CLOSE, uintptr(fd), 0, 0)
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
