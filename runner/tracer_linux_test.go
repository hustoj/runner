//go:build linux

package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetNameReturnsFallbackForUnknownSyscall(t *testing.T) {
	assert.Equal(t, "unknown_9999", getName(9999))
}
