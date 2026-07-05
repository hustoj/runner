//go:build linux

package runner

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/hustoj/runner/sec"
	"golang.org/x/sys/unix"
)

const (
	seccompDataSyscallOffset = 0
	seccompDataArchOffset    = 4
)

type childSeccompSpec struct {
	enabled       bool
	filter        []unix.SockFilter
	traceSyscalls []uint32
}

func prepareChildSeccompSpec(setting *TaskConfig) (childSeccompSpec, error) {
	if setting.effectiveSyscallBackend() != syscallBackendHybrid {
		return childSeccompSpec{}, nil
	}

	policy, err := setting.compileSyscallPolicy()
	if err != nil {
		return childSeccompSpec{}, err
	}
	if err := validateHybridSyscallPolicy(setting.effectiveSyscallPolicy()); err != nil {
		return childSeccompSpec{}, err
	}
	allowedIDs, err := seccompSyscallIDs(policy.SeccompAllowedCalls)
	if err != nil {
		return childSeccompSpec{}, err
	}
	traceIDs, err := seccompSyscallIDs(policy.SeccompTracedCalls)
	if err != nil {
		return childSeccompSpec{}, err
	}
	oneTimeIDs, err := seccompSyscallIDs(policy.Ptrace.OneTimeCalls)
	if err != nil {
		return childSeccompSpec{}, err
	}
	if err := validateHybridTracePolicy(allowedIDs, traceIDs, oneTimeIDs); err != nil {
		return childSeccompSpec{}, err
	}
	filter, err := buildSeccompHybridFilter(allowedIDs, traceIDs)
	if err != nil {
		return childSeccompSpec{}, err
	}
	return childSeccompSpec{
		enabled:       true,
		filter:        filter,
		traceSyscalls: traceIDs,
	}, nil
}

func seccompSyscallIDs(names []string) ([]uint32, error) {
	seen := make(map[uint32]struct{}, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		id, err := sec.SCTbl.GetID(name)
		if err != nil {
			return nil, fmt.Errorf("seccomp syscall %q: %w", name, err)
		}
		seen[uint32(id)] = struct{}{}
	}

	ids := make([]uint32, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sortUint32(ids)
	return ids, nil
}

func validateHybridTracePolicy(allowedIDs, traceIDs, oneTimeIDs []uint32) error {
	for _, traceID := range traceIDs {
		if containsUint32(allowedIDs, traceID) {
			return fmt.Errorf("seccomp hybrid requires traced syscall %s(%d) to be trace-only; remove it from the runtime allowlist", getName(uint64(traceID)), traceID)
		}
	}

	execveID := uint32(syscall.SYS_EXECVE)
	if !containsUint32(oneTimeIDs, execveID) {
		return errors.New("seccomp hybrid requires execve in OneTimeCalls")
	}
	return nil
}

func buildSeccompHybridFilter(allowedSyscalls, tracedSyscalls []uint32) ([]unix.SockFilter, error) {
	if len(allowedSyscalls) == 0 && len(tracedSyscalls) == 0 {
		return nil, errors.New("seccomp policy cannot be empty")
	}

	auditArch, err := seccompAuditArch()
	if err != nil {
		return nil, err
	}

	filter := []unix.SockFilter{
		seccompStmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, seccompDataArchOffset),
		seccompJump(unix.BPF_JMP|unix.BPF_JEQ|unix.BPF_K, auditArch, 1, 0),
		seccompStmt(unix.BPF_RET|unix.BPF_K, unix.SECCOMP_RET_KILL_PROCESS),
		seccompStmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, seccompDataSyscallOffset),
	}
	filter = appendSeccompReturnRules(filter, allowedSyscalls, unix.SECCOMP_RET_ALLOW)
	filter = appendSeccompReturnRules(filter, tracedSyscalls, unix.SECCOMP_RET_TRACE)
	filter = append(filter, seccompStmt(unix.BPF_RET|unix.BPF_K, unix.SECCOMP_RET_KILL_PROCESS))
	if len(filter) > unix.BPF_MAXINSNS {
		return nil, fmt.Errorf("seccomp filter exceeds instruction limit: %d > %d", len(filter), unix.BPF_MAXINSNS)
	}
	return filter, nil
}

func appendSeccompReturnRules(filter []unix.SockFilter, syscallIDs []uint32, action uint32) []unix.SockFilter {
	for _, syscallID := range syscallIDs {
		filter = append(
			filter,
			seccompJump(unix.BPF_JMP|unix.BPF_JEQ|unix.BPF_K, syscallID, 0, 1),
			seccompStmt(unix.BPF_RET|unix.BPF_K, action),
		)
	}
	return filter
}

func seccompAuditArch() (uint32, error) {
	switch runtime.GOARCH {
	case "amd64":
		return unix.AUDIT_ARCH_X86_64, nil
	case "arm64":
		return unix.AUDIT_ARCH_AARCH64, nil
	default:
		return 0, fmt.Errorf("seccomp hybrid is not supported on linux/%s", runtime.GOARCH)
	}
}

func installSeccompFilter(spec childSeccompSpec) syscall.Errno {
	if !spec.enabled {
		return 0
	}
	if len(spec.filter) == 0 {
		return syscall.EINVAL
	}
	if len(spec.filter) > unix.BPF_MAXINSNS {
		return syscall.EINVAL
	}

	program := unix.SockFprog{
		Len:    uint16(len(spec.filter)),
		Filter: &spec.filter[0],
	}
	_, _, errno := syscall.RawSyscall6(
		syscall.SYS_PRCTL,
		uintptr(unix.PR_SET_SECCOMP),
		uintptr(unix.SECCOMP_MODE_FILTER),
		uintptr(unsafe.Pointer(&program)),
		0,
		0,
		0,
	)
	return errno
}

func seccompStmt(code, k uint32) unix.SockFilter {
	return unix.SockFilter{
		Code: uint16(code),
		K:    k,
	}
}

func seccompJump(code, k uint32, jt, jf uint8) unix.SockFilter {
	return unix.SockFilter{
		Code: uint16(code),
		Jt:   jt,
		Jf:   jf,
		K:    k,
	}
}

func sortUint32(values []uint32) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j-1] > values[j]; j-- {
			values[j-1], values[j] = values[j], values[j-1]
		}
	}
}

func containsUint32(values []uint32, target uint32) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
