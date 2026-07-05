//go:build linux

package runner

import (
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
