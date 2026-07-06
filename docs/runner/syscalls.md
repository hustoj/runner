# Syscall Whitelist Reference

> **Source of truth**: The actual defaults are in [`runner/config.go`](../../runner/config.go)
> (`AllowedCalls` / `OneTimeCalls`) and the platform-specific defaults in
> `runner/defaults_linux_*.go`. Per-case overrides live in each `case.json`
> under legacy `AdditionCalls` or structured `SyscallPolicy`. This document is a human-readable summary.
> With `SyscallBackend: "hybrid"`, Linux turns the effective runtime allowlist
> into seccomp-BPF `ALLOW` rules installed before the user program starts.
> `OneTimeCalls` such as bootstrap `execve`, plus `SyscallPolicy.Trace` /
> `SyscallPolicy.Audit`, use `SECCOMP_RET_TRACE`, so ptrace remains the startup
> boundary and named audit path instead of permanently allowing one-time calls.
> Hybrid uses event-only ptrace resume for performance, so ordinary seccomp
> `ALLOW` calls do not produce ptrace syscall enter/exit stops. Syscalls that
> require argument filtering, currently `prlimit64`, are compiled as seccomp
> `TRACE` even when they come from the runtime allowlist.

## Default allowed syscalls (runtime-supported Linux platforms)

These base defaults are permitted via `AllowedCalls` in `TaskConfig` on Linux before any platform-specific additions:

```
read, write, brk, fstat, uname, mmap,
exit_group, exit,
readlinkat, faccessat,
mprotect, set_tid_address, set_robust_list,
rseq, prlimit64, getrandom, rt_sigreturn
```

`prlimit64` is query-only: calls with `new_rlim == NULL` are allowed, while
SET operations (`new_rlim != NULL`) are rejected before execution.

### One-time calls (default)

These are consumed on first use and then denied:

```
execve
```

### Structured policy fields

`SyscallPolicy` is the Phase 2 compatibility layer over the legacy lists:

```json
{
  "SyscallPolicy": {
    "Allow": ["read"],
    "Deny": ["ptrace", "bpf"],
    "Trace": ["clone3"],
    "Audit": ["getpid"]
  }
}
```

- `Allow` is merged with legacy `AllowedCalls + AdditionCalls`.
- `Deny` overrides that merged allowlist, so cases can remove syscalls inherited from defaults or language fixtures.
- `Trace` is permitted under ptrace-only, and becomes a seccomp `TRACE` rule under hybrid.
- `Audit` follows the same allow/trace backend mapping as `Trace`, and additionally emits an audit log entry when observed; it is named observation, not full syscall-stream replay.
- `Deny` may overlap allow rules because it subtracts from them, but it cannot overlap `Trace`, `Audit`, or `OneTimeCalls`.
- Under hybrid, `write`, `close`, `exit`, and `exit_group` are reserved by the runner startup protocol. They may be runtime-allowed, but cannot be placed in `Deny`, `Trace`, `Audit`, or `OneTimeCalls`.

The runner first normalizes these lists into an effective policy, then compiles
that policy into backend inputs. ptrace receives a `callPolicySpec`; hybrid
seccomp receives final `ALLOW` and `TRACE` syscall lists.

### Platform-specific additions

**linux/amd64** adds:

```
arch_prctl, readlink, access
```

**linux/arm64**: no additional defaults.

## Java example (`tests/java/case.json`)

Java programs require a significantly larger syscall surface. The current
Java test case adds these via `AdditionCalls`:

```
clock_getres, clock_nanosleep, clone3, close, connect,
faccessat2, fchdir, fcntl, flock, ftruncate, futex,
getcwd, getdents64, geteuid, getpid, getrusage, gettid, getuid,
ioctl, kill, lseek, madvise, mkdir, munmap,
newfstatat, openat, prctl, pread64,
rt_sigaction, rt_sigprocmask,
sched_getaffinity, sched_yield,
socket, stat, statfs, sysinfo, unlink
```

This list was derived from tracing OpenJDK 17+ on linux/amd64. Other JVM
versions or architectures may require adjustments.

The syscall list alone is not sufficient for current OpenJDK builds. The JVM
also creates more than the default `MaxProcs: 16` threads during startup, so
the Java fixture raises `MaxProcs` explicitly.
