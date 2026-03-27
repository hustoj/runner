package sec

import "fmt"

// SCTable provides bidirectional mapping between syscall names and IDs.
// The actual entries are populated by architecture-specific table files
// (table_linux_amd64.go, table_linux_arm64.go) using golang.org/x/sys/unix
// constants, ensuring correct syscall numbers for each architecture.
type SCTable struct {
	nameToInt map[string]int
	intToName map[int]string
}

var SCTbl *SCTable

func (t *SCTable) Init() {
	t.nameToInt = make(map[string]int)
	t.intToName = make(map[int]string)
	for name, id := range syscallEntries() {
		t.nameToInt[name] = id
		t.intToName[id] = name
	}
}

func (t *SCTable) GetName(callID int) (string, error) {
	if callName, ok := t.intToName[callID]; ok {
		return callName, nil
	}
	return "", fmt.Errorf("NoCallWithID %d", callID)
}

func (t *SCTable) GetID(callName string) (int, error) {
	callID, ok := t.nameToInt[callName]
	if ok {
		return callID, nil
	}
	return -1, fmt.Errorf("NoCallNamed %s", callName)
}

func init() {
	SCTbl = new(SCTable)
	SCTbl.Init()
}
