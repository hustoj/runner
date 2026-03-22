package runner

import "github.com/hustoj/runner/sec"

type CallPolicy struct {
	oneTimeCalls map[uint64]bool
	allowedCalls map[uint64]bool
}

func makeCallPolicy(ones *[]string, allows *[]string) *CallPolicy {
	oneTimeCalls := make(map[uint64]bool)
	allowedCalls := make(map[uint64]bool)
	for _, s := range *ones {
		n, err := sec.SCTbl.GetID(s)
		checkErr(err)
		oneTimeCalls[uint64(n)] = true
	}
	for _, s := range *allows {
		if len(s) == 0 {
			continue
		}
		n, err := sec.SCTbl.GetID(s)
		checkErr(err)
		allowedCalls[uint64(n)] = true
	}

	return &CallPolicy{oneTimeCalls, allowedCalls}
}

func (c *CallPolicy) CheckID(callID uint64) bool {
	if c.allowedCalls[callID] {
		return true
	}
	if c.oneTimeCalls[callID] {
		delete(c.oneTimeCalls, callID)
		return true
	}
	return false
}
