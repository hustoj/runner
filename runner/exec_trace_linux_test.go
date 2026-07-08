//go:build linux && (amd64 || arm64)

package runner

import (
	"errors"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

func TestHandleAttachStopSetsPtraceOptionsAndResumesTracee(t *testing.T) {
	useNopLogger(t)
	restorePtraceHooks := stubPtraceHooks(t)

	const pid = traceHandlerTestPIDBase + 10
	process := NewProcess(pid)
	process.CurrentPid = pid
	task := newTraceHandlerTask(process)

	var setOptionsCalled bool
	restorePtraceHooks.setOptions = func(gotPID int, options int) error {
		setOptionsCalled = true
		assert.Equal(t, pid, gotPID)
		wantOptions := syscall.PTRACE_O_TRACESYSGOOD | ptraceOExitKill | ptraceForkTraceEvents | unix.PTRACE_O_TRACESECCOMP
		assert.Equal(t, wantOptions, options)
		return nil
	}

	var resumed bool
	restorePtraceHooks.cont = func(gotPID int, sig int) error {
		resumed = true
		assert.Equal(t, pid, gotPID)
		assert.Equal(t, 0, sig)
		return nil
	}
	restorePtraceHooks.syscall = func(gotPID int, sig int) error {
		t.Fatalf("PtraceSyscall called with pid=%d sig=%d, want PtraceCont", gotPID, sig)
		return nil
	}

	ctx := traceContext{
		process:      process,
		tracer:       &TracerDetect{},
		traceSeccomp: true,
		resumeMode:   traceResumeEventStops,
	}

	decision := task.handleAttachStop(ctx)

	assert.Equal(t, traceContinue, decision)
	assert.True(t, setOptionsCalled)
	assert.True(t, resumed)
	assert.Equal(t, ACCEPT, task.Result.RetCode)
}

func TestHandleAttachStopAbortsWhenPtraceOptionsFail(t *testing.T) {
	useNopLogger(t)
	restorePtraceHooks := stubPtraceHooks(t)

	process := NewProcess(traceHandlerTestPIDBase + 11)
	process.CurrentPid = traceHandlerTestPIDBase + 11
	task := newTraceHandlerTask(process)

	restorePtraceHooks.setOptions = func(int, int) error {
		return syscall.EIO
	}
	restorePtraceHooks.syscall = func(gotPID int, sig int) error {
		t.Fatalf("resume called with pid=%d sig=%d after ptrace options failure", gotPID, sig)
		return nil
	}
	restorePtraceHooks.cont = restorePtraceHooks.syscall

	decision := task.handleAttachStop(traceContext{
		process: process,
		tracer:  &TracerDetect{},
	})

	assert.Equal(t, traceStop, decision)
	assert.Equal(t, RUNTIME_ERROR, task.Result.RetCode)
	assert.True(t, process.IsKilled)
}

func TestHandleSignalStopForwardsSignal(t *testing.T) {
	useNopLogger(t)
	restorePtraceHooks := stubPtraceHooks(t)

	const pid = traceHandlerTestPIDBase + 12
	process := NewProcess(pid)
	process.CurrentPid = pid
	process.Status = stoppedStatus(syscall.SIGUSR1)
	task := newTraceHandlerTask(process)

	var resumed bool
	restorePtraceHooks.syscall = func(gotPID int, sig int) error {
		resumed = true
		assert.Equal(t, pid, gotPID)
		assert.Equal(t, int(syscall.SIGUSR1), sig)
		return nil
	}
	restorePtraceHooks.cont = func(gotPID int, sig int) error {
		t.Fatalf("PtraceCont called with pid=%d sig=%d, want PtraceSyscall", gotPID, sig)
		return nil
	}

	decision := task.handleSignalOrBrokenStop(traceContext{
		process:    process,
		tracer:     &TracerDetect{},
		resumeMode: traceResumeSyscallStops,
	})

	assert.Equal(t, traceContinue, decision)
	assert.True(t, resumed)
	assert.Equal(t, ACCEPT, task.Result.RetCode)
	assert.False(t, process.IsKilled)
}

func TestHandleSignalStopMarksOutputLimitSignalAndStops(t *testing.T) {
	useNopLogger(t)
	restorePtraceHooks := stubPtraceHooks(t)

	process := NewProcess(traceHandlerTestPIDBase + 13)
	process.CurrentPid = traceHandlerTestPIDBase + 13
	process.Status = stoppedStatus(syscall.SIGXFSZ)
	task := newTraceHandlerTask(process)
	task.setting = &TaskConfig{Output: 0}

	restorePtraceHooks.syscall = func(gotPID int, sig int) error {
		t.Fatalf("resume called with pid=%d sig=%d after output limit signal", gotPID, sig)
		return nil
	}
	restorePtraceHooks.cont = restorePtraceHooks.syscall

	decision := task.handleSignalOrBrokenStop(traceContext{
		process:    process,
		tracer:     &TracerDetect{},
		resumeMode: traceResumeSyscallStops,
	})

	assert.Equal(t, traceStop, decision)
	assert.Equal(t, OUTPUT_LIMIT, task.Result.RetCode)
	assert.True(t, process.IsKilled)
}

func TestHandleSyscallStopAbortsOnPolicyViolation(t *testing.T) {
	useNopLogger(t)
	restorePtraceHooks := stubPtraceHooks(t)

	process := NewProcess(traceHandlerTestPIDBase + 14)
	process.CurrentPid = traceHandlerTestPIDBase + 14
	task := newTraceHandlerTask(process)
	tracer := tracerWithPolicy(t, process.CurrentPid, callPolicySpec{})

	restorePtraceHooks.getRegs = func(pid int, regs *syscall.PtraceRegs) error {
		assert.Equal(t, process.CurrentPid, pid)
		setTestSyscallNumber(regs, uint64(syscall.SYS_GETPID))
		return nil
	}
	restorePtraceHooks.syscall = func(gotPID int, sig int) error {
		t.Fatalf("resume called with pid=%d sig=%d after syscall violation", gotPID, sig)
		return nil
	}

	decision := task.handleSyscallStop(traceContext{
		process:    process,
		tracer:     tracer,
		resumeMode: traceResumeSyscallStops,
	})

	assert.Equal(t, traceStop, decision)
	assert.Equal(t, RUNTIME_ERROR, task.Result.RetCode)
	assert.True(t, process.IsKilled)
}

func TestHandleSyscallStopContinuesWhenTraceeAlreadyGone(t *testing.T) {
	useNopLogger(t)
	restorePtraceHooks := stubPtraceHooks(t)

	process := NewProcess(traceHandlerTestPIDBase + 15)
	process.CurrentPid = traceHandlerTestPIDBase + 15
	task := newTraceHandlerTask(process)
	tracer := &TracerDetect{}

	restorePtraceHooks.syscall = func(gotPID int, sig int) error {
		t.Fatalf("resume called with pid=%d sig=%d for gone tracee", gotPID, sig)
		return nil
	}

	decision := task.handleSyscallStop(traceContext{
		process:    process,
		tracer:     tracer,
		resumeMode: traceResumeSyscallStops,
	})

	assert.Equal(t, traceContinue, decision)
	assert.Equal(t, ACCEPT, task.Result.RetCode)
	assert.False(t, process.IsKilled)
}

func TestHandleSyscallStopAbortsOnTracerError(t *testing.T) {
	useNopLogger(t)
	restorePtraceHooks := stubPtraceHooks(t)

	process := NewProcess(traceHandlerTestPIDBase + 16)
	process.CurrentPid = traceHandlerTestPIDBase + 16
	task := newTraceHandlerTask(process)
	tracer := tracerWithPolicy(t, process.CurrentPid, callPolicySpec{AllowedCalls: []string{"getpid"}})

	restorePtraceHooks.getRegs = func(pid int, regs *syscall.PtraceRegs) error {
		assert.Equal(t, process.CurrentPid, pid)
		return syscall.EIO
	}

	decision := task.handleSyscallStop(traceContext{
		process:    process,
		tracer:     tracer,
		resumeMode: traceResumeSyscallStops,
	})

	assert.Equal(t, traceStop, decision)
	assert.Equal(t, RUNTIME_ERROR, task.Result.RetCode)
	assert.True(t, process.IsKilled)
}

func TestHandleSyscallStopResumesAllowedSyscall(t *testing.T) {
	useNopLogger(t)
	restorePtraceHooks := stubPtraceHooks(t)

	const pid = traceHandlerTestPIDBase + 17
	process := NewProcess(pid)
	process.CurrentPid = pid
	task := newTraceHandlerTask(process)
	tracer := tracerWithPolicy(t, process.CurrentPid, callPolicySpec{AllowedCalls: []string{"getpid"}})

	restorePtraceHooks.getRegs = func(gotPID int, regs *syscall.PtraceRegs) error {
		assert.Equal(t, pid, gotPID)
		setTestSyscallNumber(regs, uint64(syscall.SYS_GETPID))
		return nil
	}

	var resumed bool
	restorePtraceHooks.syscall = func(gotPID int, sig int) error {
		resumed = true
		assert.Equal(t, pid, gotPID)
		assert.Equal(t, 0, sig)
		return nil
	}

	decision := task.handleSyscallStop(traceContext{
		process:    process,
		tracer:     tracer,
		resumeMode: traceResumeSyscallStops,
	})

	assert.Equal(t, traceContinue, decision)
	assert.True(t, resumed)
	assert.Equal(t, ACCEPT, task.Result.RetCode)
	assert.False(t, process.IsKilled)
}

func TestHandleSeccompEventDecisions(t *testing.T) {
	tests := []struct {
		name         string
		tracer       *TracerDetect
		getRegs      func(*syscall.PtraceRegs) error
		wantDecision traceDecision
		wantResult   int
		wantKilled   bool
	}{
		{
			name:         "tracee already gone",
			tracer:       &TracerDetect{},
			wantDecision: traceContinue,
			wantResult:   ACCEPT,
		},
		{
			name:   "policy violation aborts",
			tracer: tracerWithPolicy(t, traceHandlerTestPIDBase+18, callPolicySpec{}),
			getRegs: func(regs *syscall.PtraceRegs) error {
				setTestSyscallNumber(regs, uint64(syscall.SYS_GETPID))
				return nil
			},
			wantDecision: traceStop,
			wantResult:   RUNTIME_ERROR,
			wantKilled:   true,
		},
		{
			name:   "tracer error aborts",
			tracer: tracerWithPolicy(t, traceHandlerTestPIDBase+18, callPolicySpec{AllowedCalls: []string{"getpid"}}),
			getRegs: func(*syscall.PtraceRegs) error {
				return syscall.EIO
			},
			wantDecision: traceStop,
			wantResult:   RUNTIME_ERROR,
			wantKilled:   true,
		},
		{
			name:   "allowed syscall continues",
			tracer: tracerWithPolicy(t, traceHandlerTestPIDBase+18, callPolicySpec{AllowedCalls: []string{"getpid"}}),
			getRegs: func(regs *syscall.PtraceRegs) error {
				setTestSyscallNumber(regs, uint64(syscall.SYS_GETPID))
				return nil
			},
			wantDecision: traceContinue,
			wantResult:   ACCEPT,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useNopLogger(t)
			restorePtraceHooks := stubPtraceHooks(t)

			process := NewProcess(traceHandlerTestPIDBase + 18)
			process.CurrentPid = traceHandlerTestPIDBase + 18
			task := newTraceHandlerTask(process)

			if tt.getRegs != nil {
				restorePtraceHooks.getRegs = func(pid int, regs *syscall.PtraceRegs) error {
					assert.Equal(t, process.CurrentPid, pid)
					return tt.getRegs(regs)
				}
			}

			decision := task.handleSeccompEvent(traceContext{
				process: process,
				tracer:  tt.tracer,
			})

			assert.Equal(t, tt.wantDecision, decision)
			assert.Equal(t, tt.wantResult, task.Result.RetCode)
			assert.Equal(t, tt.wantKilled, process.IsKilled)
		})
	}
}

func TestHandleForkLikePtraceEventRegistersChildTracees(t *testing.T) {
	tests := []struct {
		name            string
		event           int
		childPID        int
		wantThreadGroup bool
	}{
		{
			name:     "clone keeps thread group unresolved until proc sampling",
			event:    ptraceEventClone,
			childPID: traceHandlerTestPIDBase + 20,
		},
		{
			name:            "fork starts a new thread group",
			event:           ptraceEventFork,
			childPID:        traceHandlerTestPIDBase + 21,
			wantThreadGroup: true,
		},
		{
			name:            "vfork starts a new thread group",
			event:           ptraceEventVFork,
			childPID:        traceHandlerTestPIDBase + 22,
			wantThreadGroup: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useNopLogger(t)
			restorePtraceHooks := stubPtraceHooks(t)

			process := NewProcess(traceHandlerTestPIDBase + 19)
			process.CurrentPid = traceHandlerTestPIDBase + 19
			process.Status = ptraceEventStopStatus(tt.event)
			task := newTraceHandlerTask(process)
			tracer := &TracerDetect{}
			tracer.RegisterTracee(process.Pid, false)

			restorePtraceHooks.getEventMsg = func(pid int) (uint, error) {
				assert.Equal(t, process.CurrentPid, pid)
				return uint(tt.childPID), nil
			}

			decision := task.handleForkLikePtraceEvent(traceContext{
				process: process,
				tracer:  tracer,
			})

			assert.Equal(t, traceContinue, decision)
			assert.True(t, process.HasTracee(tt.childPID))
			assert.True(t, tracer.HasTracee(tt.childPID))
			tgid, ok := process.ThreadGroup(tt.childPID)
			assert.Equal(t, tt.wantThreadGroup, ok)
			if tt.wantThreadGroup {
				assert.Equal(t, tt.childPID, tgid)
			}
		})
	}
}

func TestHandleForkLikePtraceEventAbortsWhenEventMessageFails(t *testing.T) {
	useNopLogger(t)
	restorePtraceHooks := stubPtraceHooks(t)

	process := NewProcess(traceHandlerTestPIDBase + 23)
	process.CurrentPid = traceHandlerTestPIDBase + 23
	process.Status = ptraceEventStopStatus(ptraceEventFork)
	task := newTraceHandlerTask(process)
	tracer := &TracerDetect{}
	tracer.RegisterTracee(process.Pid, false)

	restorePtraceHooks.getEventMsg = func(int) (uint, error) {
		return 0, syscall.EIO
	}

	decision := task.handleForkLikePtraceEvent(traceContext{
		process: process,
		tracer:  tracer,
	})

	assert.Equal(t, traceStop, decision)
	assert.Equal(t, RUNTIME_ERROR, task.Result.RetCode)
	assert.True(t, process.IsKilled)
}

func newTraceHandlerTask(process *Process) *RunningTask {
	task := &RunningTask{
		process:     process,
		Result:      &Result{},
		timeLimit:   1_000_000,
		memoryLimit: 1_000_000,
	}
	task.Result.Init()
	return task
}

const traceHandlerTestPIDBase = 1_000_000_000

type ptraceHookStubs struct {
	syscall     func(pid int, sig int) error
	cont        func(pid int, sig int) error
	getRegs     func(pid int, regs *syscall.PtraceRegs) error
	getEventMsg func(pid int) (uint, error)
	setOptions  func(pid int, options int) error
}

func stubPtraceHooks(t *testing.T) *ptraceHookStubs {
	t.Helper()

	originalSyscall := ptraceSyscallCall
	originalCont := ptraceContCall
	originalGetRegs := ptraceGetRegs
	originalGetEventMsg := ptraceGetEventMsgCall
	originalSetOptions := ptraceSetOptionsCall

	stubs := &ptraceHookStubs{
		syscall: func(pid int, sig int) error {
			return errors.New("unexpected PtraceSyscall")
		},
		cont: func(pid int, sig int) error {
			return errors.New("unexpected PtraceCont")
		},
		getRegs: func(pid int, regs *syscall.PtraceRegs) error {
			return errors.New("unexpected PtraceGetRegs")
		},
		getEventMsg: func(pid int) (uint, error) {
			return 0, errors.New("unexpected PtraceGetEventMsg")
		},
		setOptions: func(pid int, options int) error {
			return errors.New("unexpected PtraceSetOptions")
		},
	}

	ptraceSyscallCall = func(pid int, sig int) error {
		return stubs.syscall(pid, sig)
	}
	ptraceContCall = func(pid int, sig int) error {
		return stubs.cont(pid, sig)
	}
	ptraceGetRegs = func(pid int, regs *syscall.PtraceRegs) error {
		return stubs.getRegs(pid, regs)
	}
	ptraceGetEventMsgCall = func(pid int) (uint, error) {
		return stubs.getEventMsg(pid)
	}
	ptraceSetOptionsCall = func(pid int, options int) error {
		return stubs.setOptions(pid, options)
	}

	t.Cleanup(func() {
		ptraceSyscallCall = originalSyscall
		ptraceContCall = originalCont
		ptraceGetRegs = originalGetRegs
		ptraceGetEventMsgCall = originalGetEventMsg
		ptraceSetOptionsCall = originalSetOptions
	})

	return stubs
}

func tracerWithPolicy(t *testing.T, pid int, spec callPolicySpec) *TracerDetect {
	t.Helper()

	policy, err := makeCallPolicy(spec)
	if err != nil {
		t.Fatalf("makeCallPolicy() error = %v", err)
	}
	tracer := &TracerDetect{}
	tracer.RegisterTracee(pid, false)
	tracer.setCallPolicy(policy)
	return tracer
}

func stoppedStatus(sig syscall.Signal) syscall.WaitStatus {
	return syscall.WaitStatus((int(sig) << 8) | 0x7f)
}

func ptraceEventStopStatus(event int) syscall.WaitStatus {
	return syscall.WaitStatus((event << 16) | (int(syscall.SIGTRAP) << 8) | 0x7f)
}

func useNopLogger(t *testing.T) {
	t.Helper()

	previousLog := log
	SetLogger(zap.NewNop().Sugar())
	t.Cleanup(func() {
		SetLogger(previousLog)
	})
}
