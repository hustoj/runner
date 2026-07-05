//go:build linux

package runner

import (
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

func TestSeccompSyscallIDsDeduplicatesAndSorts(t *testing.T) {
	ids, err := seccompSyscallIDs([]string{"write", "read", "read"})
	if err != nil {
		t.Fatalf("seccompSyscallIDs() error = %v", err)
	}

	want := []uint32{uint32(syscall.SYS_READ), uint32(syscall.SYS_WRITE)}
	if len(ids) != len(want) {
		t.Fatalf("seccompSyscallIDs() = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("seccompSyscallIDs()[%d] = %d, want %d; full=%v", i, ids[i], want[i], ids)
		}
	}
}

func TestPrepareChildSeccompSpecIncludesStartupFailureReportingCalls(t *testing.T) {
	spec, err := prepareChildSeccompSpec(&TaskConfig{
		SyscallBackend: syscallBackendHybrid,
		OneTimeCalls:   []string{"execve"},
		AllowedCalls:   []string{"read"},
	})
	if err != nil {
		t.Fatalf("prepareChildSeccompSpec() error = %v", err)
	}
	if !spec.enabled {
		t.Fatal("prepareChildSeccompSpec() enabled = false, want true")
	}
	if !containsUint32(spec.traceSyscalls, uint32(syscall.SYS_EXECVE)) {
		t.Fatalf("prepareChildSeccompSpec() traceSyscalls = %v, want execve", spec.traceSyscalls)
	}

	filter := spec.filter
	for _, syscallID := range []uint32{
		uint32(syscall.SYS_WRITE),
		uint32(syscall.SYS_CLOSE),
		uint32(syscall.SYS_EXIT),
		uint32(syscall.SYS_EXIT_GROUP),
	} {
		if !seccompFilterHasSyscall(filter, syscallID) {
			t.Fatalf("seccomp filter does not allow startup reporting syscall %d", syscallID)
		}
	}
	action, ok := seccompFilterActionForSyscall(filter, uint32(syscall.SYS_EXECVE))
	if !ok {
		t.Fatal("seccomp filter has no execve rule")
	}
	if action != unix.SECCOMP_RET_TRACE {
		t.Fatalf("execve seccomp action = %#x, want TRACE", action)
	}
}

func TestPrepareChildSeccompSpecTracesStructuredPolicyCalls(t *testing.T) {
	spec, err := prepareChildSeccompSpec(&TaskConfig{
		SyscallBackend: syscallBackendHybrid,
		OneTimeCalls:   []string{"execve"},
		AllowedCalls:   []string{"read"},
		SyscallPolicy: SyscallPolicyConfig{
			Trace: []string{"getpid"},
			Audit: []string{"getppid"},
		},
	})
	if err != nil {
		t.Fatalf("prepareChildSeccompSpec() error = %v", err)
	}

	for _, syscallID := range []uint32{
		uint32(syscall.SYS_EXECVE),
		uint32(syscall.SYS_GETPID),
		uint32(syscall.SYS_GETPPID),
	} {
		action, ok := seccompFilterActionForSyscall(spec.filter, syscallID)
		if !ok {
			t.Fatalf("seccomp filter has no rule for syscall %d", syscallID)
		}
		if action != unix.SECCOMP_RET_TRACE {
			t.Fatalf("syscall %d seccomp action = %#x, want TRACE", syscallID, action)
		}
	}

	action, ok := seccompFilterActionForSyscall(spec.filter, uint32(syscall.SYS_READ))
	if !ok {
		t.Fatal("seccomp filter has no read rule")
	}
	if action != unix.SECCOMP_RET_ALLOW {
		t.Fatalf("read seccomp action = %#x, want ALLOW", action)
	}
}

func TestPrepareChildSeccompSpecRemovesDeniedAllowedCalls(t *testing.T) {
	spec, err := prepareChildSeccompSpec(&TaskConfig{
		SyscallBackend: syscallBackendHybrid,
		OneTimeCalls:   []string{"execve"},
		AllowedCalls:   []string{"read", "write"},
		SyscallPolicy: SyscallPolicyConfig{
			Allow: []string{"fstat"},
			Deny:  []string{"read", "fstat"},
		},
	})
	if err != nil {
		t.Fatalf("prepareChildSeccompSpec() error = %v", err)
	}

	for _, syscallID := range []uint32{uint32(syscall.SYS_READ), uint32(syscall.SYS_FSTAT)} {
		if action, ok := seccompFilterActionForSyscall(spec.filter, syscallID); ok {
			t.Fatalf("denied syscall %d has seccomp action %#x, want no allow/trace rule", syscallID, action)
		}
	}
	action, ok := seccompFilterActionForSyscall(spec.filter, uint32(syscall.SYS_WRITE))
	if !ok {
		t.Fatal("seccomp filter has no write rule")
	}
	if action != unix.SECCOMP_RET_ALLOW {
		t.Fatalf("write seccomp action = %#x, want ALLOW", action)
	}
}

func TestPrepareChildSeccompSpecRejectsDeniedStartupProtocolCalls(t *testing.T) {
	_, err := prepareChildSeccompSpec(&TaskConfig{
		SyscallBackend: syscallBackendHybrid,
		OneTimeCalls:   []string{"execve"},
		AllowedCalls:   []string{"read"},
		SyscallPolicy:  SyscallPolicyConfig{Deny: []string{"write"}},
	})
	if err == nil {
		t.Fatal("prepareChildSeccompSpec() error = nil, want startup protocol denial rejection")
	}
	if !strings.Contains(err.Error(), "hybrid startup protocol") {
		t.Fatalf("prepareChildSeccompSpec() error = %q, want startup protocol context", err)
	}
}

func TestPrepareChildSeccompSpecRejectsDenyTraceOverlap(t *testing.T) {
	_, err := prepareChildSeccompSpec(&TaskConfig{
		SyscallBackend: syscallBackendHybrid,
		OneTimeCalls:   []string{"execve"},
		AllowedCalls:   []string{"read"},
		SyscallPolicy: SyscallPolicyConfig{
			Trace: []string{"getpid"},
			Deny:  []string{"getpid"},
		},
	})
	if err == nil {
		t.Fatal("prepareChildSeccompSpec() error = nil, want deny/trace overlap rejection")
	}
}

func TestPrepareChildSeccompSpecRejectsPermanentExecveAllow(t *testing.T) {
	_, err := prepareChildSeccompSpec(&TaskConfig{
		SyscallBackend: syscallBackendHybrid,
		OneTimeCalls:   []string{"execve"},
		AllowedCalls:   []string{"read", "execve"},
	})
	if err == nil {
		t.Fatal("prepareChildSeccompSpec() error = nil, want execve allowlist rejection")
	}
}

func TestPrepareChildSeccompSpecRejectsAnyOneTimeAllowOverlap(t *testing.T) {
	_, err := prepareChildSeccompSpec(&TaskConfig{
		SyscallBackend: syscallBackendHybrid,
		OneTimeCalls:   []string{"execve", "getpid"},
		AllowedCalls:   []string{"read"},
		AdditionCalls:  []string{"getpid"},
	})
	if err == nil {
		t.Fatal("prepareChildSeccompSpec() error = nil, want one-time overlap rejection")
	}
}

func TestPrepareChildSeccompSpecRequiresExecveOneTime(t *testing.T) {
	_, err := prepareChildSeccompSpec(&TaskConfig{
		SyscallBackend: syscallBackendHybrid,
		AllowedCalls:   []string{"read"},
	})
	if err == nil {
		t.Fatal("prepareChildSeccompSpec() error = nil, want missing execve one-time rejection")
	}
}

func TestPrepareChildSeccompSpecRequiresExecveInOneTimeCalls(t *testing.T) {
	_, err := prepareChildSeccompSpec(&TaskConfig{
		SyscallBackend: syscallBackendHybrid,
		OneTimeCalls:   []string{"getpid"},
		AllowedCalls:   []string{"read"},
		SyscallPolicy:  SyscallPolicyConfig{Trace: []string{"execve"}},
	})
	if err == nil {
		t.Fatal("prepareChildSeccompSpec() error = nil, want execve one-time rejection")
	}
}

func TestBuildSeccompHybridFilterRejectsTooManyInstructions(t *testing.T) {
	allowedSyscalls := make([]uint32, (unix.BPF_MAXINSNS-5)/2+1)
	_, err := buildSeccompHybridFilter(allowedSyscalls, nil)
	if err == nil {
		t.Fatal("buildSeccompHybridFilter() error = nil, want instruction limit rejection")
	}
	if !strings.Contains(err.Error(), "seccomp filter exceeds") {
		t.Fatalf("buildSeccompHybridFilter() error = %q, want instruction limit context", err)
	}
}

func TestInstallSeccompFilterRejectsTooManyInstructions(t *testing.T) {
	errno := installSeccompFilter(childSeccompSpec{
		enabled: true,
		filter:  make([]unix.SockFilter, unix.BPF_MAXINSNS+1),
	})
	if errno != syscall.EINVAL {
		t.Fatalf("installSeccompFilter() errno = %v, want EINVAL", errno)
	}
}

func TestBuildSeccompAllowlistFilterRejectsWrongArchBeforeSyscallAllowlist(t *testing.T) {
	filter, err := buildSeccompHybridFilter([]uint32{uint32(syscall.SYS_READ)}, []uint32{uint32(syscall.SYS_EXECVE)})
	if err != nil {
		t.Fatalf("buildSeccompHybridFilter() error = %v", err)
	}

	if len(filter) < 5 {
		t.Fatalf("filter len = %d, want at least 5", len(filter))
	}
	if filter[0].Code != uint16(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS) || filter[0].K != seccompDataArchOffset {
		t.Fatalf("filter[0] = %#v, want load arch", filter[0])
	}
	if filter[1].Code != uint16(unix.BPF_JMP|unix.BPF_JEQ|unix.BPF_K) || filter[1].Jt != 1 || filter[1].Jf != 0 {
		t.Fatalf("filter[1] = %#v, want arch equality jump", filter[1])
	}
	if filter[2].Code != uint16(unix.BPF_RET|unix.BPF_K) || filter[2].K != unix.SECCOMP_RET_KILL_PROCESS {
		t.Fatalf("filter[2] = %#v, want kill wrong arch", filter[2])
	}
	if filter[3].Code != uint16(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS) || filter[3].K != seccompDataSyscallOffset {
		t.Fatalf("filter[3] = %#v, want load syscall nr", filter[3])
	}
	if filter[len(filter)-1].K != unix.SECCOMP_RET_KILL_PROCESS {
		t.Fatalf("last filter = %#v, want default kill", filter[len(filter)-1])
	}
}

func seccompFilterHasSyscall(filter []unix.SockFilter, syscallID uint32) bool {
	_, ok := seccompFilterActionForSyscall(filter, syscallID)
	return ok
}

func seccompFilterActionForSyscall(filter []unix.SockFilter, syscallID uint32) (uint32, bool) {
	for i := 0; i < len(filter)-1; i++ {
		instruction := filter[i]
		if instruction.Code == uint16(unix.BPF_JMP|unix.BPF_JEQ|unix.BPF_K) && instruction.K == syscallID {
			return filter[i+1].K, true
		}
	}
	return 0, false
}
