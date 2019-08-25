package runner

import "github.com/hustoj/runner/sec"

type CallPolicy struct {
	oneTimeCalls []bool
	allowedCalls []bool
}

func getName(syscallID uint64) string {
	name, err := sec.SCTbl.GetName(int(syscallID))
	checkErr(err)
	return name
}

func makeCallPolicy(ones *[]string, allows *[]string) *CallPolicy {
	var oneTimeCalls = make([]bool, 400)
	var allowedCalls = make([]bool, 400)
	for _, s := range *ones {
		n, err := sec.SCTbl.GetID(s)
		checkErr(err)
		oneTimeCalls[n] = true
	}
	for _, s := range *allows {
		n, err := sec.SCTbl.GetID(s)
		checkErr(err)
		allowedCalls[n] = true
	}

	return &CallPolicy{oneTimeCalls, allowedCalls}
}

func (c *CallPolicy) CheckID(callID uint64) bool {
	if c.allowedCalls[callID] {
		return true
	}
	if c.oneTimeCalls[callID] {
		c.oneTimeCalls[callID] = false
		return true
	}
	return false
}
