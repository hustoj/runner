package runner

import (
	"os/exec"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestKillProcessTreeNoRaceWithTraceLoopState runs the real wall-clock watchdog
// goroutine (which calls killProcessTree) concurrently with the Process/Result
// state mutations the trace loop performs, under the race detector.
//
// The watchdog is only allowed to issue idempotent SIGKILLs and read immutable
// task state (task.process.Pid, task.taskCtrl); it must NOT write Process.IsKilled,
// Result, or the rusage maps, which stay owned by the trace-loop goroutine. A
// passing `go test -race` run proves that invariant holds (and guards against
// later "fixes" that would move state writes onto the watchdog goroutine).
func TestKillProcessTreeNoRaceWithTraceLoopState(t *testing.T) {
	cmd := exec.Command("/bin/sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot spawn child for kill target: %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	pid := cmd.Process.Pid
	if _, err := InitLogger("/dev/null", false); err != nil {
		t.Fatalf("InitLogger(/dev/null) error = %v", err)
	}
	task := &RunningTask{
		setting: &TaskConfig{WallClock: 1},
		process: NewProcess(pid),
	}
	task.Result = &Result{}
	task.Result.Init()
	task.wallClockTimedOut.Store(false)

	stopWatchdog := task.startWallClockWatchdog()
	defer stopWatchdog()

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Run the trace-loop-side mutations past the 1s watchdog deadline so
		// killProcessTree overlaps this work. recordRusage/RemoveTracee exercise
		// the finalize path; Result writes exercise the fields the watchdog
		// must not touch.
		deadline := time.Now().Add(1500 * time.Millisecond)
		for i := 0; time.Now().Before(deadline); i++ {
			task.process.recordRusage(pid, syscall.Rusage{Maxrss: int64(i)})
			task.process.SetThreadGroup(pid, pid)
			task.process.RemoveTracee(pid)
			task.process.AddTracee(pid)
			task.Result.RetCode = RUNTIME_ERROR
			task.Result.PeakMemory = int64(i)
			runtime.Gosched()
		}
		task.process.Kill()
	}()

	<-done

	assert.NotNil(t, task.process)
}
