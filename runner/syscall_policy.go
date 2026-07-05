package runner

import "fmt"

type SyscallPolicyConfig struct {
	Allow []string `default:"" json:"Allow"`
	Deny  []string `default:"" json:"Deny"`
	Trace []string `default:"" json:"Trace"`
	Audit []string `default:"" json:"Audit"`
}

type EffectiveSyscallPolicy struct {
	OneTime []string
	Allow   []string
	Deny    []string
	Trace   []string
	Audit   []string
}

type compiledSyscallPolicy struct {
	Ptrace              callPolicySpec
	SeccompAllowedCalls []string
	SeccompTracedCalls  []string
}

// postSeccompStartupCalls are owned by the runner's hybrid startup protocol.
// User policy cannot deny them because the child may need them after installing
// the seccomp filter to report execve failures and exit cleanly.
var postSeccompStartupCalls = []string{"write", "close", "exit", "exit_group"}

func (tc *TaskConfig) effectiveSyscallPolicy() EffectiveSyscallPolicy {
	allow := make([]string, 0, len(tc.AllowedCalls)+len(tc.AdditionCalls)+len(tc.SyscallPolicy.Allow))
	allow = append(allow, tc.AllowedCalls...)
	allow = append(allow, tc.AdditionCalls...)
	allow = append(allow, tc.SyscallPolicy.Allow...)
	deny := uniqueSyscallNames(tc.SyscallPolicy.Deny)

	return EffectiveSyscallPolicy{
		OneTime: uniqueSyscallNames(tc.OneTimeCalls),
		Allow:   removeSyscallNames(uniqueSyscallNames(allow), deny),
		Deny:    deny,
		Trace:   uniqueSyscallNames(tc.SyscallPolicy.Trace),
		Audit:   uniqueSyscallNames(tc.SyscallPolicy.Audit),
	}
}

func (tc *TaskConfig) validateSyscallPolicy() error {
	for _, check := range []struct {
		field string
		names []string
	}{
		{"syscallPolicy.allow", tc.SyscallPolicy.Allow},
		{"syscallPolicy.deny", tc.SyscallPolicy.Deny},
		{"syscallPolicy.trace", tc.SyscallPolicy.Trace},
		{"syscallPolicy.audit", tc.SyscallPolicy.Audit},
	} {
		if err := validateSyscallNames(check.field, check.names); err != nil {
			return err
		}
	}

	return validateSyscallPolicyOwnership(tc.effectiveSyscallPolicy())
}

func (tc *TaskConfig) compileSyscallPolicy() (compiledSyscallPolicy, error) {
	if err := tc.validateSyscallPolicy(); err != nil {
		return compiledSyscallPolicy{}, err
	}
	return tc.effectiveSyscallPolicy().compile(), nil
}

func (policy EffectiveSyscallPolicy) compile() compiledSyscallPolicy {
	seccompAllowed := make([]string, 0, len(policy.Allow)+len(postSeccompStartupCalls))
	seccompAllowed = append(seccompAllowed, policy.seccompAllowedCalls()...)
	seccompAllowed = append(seccompAllowed, postSeccompStartupCalls...)

	return compiledSyscallPolicy{
		Ptrace: callPolicySpec{
			OneTimeCalls: append([]string(nil), policy.OneTime...),
			AllowedCalls: policy.ptraceAllowedCalls(),
			AuditCalls:   append([]string(nil), policy.Audit...),
		},
		SeccompAllowedCalls: uniqueSyscallNames(seccompAllowed),
		SeccompTracedCalls:  policy.seccompTracedCalls(),
	}
}

func (policy EffectiveSyscallPolicy) ptraceAllowedCalls() []string {
	allowed := make([]string, 0, len(policy.Allow)+len(policy.Trace)+len(policy.Audit))
	allowed = append(allowed, policy.Allow...)
	allowed = append(allowed, policy.Trace...)
	allowed = append(allowed, policy.Audit...)
	return uniqueSyscallNames(allowed)
}

func (policy EffectiveSyscallPolicy) seccompAllowedCalls() []string {
	return append([]string(nil), policy.Allow...)
}

func (policy EffectiveSyscallPolicy) seccompTracedCalls() []string {
	traced := make([]string, 0, len(policy.OneTime)+len(policy.Trace)+len(policy.Audit))
	traced = append(traced, policy.OneTime...)
	traced = append(traced, policy.Trace...)
	traced = append(traced, policy.Audit...)
	return uniqueSyscallNames(traced)
}

func validateSyscallPolicyOwnership(policy EffectiveSyscallPolicy) error {
	ownerByName := map[string]string{}
	for _, category := range []struct {
		name  string
		calls []string
	}{
		{"oneTimeCalls", policy.OneTime},
		{"runtime allowlist", policy.Allow},
		{"syscallPolicy.trace", policy.Trace},
		{"syscallPolicy.audit", policy.Audit},
	} {
		for _, call := range category.calls {
			if call == "" {
				continue
			}
			owner, exists := ownerByName[call]
			if exists {
				return fmt.Errorf("syscall %q is assigned to both %s and %s", call, owner, category.name)
			}
			ownerByName[call] = category.name
		}
	}

	for _, deniedCall := range policy.Deny {
		if owner, exists := ownerByName[deniedCall]; exists && owner != "runtime allowlist" {
			return fmt.Errorf("syscall %q cannot be both denied and %s", deniedCall, owner)
		}
	}
	return nil
}

func validateHybridSyscallPolicy(policy EffectiveSyscallPolicy) error {
	for _, check := range []struct {
		field string
		calls []string
	}{
		{"oneTimeCalls", policy.OneTime},
		{"syscallPolicy.deny", policy.Deny},
		{"syscallPolicy.trace", policy.Trace},
		{"syscallPolicy.audit", policy.Audit},
	} {
		for _, call := range check.calls {
			if containsSyscallName(postSeccompStartupCalls, call) {
				return fmt.Errorf("%s cannot include %q: syscall is reserved for hybrid startup protocol", check.field, call)
			}
		}
	}
	return nil
}

func uniqueSyscallNames(names []string) []string {
	seen := make(map[string]struct{}, len(names))
	unique := make([]string, 0, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		unique = append(unique, name)
	}
	return unique
}

func removeSyscallNames(names, remove []string) []string {
	if len(remove) == 0 {
		return append([]string(nil), names...)
	}

	removeSet := make(map[string]struct{}, len(remove))
	for _, name := range remove {
		removeSet[name] = struct{}{}
	}

	filtered := make([]string, 0, len(names))
	for _, name := range names {
		if _, shouldRemove := removeSet[name]; shouldRemove {
			continue
		}
		filtered = append(filtered, name)
	}
	return filtered
}

func containsSyscallName(names []string, target string) bool {
	for _, name := range names {
		if name == target {
			return true
		}
	}
	return false
}
