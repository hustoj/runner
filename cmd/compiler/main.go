package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/sirupsen/logrus"
	"hustoj/runner/runner"
)

var log *logrus.Logger

type RunResult struct {
	Success bool `json:"success"`
}

func setrLimits(cpu, memory, output, stack uint64) {
	syscall.Setrlimit(syscall.RLIMIT_CPU, &syscall.Rlimit{Max: cpu + 1, Cur: cpu})
	syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Max: output << 20, Cur: output << 20})
	syscall.Setrlimit(syscall.RLIMIT_STACK, &syscall.Rlimit{Max: stack << 20, Cur: stack << 20})
	syscall.Setrlimit(syscall.RLIMIT_AS, &syscall.Rlimit{Max: memory << 20, Cur: memory << 20})
	syscall.Syscall(syscall.SYS_ALARM, uintptr(cpu*3+2), 0, 0)
}

func doCompile(cfg *CompileConfig) {
	setrLimits(uint64(cfg.CPU), uint64(cfg.Memory), uint64(cfg.Output), uint64(cfg.Stack))
	runner.DupFileForWrite("compile.err", os.Stderr)
	runner.DupFileForWrite("compile.out", os.Stdout)
	binary, lookErr := exec.LookPath(cfg.Command)
	if lookErr != nil {
		panic(lookErr)
	}

	env := os.Environ()
	args := makeArgs(binary, cfg)
	err := syscall.Exec(binary, args, env)
	if err != nil {
		fmt.Printf("exec failed: %s\n", err)
	}
}

func makeArgs(binary string, cfg *CompileConfig) []string {
	args := cfg.GetArgs()
	return append([]string{binary}, args...)
}

func runProcessC(cfg *CompileConfig) int {
	pid := fork()
	if pid < 0 {
		panic(errors.New("fork error"))
	}
	if pid == 0 {
		doCompile(cfg)
	}
	return pid
}

func fork() int {
	r1, _, _ := syscall.Syscall(syscall.SYS_FORK, 0, 0, 0)
	return int(r1)
}

func handle(cfg *CompileConfig) *RunResult {
	var status syscall.WaitStatus
	var rusage syscall.Rusage
	pid := runProcessC(cfg)
	log.Info("Child Pid is: ", pid)

	result := RunResult{Success: true}

	pid, _ = syscall.Wait4(pid, &status, 0, &rusage)
	if status != 0 {
		result.Success = false
	}
	return &result
}

func main() {
	m := loadConfig()
	fmt.Printf("%#v\n", m)
	log = runner.InitLogger(m.LogPath, m.Verbose)
	r := handle(m)
	res, _ := json.Marshal(r)
	fmt.Println(string(res))
}
