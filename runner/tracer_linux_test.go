//go:build linux

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
