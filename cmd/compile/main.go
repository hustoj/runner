package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"syscall"

	log "github.com/sirupsen/logrus"
)

const (
	RESULT_CE = "CE"
	RESULT_OK = "OK"
	RESULT_TL = "TL"
	RESULT_ML = "ML"
	RESULT_OL = "OL"
	RESULT_RE = "RE"
	RESULT_RF = "RF"
)

type RunResult struct {
	Success bool   `json:"success"`
	Result  string `json:"result"`
	Memory  int    `json:"mem"`
	Time    int    `json:"time"`
}
func setrLimits(cpu, memory, output, stack uint64) {
	syscall.Setrlimit(syscall.RLIMIT_CPU, &syscall.Rlimit{Max: cpu + 1, Cur: cpu})
	syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Max: output << 20, Cur: output << 20})
	syscall.Setrlimit(syscall.RLIMIT_STACK, &syscall.Rlimit{Max: stack << 20, Cur: stack << 20})
	syscall.Setrlimit(syscall.RLIMIT_AS, &syscall.Rlimit{Max: memory << 20, Cur: memory << 20})
	syscall.Syscall(syscall.SYS_ALARM, uintptr(cpu*3+2), 0, 0)
}

func runProcessC(cfg *CompileConfig) int {
	pid := fork()
	if pid < 0 {
		panic(errors.New("fork error"))
	}
	if pid == 0 {
		setrLimits(uint64(cfg.CPU), uint64(cfg.Memory), uint64(cfg.Output), uint64(cfg.Stack))
		syscall.Exec(cfg.Command, []string{cfg.Command, "-o", "main", "-O2", "-fmax-errors=10", "-Wall", "-lm", "--static", "--std=c99", "main.c"}, []string{"PATH=/usr/bin/"})
	}
	return pid
}

func fork() int {
	r1, _, _ := syscall.Syscall(syscall.SYS_FORK, 0, 0, 0)
	return int(r1)
}

func runner(cfg *CompileConfig) *RunResult {
	var status syscall.WaitStatus
	var rusage syscall.Rusage
	pid := runProcessC(cfg)
	log.Info("Child Pid is: ", pid)

	result := RunResult{Success: true, Result: RESULT_OK, Time: 0, Memory: 0}

	pid, _ = syscall.Wait4(pid, &status, 0, &rusage)
	if status != 0 {
		result.Success = false
		result.Result = RESULT_CE
	}
	return &result
}

func main() {
	m := loadConfig()
	configLogger(m.Verbose)
	r := runner(m)
	res, _ := json.Marshal(r)
	fmt.Println(string(res))
}

func configLogger(debug bool) {
	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	if !debug {
		log.SetLevel(log.WarnLevel)
	}
}
