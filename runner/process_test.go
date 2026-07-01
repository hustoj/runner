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
