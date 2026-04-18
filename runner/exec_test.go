package runner

import (
	"os/exec"
	"strconv"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func waitStatusForExit(t *testing.T, code int) syscall.WaitStatus {
	t.Helper()

	cmd := exec.Command("sh", "-c", "exit "+strconv.Itoa(code))
	_ = cmd.Run()
	if cmd.ProcessState == nil {
		t.Fatal("process state is nil")
	}

	status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
	if !ok {
		t.Fatalf("unexpected process state type: %T", cmd.ProcessState.Sys())
	}
	return status
}

func TestApplyExitCodeKeepsAcceptOnZeroExit(t *testing.T) {
	task := RunningTask{Result: &Result{}}
	task.Result.Init()

	task.applyExitCode(waitStatusForExit(t, 0))

	assert.Equal(t, ACCEPT, task.Result.RetCode)
}

func TestApplyExitCodeMarksRuntimeErrorOnNonZeroExit(t *testing.T) {
	task := RunningTask{Result: &Result{}}
	task.Result.Init()

	task.applyExitCode(waitStatusForExit(t, 1))

	assert.Equal(t, RUNTIME_ERROR, task.Result.RetCode)
}

func TestApplyExitCodeDoesNotOverrideExistingResult(t *testing.T) {
	task := RunningTask{Result: &Result{}}
	task.Result.Init()
	task.Result.RetCode = TIME_LIMIT

	task.applyExitCode(waitStatusForExit(t, 1))

	assert.Equal(t, TIME_LIMIT, task.Result.RetCode)
}
