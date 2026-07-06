package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type RunResult struct {
	Success bool `json:"success"`
}

func main() {
	if isCompilerBootstrapProcess() {
		m, err := loadBootstrapConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "compiler: load bootstrap config: %v\n", err)
			os.Exit(1)
		}
		bootstrapCompile(m)
		return
	}

	m := loadConfig()
	initLog(m)
	r := handle(m)
	res, _ := json.Marshal(r)
	fmt.Println(string(res))
}
