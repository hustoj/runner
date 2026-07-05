package runner

import (
	"fmt"

	"github.com/hustoj/runner/sec"
)

type CallPolicy struct {
	oneTimeCalls map[uint64]bool
	allowedCalls map[uint64]bool
	auditCalls   map[uint64]bool
}

type callPolicySpec struct {
	OneTimeCalls []string
	AllowedCalls []string
	AuditCalls   []string
}

func makeCallPolicy(spec callPolicySpec) (*CallPolicy, error) {
	oneTimeCalls, err := syscallIDSet("one-time syscall", spec.OneTimeCalls)
	if err != nil {
		return nil, err
	}
	allowedCalls, err := syscallIDSet("allowed syscall", spec.AllowedCalls)
	if err != nil {
		return nil, err
	}
	auditCalls, err := syscallIDSet("audit syscall", spec.AuditCalls)
	if err != nil {
		return nil, err
	}

	return &CallPolicy{oneTimeCalls, allowedCalls, auditCalls}, nil
}

func syscallIDSet(category string, names []string) (map[uint64]bool, error) {
	calls := make(map[uint64]bool)
	for _, s := range names {
		if len(s) == 0 {
			continue
		}
		n, err := sec.SCTbl.GetID(s)
		if err != nil {
			return nil, fmt.Errorf("%s %q: %w", category, s, err)
		}
		calls[uint64(n)] = true
	}
	return calls, nil
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

func (c *CallPolicy) ShouldAudit(callID uint64) bool {
	return c.auditCalls[callID]
}
