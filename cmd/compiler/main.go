package main

import (
	"encoding/json"
	"fmt"
)

type RunResult struct {
	Success bool `json:"success"`
}

func main() {
	m := loadConfig()
	initLog(m)
	r := handle(m)
	res, _ := json.Marshal(r)
	fmt.Println(string(res))
}
