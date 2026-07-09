package runner

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

// GetTimeCost accumulates CPU time (Utime + Stime) across all waited tracees,
// returning microseconds. It does not deduplicate by thread group.
func TestGetTimeCostEmpty(t *testing.T) {
	process := NewProcess(1)
	assert.Equal(t, int64(0), process.GetTimeCost())
}

func TestGetTimeCostSingleEntry(t *testing.T) {
	process := NewProcess(1)
	process.recordRusage(1, syscall.Rusage{
		Utime: syscall.Timeval{Sec: 2, Usec: 500_000},
		Stime: syscall.Timeval{Sec: 1, Usec: 500_000},
	})
	// (2+1)*1e6 + (500000+500000) = 3_000_000 + 1_000_000 = 4_000_000
	assert.Equal(t, int64(4_000_000), process.GetTimeCost())
}

func TestGetTimeCostMultipleEntries(t *testing.T) {
	process := NewProcess(1)
	process.recordRusage(1, syscall.Rusage{
		Utime: syscall.Timeval{Sec: 1, Usec: 0},
		Stime: syscall.Timeval{Sec: 0, Usec: 0},
	})
	process.recordRusage(2, syscall.Rusage{
		Utime: syscall.Timeval{Sec: 0, Usec: 500_000},
		Stime: syscall.Timeval{Sec: 1, Usec: 500_000},
	})
	// entry 1: (1+0)*1e6 + 0 = 1_000_000
	// entry 2: (0+1)*1e6 + (500_000+500_000) = 2_000_000
	// total: 3_000_000
	assert.Equal(t, int64(3_000_000), process.GetTimeCost())
}

func TestGetTimeCostOverwriteByPid(t *testing.T) {
	// recordRusage keyed by pid replaces prior value, not accumulates.
	process := NewProcess(1)
	process.recordRusage(1, syscall.Rusage{
		Utime: syscall.Timeval{Sec: 5, Usec: 0},
	})
	process.recordRusage(1, syscall.Rusage{
		Utime: syscall.Timeval{Sec: 1, Usec: 0},
	})
	// Only the latest value for pid=1 counts: 1*1e6
	assert.Equal(t, int64(1_000_000), process.GetTimeCost())
}

// RemoveTracee finalizes a tracee's rusage instead of discarding it, so an
// exited tracee keeps contributing CPU time and peak memory until the Process is
// discarded. This preserves accounting that wait4 already reported and stops a
// reused PID from silently dropping the prior occupant's stats.
func TestRemoveTraceePreservesFinalizedCpuTime(t *testing.T) {
	process := NewProcess(1)
	process.recordRusage(1, syscall.Rusage{
		Utime: syscall.Timeval{Sec: 2, Usec: 500_000},
		Stime: syscall.Timeval{Sec: 1, Usec: 500_000},
	})
	want := int64(4_000_000) // (2+1)*1e6 + (500k+500k)

	process.RemoveTracee(1)

	assert.Equal(t, want, process.GetTimeCost(), "exited tracee CPU time must remain in the total")
}

func TestRemoveTraceePreservesFinalizedMemoryPeak(t *testing.T) {
	process := NewProcess(1)
	process.recordRusage(1, syscall.Rusage{Maxrss: 500})

	process.RemoveTracee(1)

	assert.Equal(t, int64(500), process.Memory(), "exited tracee peak memory must remain in the total")
}

// A reused PID must not have its new occupant overwrite the prior occupant's
// accounting: both contribute (CPU summed, peak maxed) while the PID belongs to
// the same traced tree.
func TestReusedPidKeepsBothOccupantsAccounting(t *testing.T) {
	process := NewProcess(1)

	// Prior occupant of pid 500.
	process.recordRusage(500, syscall.Rusage{
		Utime:  syscall.Timeval{Sec: 1},
		Maxrss: 1000,
	})
	process.SetThreadGroup(500, 500)
	process.RemoveTracee(500)

	// PID 500 is reused by a different occupant.
	process.recordRusage(500, syscall.Rusage{
		Utime:  syscall.Timeval{Sec: 0, Usec: 500_000},
		Stime:  syscall.Timeval{Sec: 0, Usec: 500_000},
		Maxrss: 100,
	})
	process.SetThreadGroup(500, 500)

	// CPU: 1s (prior) + 1s (new) = 2s.
	assert.Equal(t, int64(2_000_000), process.GetTimeCost())
	// Peak: max(1000, 100) = 1000 (prior occupant set the high-water mark).
	assert.Equal(t, int64(1000), process.Memory())
}
