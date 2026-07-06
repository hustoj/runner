//go:build linux && (amd64 || arm64)

package runner

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestGetNameReturnsFallbackForUnknownSyscall(t *testing.T) {
	assert.Equal(t, "unknown_9999", getName(9999))
}

func TestConsumeBootstrapCallConsumesExecveQuota(t *testing.T) {
	tracer := &TracerDetect{}

	policy, err := makeCallPolicy(callPolicySpec{OneTimeCalls: []string{"execve"}})
	assert.NoError(t, err)
	tracer.setCallPolicy(policy)
	tracer.consumeBootstrapCall(syscall.SYS_EXECVE)

	assert.False(t, tracer.callPolicy.CheckID(uint64(syscall.SYS_EXECVE)))
}

func TestTracerDetectTracksTraceeStatePerPid(t *testing.T) {
	tracer := &TracerDetect{}
	tracer.RegisterTracee(100, false)
	tracer.RegisterTracee(101, true)

	assert.True(t, tracer.HasTracee(100))
	assert.True(t, tracer.ConsumeAttachStop(101, syscall.WaitStatus((int(syscall.SIGSTOP)<<8)|0x7f)))
	assert.False(t, tracer.ConsumeAttachStop(101, syscall.WaitStatus((int(syscall.SIGSTOP)<<8)|0x7f)))
	assert.False(t, tracer.ConsumeAttachStop(100, syscall.WaitStatus((int(syscall.SIGSTOP)<<8)|0x7f)))
	assert.False(t, tracer.ConsumeAttachStop(102, syscall.WaitStatus((int(syscall.SIGSTOP)<<8)|0x7f)))

	tracer.RemoveTracee(101)
	assert.False(t, tracer.HasTracee(101))
}

func TestCheckSyscallTreatsUnknownPidAsGone(t *testing.T) {
	tracer := &TracerDetect{}

	assert.Equal(t, syscallCheckTraceeGone, tracer.checkSyscall(1234))
}

func TestCheckSyscallTreatsESRCHAsTraceeGone(t *testing.T) {
	tracer := &TracerDetect{}
	tracer.RegisterTracee(100, false)

	original := ptraceGetRegs
	ptraceGetRegs = func(pid int, regs *syscall.PtraceRegs) error {
		assert.Equal(t, 100, pid)
		return syscall.ESRCH
	}
	t.Cleanup(func() {
		ptraceGetRegs = original
	})

	assert.Equal(t, syscallCheckTraceeGone, tracer.checkSyscall(100))
}

func TestCheckSyscallTreatsUnexpectedPtraceErrorsAsTracerError(t *testing.T) {
	tracer := &TracerDetect{}
	tracer.RegisterTracee(100, false)

	original := ptraceGetRegs
	ptraceGetRegs = func(pid int, regs *syscall.PtraceRegs) error {
		assert.Equal(t, 100, pid)
		return syscall.EIO
	}
	t.Cleanup(func() {
		ptraceGetRegs = original
	})

	assert.Equal(t, syscallCheckTracerError, tracer.checkSyscall(100))
}

func TestCheckSeccompTraceRejectsConsumedExecveBeforeEnter(t *testing.T) {
	tracer := &TracerDetect{}
	tracer.RegisterTracee(100, false)

	policy, err := makeCallPolicy(callPolicySpec{OneTimeCalls: []string{"execve"}})
	assert.NoError(t, err)
	tracer.setCallPolicy(policy)
	tracer.consumeBootstrapCall(syscall.SYS_EXECVE)

	original := ptraceGetRegs
	ptraceGetRegs = func(pid int, regs *syscall.PtraceRegs) error {
		assert.Equal(t, 100, pid)
		setTestSyscallNumber(regs, uint64(syscall.SYS_EXECVE))
		return nil
	}
	t.Cleanup(func() {
		ptraceGetRegs = original
	})

	assert.Equal(t, syscallCheckViolation, tracer.checkSeccompTrace(100))
}

func TestCheckSeccompTraceConsumesPolicyAtEventStop(t *testing.T) {
	tracer := &TracerDetect{}
	tracer.RegisterTracee(100, false)

	policy, err := makeCallPolicy(callPolicySpec{OneTimeCalls: []string{"getpid"}})
	assert.NoError(t, err)
	tracer.setCallPolicy(policy)

	original := ptraceGetRegs
	ptraceGetRegs = func(pid int, regs *syscall.PtraceRegs) error {
		assert.Equal(t, 100, pid)
		setTestSyscallNumber(regs, uint64(syscall.SYS_GETPID))
		return nil
	}
	t.Cleanup(func() {
		ptraceGetRegs = original
	})

	assert.Equal(t, syscallCheckOK, tracer.checkSeccompTrace(100))
	assert.False(t, tracer.callPolicy.CheckID(uint64(syscall.SYS_GETPID)))
}

func TestCheckSyscallAllowsPrlimit64Query(t *testing.T) {
	tracer := &TracerDetect{}
	tracer.RegisterTracee(100, false)

	policy, err := makeCallPolicy(callPolicySpec{AllowedCalls: []string{"prlimit64"}})
	assert.NoError(t, err)
	tracer.setCallPolicy(policy)

	original := ptraceGetRegs
	ptraceGetRegs = func(pid int, regs *syscall.PtraceRegs) error {
		assert.Equal(t, 100, pid)
		setTestSyscallNumber(regs, uint64(syscall.SYS_PRLIMIT64))
		setTestSyscallArgument(regs, 2, 0)
		return nil
	}
	t.Cleanup(func() {
		ptraceGetRegs = original
	})

	assert.Equal(t, syscallCheckOK, tracer.checkSyscall(100))
}

func TestCheckSyscallRejectsPrlimit64Set(t *testing.T) {
	tracer := &TracerDetect{}
	tracer.RegisterTracee(100, false)

	policy, err := makeCallPolicy(callPolicySpec{AllowedCalls: []string{"prlimit64"}})
	assert.NoError(t, err)
	tracer.setCallPolicy(policy)

	original := ptraceGetRegs
	ptraceGetRegs = func(pid int, regs *syscall.PtraceRegs) error {
		assert.Equal(t, 100, pid)
		setTestSyscallNumber(regs, uint64(syscall.SYS_PRLIMIT64))
		setTestSyscallArgument(regs, 2, 0x1234)
		return nil
	}
	t.Cleanup(func() {
		ptraceGetRegs = original
	})

	assert.Equal(t, syscallCheckViolation, tracer.checkSyscall(100))
}

func TestCheckSeccompTraceRejectsPrlimit64Set(t *testing.T) {
	tracer := &TracerDetect{}
	tracer.RegisterTracee(100, false)

	policy, err := makeCallPolicy(callPolicySpec{AllowedCalls: []string{"prlimit64"}})
	assert.NoError(t, err)
	tracer.setCallPolicy(policy)

	original := ptraceGetRegs
	ptraceGetRegs = func(pid int, regs *syscall.PtraceRegs) error {
		assert.Equal(t, 100, pid)
		setTestSyscallNumber(regs, uint64(syscall.SYS_PRLIMIT64))
		setTestSyscallArgument(regs, 2, 0x1234)
		return nil
	}
	t.Cleanup(func() {
		ptraceGetRegs = original
	})

	assert.Equal(t, syscallCheckViolation, tracer.checkSeccompTrace(100))
}

func TestCheckSyscallLogsAuditCall(t *testing.T) {
	observedLogs := useObservedLogger(t)
	tracer := &TracerDetect{}
	tracer.RegisterTracee(100, false)

	policy, err := makeCallPolicy(callPolicySpec{
		AllowedCalls: []string{"getpid"},
		AuditCalls:   []string{"getpid"},
	})
	assert.NoError(t, err)
	tracer.setCallPolicy(policy)

	original := ptraceGetRegs
	ptraceGetRegs = func(pid int, regs *syscall.PtraceRegs) error {
		assert.Equal(t, 100, pid)
		setTestSyscallNumber(regs, uint64(syscall.SYS_GETPID))
		return nil
	}
	t.Cleanup(func() {
		ptraceGetRegs = original
	})

	assert.Equal(t, syscallCheckOK, tracer.checkSyscall(100))
	assert.Equal(t, 1, observedLogs.FilterMessageSnippet("audit syscall source=ptrace pid=100 syscall=getpid").Len())
}

func TestCheckSeccompTraceLogsAuditCall(t *testing.T) {
	observedLogs := useObservedLogger(t)
	tracer := &TracerDetect{}
	tracer.RegisterTracee(100, false)

	policy, err := makeCallPolicy(callPolicySpec{
		AllowedCalls: []string{"getpid"},
		AuditCalls:   []string{"getpid"},
	})
	assert.NoError(t, err)
	tracer.setCallPolicy(policy)

	original := ptraceGetRegs
	ptraceGetRegs = func(pid int, regs *syscall.PtraceRegs) error {
		assert.Equal(t, 100, pid)
		setTestSyscallNumber(regs, uint64(syscall.SYS_GETPID))
		return nil
	}
	t.Cleanup(func() {
		ptraceGetRegs = original
	})

	assert.Equal(t, syscallCheckOK, tracer.checkSeccompTrace(100))
	assert.Equal(t, 1, observedLogs.FilterMessageSnippet("audit syscall source=seccomp pid=100 syscall=getpid").Len())
}

func TestCheckSeccompTraceChecksPolicyRegardlessOfSyscallPhase(t *testing.T) {
	tracer := &TracerDetect{}
	tracer.RegisterTracee(100, false)
	tracer.setInSyscall(100, true)

	policy, err := makeCallPolicy(callPolicySpec{})
	assert.NoError(t, err)
	tracer.setCallPolicy(policy)

	original := ptraceGetRegs
	ptraceGetRegs = func(pid int, regs *syscall.PtraceRegs) error {
		assert.Equal(t, 100, pid)
		setTestSyscallNumber(regs, uint64(syscall.SYS_GETPID))
		return nil
	}
	t.Cleanup(func() {
		ptraceGetRegs = original
	})

	assert.Equal(t, syscallCheckViolation, tracer.checkSeccompTrace(100))
	assert.True(t, tracer.inSyscall(100))
}

func useObservedLogger(t *testing.T) *observer.ObservedLogs {
	t.Helper()

	previousLog := log
	core, observedLogs := observer.New(zap.DebugLevel)
	SetLogger(zap.New(core).Sugar())
	t.Cleanup(func() {
		SetLogger(previousLog)
	})

	return observedLogs
}
