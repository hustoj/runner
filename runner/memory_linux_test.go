//go:build linux

package runner

import (
	"errors"
	"os"
	"syscall"
	"testing"

	"go.uber.org/zap"
)

func TestRefreshMemoryAggregatesActiveTraceesByThreadGroup(t *testing.T) {
	SetLogger(zap.NewNop().Sugar())
	defer SetLogger(nil)

	oldProcStatusReader := procStatusReader
	procStatusReader = func(path string) ([]byte, error) {
		switch path {
		case "/proc/100/status":
			return []byte("Name:\tmain\nTgid:\t100\nVmHWM:\t64 kB\n"), nil
		case "/proc/101/status":
			return []byte("Name:\tworker\nTgid:\t100\nVmHWM:\t64 kB\n"), nil
		case "/proc/200/status":
			return []byte("Name:\thelper\nTgid:\t200\nVmHWM:\t48 kB\n"), nil
		default:
			return nil, os.ErrNotExist
		}
	}
	defer func() {
		procStatusReader = oldProcStatusReader
	}()

	process := NewProcess(100)
	process.AddTracee(101)
	process.AddTracee(200)

	task := &RunningTask{
		process: process,
		Result:  &Result{},
	}
	task.Result.Init()
	task.refreshMemory()

	if task.Result.PeakMemory != 112 {
		t.Fatalf("PeakMemory = %d, want 112", task.Result.PeakMemory)
	}
}

func TestRefreshMemoryUsesAdjustedRusageAsPeakFallback(t *testing.T) {
	SetLogger(zap.NewNop().Sugar())
	defer SetLogger(nil)

	oldProcStatusReader := procStatusReader
	procStatusReader = func(path string) ([]byte, error) {
		return nil, errors.New("proc status unavailable")
	}
	defer func() {
		procStatusReader = oldProcStatusReader
	}()

	process := NewProcess(100)
	process.SetThreadGroup(100, 100)
	process.SetRusageOffset(100, 64)
	process.recordRusage(100, syscall.Rusage{Maxrss: 96})

	task := &RunningTask{
		process: process,
		Result:  &Result{},
	}
	task.Result.Init()
	task.refreshMemory()

	if task.Result.RusageMemory != 32 {
		t.Fatalf("RusageMemory = %d, want 32", task.Result.RusageMemory)
	}
	if task.Result.PeakMemory != 32 {
		t.Fatalf("PeakMemory = %d, want 32", task.Result.PeakMemory)
	}
}
