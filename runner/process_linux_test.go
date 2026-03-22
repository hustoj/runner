//go:build linux

package runner

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessTraceStopClassification(t *testing.T) {
	process := &Process{}

	process.Status = syscall.WaitStatus((int(syscall.SIGTRAP) << 8) | 0x7f)
	assert.True(t, process.IsInitialTraceStop())
	assert.False(t, process.IsSyscallStop())

	process.Status = syscall.WaitStatus((int(syscallStopSignal) << 8) | 0x7f)
	assert.False(t, process.IsInitialTraceStop())
	assert.True(t, process.IsSyscallStop())

	process.Status = syscall.WaitStatus((int(syscall.SIGSEGV) << 8) | 0x7f)
	assert.False(t, process.IsInitialTraceStop())
	assert.False(t, process.IsSyscallStop())
}

func TestCallPolicyConsume(t *testing.T) {
	policy := &CallPolicy{
		oneTimeCalls: map[uint64]bool{uint64(syscall.SYS_EXECVE): true},
		allowedCalls: map[uint64]bool{},
	}

	policy.Consume(uint64(syscall.SYS_EXECVE))
	assert.False(t, policy.CheckID(uint64(syscall.SYS_EXECVE)))
}
