//go:build linux && (amd64 || arm64)

package sec

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

func TestNameToInt(t *testing.T) {
	id, err := SCTbl.GetID("read")
	assert.Equal(t, unix.SYS_READ, id, "read must match unix.SYS_READ")
	assert.Nil(t, err, "err must be nil")
}

func TestIntToName(t *testing.T) {
	name, err := SCTbl.GetName(unix.SYS_CLOSE)
	assert.Equal(t, "close", name, "SYS_CLOSE must resolve to close")
	assert.Nil(t, err, "err must be nil")
}

func TestNameToIntNotExisted(t *testing.T) {
	_, err := SCTbl.GetID("openn")
	assert.NotNil(t, err, "err must not be nil")
}

func TestIDNotExisted(t *testing.T) {
	_, err := SCTbl.GetName(-1)
	assert.NotNil(t, err, "err must not be nil")
}

func TestIDNotExisted2(t *testing.T) {
	_, err := SCTbl.GetName(99999)
	assert.NotNil(t, err, "err must not be nil")
}

func TestLatestKernelSyscalls(t *testing.T) {
	// openat2 exists on both amd64 and arm64
	name, err := SCTbl.GetName(unix.SYS_OPENAT2)
	assert.Equal(t, "openat2", name, "name must be openat2")
	assert.Nil(t, err, "err must be nil")

	// rseq_slice_yield is a recent syscall (not yet in x/sys)
	name, err = SCTbl.GetName(471)
	assert.Equal(t, "rseq_slice_yield", name, "name must be rseq_slice_yield")
	assert.Nil(t, err, "err must be nil")
}

func TestArchSpecificSyscall(t *testing.T) {
	switch runtime.GOARCH {
	case "amd64":
		// arch_prctl is x86_64-specific
		id, err := SCTbl.GetID("arch_prctl")
		assert.Equal(t, unix.SYS_ARCH_PRCTL, id)
		assert.Nil(t, err)
	case "arm64":
		// arch_prctl should NOT exist on arm64
		_, err := SCTbl.GetID("arch_prctl")
		assert.NotNil(t, err)
	}
}

func TestBidirectionalConsistency(t *testing.T) {
	// Every name→id mapping should be reversible
	for name, id := range syscallEntries() {
		gotName, err := SCTbl.GetName(id)
		assert.Nil(t, err, "GetName(%d) should not error", id)
		assert.Equal(t, name, gotName, "round-trip failed for %s=%d", name, id)

		gotID, err := SCTbl.GetID(name)
		assert.Nil(t, err, "GetID(%s) should not error", name)
		assert.Equal(t, id, gotID, "round-trip failed for %s=%d", name, id)
	}
}
