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
	RetCode  int	`json:"status"`
	Memory   int64	`json:"memory"`
	TimeCost int64	`json:"time"`
}

func (res *Result) String() string {
	return fmt.Sprintf("Result: %d, Time: %d, Memory: %dkb", res.RetCode, res.TimeCost, res.Memory)
}

func (res *Result) detectSignal(signal os.Signal) {
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
	res.Memory = 0
	res.RetCode = ACCEPT
}
