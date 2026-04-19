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

	process.Status = syscall.WaitStatus((syscall.PTRACE_EVENT_CLONE << 16) | (int(syscall.SIGTRAP) << 8) | 0x7f)
	assert.False(t, process.IsInitialTraceStop())
	assert.False(t, process.IsSyscallStop())
	assert.True(t, process.IsPtraceEventStop())
	assert.Equal(t, syscall.PTRACE_EVENT_CLONE, process.PtraceEvent())

	process.Status = syscall.WaitStatus((int(syscall.SIGSEGV) << 8) | 0x7f)
	assert.False(t, process.IsInitialTraceStop())
	assert.False(t, process.IsSyscallStop())
	assert.False(t, process.IsPtraceEventStop())
}

func TestCallPolicyConsume(t *testing.T) {
	policy := &CallPolicy{
		oneTimeCalls: map[uint64]bool{uint64(syscall.SYS_EXECVE): true},
		allowedCalls: map[uint64]bool{},
	}

	policy.Consume(uint64(syscall.SYS_EXECVE))
	assert.False(t, policy.CheckID(uint64(syscall.SYS_EXECVE)))
}

func TestProcessReturnsPendingStopAfterTraceeRegistration(t *testing.T) {
	process := NewProcess(100)
	process.rememberPendingStop(101, syscall.WaitStatus((int(syscall.SIGSTOP)<<8)|0x7f), syscall.Rusage{Maxrss: 64})

	_, _, ok := process.takePendingTrackedStop()
	assert.False(t, ok)

	process.AddTracee(101)
	pid, stop, ok := process.takePendingTrackedStop()
	assert.True(t, ok)
	assert.Equal(t, 101, pid)
	assert.Equal(t, int64(64), stop.rusage.Maxrss)
}

func TestProcessMemoryAggregatesPerThreadGroup(t *testing.T) {
	process := NewProcess(100)
	process.recordRusage(100, syscall.Rusage{Maxrss: 64})
	process.recordRusage(101, syscall.Rusage{Maxrss: 32})
	process.recordRusage(200, syscall.Rusage{Maxrss: 48})
	process.SetThreadGroup(100, 100)
	process.SetThreadGroup(101, 100)
	process.SetThreadGroup(200, 200)

	assert.Equal(t, int64(112), process.Memory())
}

func TestProcessMemorySubtractsBootstrapOffset(t *testing.T) {
	process := NewProcess(100)
	process.recordRusage(100, syscall.Rusage{Maxrss: 96})
	process.recordRusage(200, syscall.Rusage{Maxrss: 48})
	process.SetThreadGroup(100, 100)
	process.SetThreadGroup(200, 200)
	process.SetRusageOffset(100, 64)

	assert.Equal(t, int64(80), process.Memory())
}
