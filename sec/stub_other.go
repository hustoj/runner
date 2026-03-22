//go:build !(linux && amd64)

package sec

import "fmt"

const MaxCallNumber = 0

type SCTable struct{}

var SCTbl *SCTable

func (t *SCTable) Init() {}

func (t *SCTable) GetName(callID int) (string, error) {
	return "", fmt.Errorf("syscall table is only available on linux/amd64")
}

func (t *SCTable) GetID(callName string) (int, error) {
	return -1, fmt.Errorf("syscall table is only available on linux/amd64")
}

func init() {
	SCTbl = new(SCTable)
}
