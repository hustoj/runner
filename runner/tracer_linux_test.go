//go:build linux && (amd64 || arm64)

package runner

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetNameReturnsFallbackForUnknownSyscall(t *testing.T) {
	assert.Equal(t, "unknown_9999", getName(9999))
}

func TestConsumeBootstrapCallConsumesExecveQuota(t *testing.T) {
	oneTimeCalls := []string{"execve"}
	allowedCalls := []string{}
	tracer := &TracerDetect{}

	policy, err := makeCallPolicy(&oneTimeCalls, &allowedCalls)
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

	tracer.setInSyscall(100, true)
	tracer.FinishPtraceEvent(100)
	assert.False(t, tracer.inSyscall(100))

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
