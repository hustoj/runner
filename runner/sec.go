package runner

import (
	"fmt"

	"github.com/hustoj/runner/sec"
)

type CallPolicy struct {
	oneTimeCalls map[uint64]bool
	allowedCalls map[uint64]bool
}

func makeCallPolicy(ones *[]string, allows *[]string) (*CallPolicy, error) {
	oneTimeCalls := make(map[uint64]bool)
	allowedCalls := make(map[uint64]bool)
	for _, s := range *ones {
		n, err := sec.SCTbl.GetID(s)
		if err != nil {
			return nil, fmt.Errorf("one-time syscall %q: %w", s, err)
		}
		oneTimeCalls[uint64(n)] = true
	}
	for _, s := range *allows {
		if len(s) == 0 {
			continue
		}
		n, err := sec.SCTbl.GetID(s)
		if err != nil {
			return nil, fmt.Errorf("allowed syscall %q: %w", s, err)
		}
		allowedCalls[uint64(n)] = true
	}

	return &CallPolicy{oneTimeCalls, allowedCalls}, nil
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

func (c *CallPolicy) Consume(callID uint64) {
	delete(c.oneTimeCalls, callID)
}
