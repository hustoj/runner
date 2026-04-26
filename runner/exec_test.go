package runner

import (
	"os/exec"
	"strconv"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
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

func TestApplyTerminationSignalTreatsSIGKILLAsTimeLimitWhenOverTime(t *testing.T) {
	_, err := InitLogger("/dev/null", false)
	assert.NoError(t, err)

	task := RunningTask{
		Result:    &Result{},
		timeLimit: 1_000_000,
	}
	task.Result.Init()
	task.Result.TimeCost = 1_000_001

	task.applyTerminationSignal(syscall.SIGKILL)

	assert.Equal(t, TIME_LIMIT, task.Result.RetCode)
}

func TestApplyTerminationSignalKeepsRuntimeErrorForSIGKILLWithinLimits(t *testing.T) {
	_, err := InitLogger("/dev/null", false)
	assert.NoError(t, err)

	task := RunningTask{
		Result:    &Result{},
		timeLimit: 1_000_000,
	}
	task.Result.Init()
	task.Result.TimeCost = 500_000

	task.applyTerminationSignal(syscall.SIGKILL)

	assert.Equal(t, RUNTIME_ERROR, task.Result.RetCode)
}

func TestApplyTerminationSignalTreatsSIGKILLAsMemoryLimitWhenControllerExceeded(t *testing.T) {
	SetLogger(zap.NewNop().Sugar())
	defer SetLogger(nil)

	task := RunningTask{
		Result: &Result{},
		memoryCtrl: fakeMemoryController{
			status: memoryStatus{
				PeakMemoryKB: 2048,
				OOMCount:     1,
			},
		},
	}
	task.Result.Init()

	task.applyTerminationSignal(syscall.SIGKILL)

	assert.Equal(t, MEMORY_LIMIT, task.Result.RetCode)
}

func TestCheckPromotesRuntimeErrorToMemoryLimitWhenControllerExceeded(t *testing.T) {
	SetLogger(zap.NewNop().Sugar())
	defer SetLogger(nil)

	task := RunningTask{
		Result: &Result{
			RetCode: RUNTIME_ERROR,
		},
		memoryCtrl: fakeMemoryController{
			status: memoryStatus{
				PeakMemoryKB: 4096,
				OOMCount:     1,
			},
		},
	}

	task.check()

	assert.Equal(t, MEMORY_LIMIT, task.Result.RetCode)
}
