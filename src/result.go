package runner

import (
	"fmt"
	"os"
	"syscall"
)

const (
	WT0           = 0
	WT1           = 1
	CI            = 2
	RI            = 3
	ACCEPT        = 4
	PRESENT_ERR   = 5
	WRONG_ANSWER  = 6
	TIME_LIMIT    = 7
	MEMORY_LIMIT  = 8
	OUTPUT_LIMIT  = 9
	RUNTIME_ERROR = 10
	COMPILE_ERROR = 11
	CO            = 12
)

type Result struct {
	RetCode  int
	Memory   int64
	TimeCost int64
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
	if signal == syscall.SIGSEGV {
		res.RetCode = MEMORY_LIMIT
		return
	}
	res.RetCode = RUNTIME_ERROR
}

func (res *Result) Init() {
	res.TimeCost = 0
	res.Memory = 0
	res.RetCode = ACCEPT
}
