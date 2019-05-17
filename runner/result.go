package runner

import (
	"fmt"
	"os"
	"syscall"
)

const (
	PENDING            = 0
	PENDING_REJUDGE    = 1
	COMPILING          = 2
	REJUDGING          = 3
	ACCEPT             = 4
	PRESENTATION_ERROR = 5
	WRONG_ANSWER       = 6
	TIME_LIMIT         = 7
	MEMORY_LIMIT       = 8
	OUTPUT_LIMIT       = 9
	RUNTIME_ERROR      = 10
	COMPILE_ERROR      = 11
	COMPILE_OK         = 12
	TEST_RUN           = 13
)

type Result struct {
	RetCode    int   `json:"status"`
	PeakMemory int64 `json:"peak_memory"`
	RusageMemory int64 `json:"rusage_memory"`
	TimeCost   int64 `json:"time"`
}

func (res *Result) String() string {
	return fmt.Sprintf("Result: %d, CPU: %d, PeakMemory: %dkb", res.RetCode, res.TimeCost, res.PeakMemory)
}

func (res *Result) isAccept() bool {
	return res.RetCode == ACCEPT
}

func (res *Result) detectSignal(signal os.Signal) {
	log.Debugf("Detect signal %v", signal)
	if signal == syscall.SIGUSR1 {
		res.RetCode = RUNTIME_ERROR
		return
	}
	if signal == syscall.SIGALRM || signal == syscall.SIGXCPU {
		res.RetCode = TIME_LIMIT
		return
	}
	if signal == syscall.SIGXFSZ {
		res.RetCode = OUTPUT_LIMIT
		return
	}

	res.RetCode = RUNTIME_ERROR
}

func (res *Result) Init() {
	res.TimeCost = 0
	res.PeakMemory = 0
	res.RetCode = ACCEPT
}
