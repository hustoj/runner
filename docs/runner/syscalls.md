# Syscall Whitelist Reference

> **Source of truth**: The actual defaults are in [`runner/config.go`](../../runner/config.go)
> (`AllowedCalls` / `OneTimeCalls`) and the platform-specific defaults in
> `runner/defaults_linux_*.go`. Per-case overrides live in each `case.json`
> under `AdditionCalls`. This document is a human-readable summary.

## Default allowed syscalls (all platforms)

These are permitted by default via `AllowedCalls` in `TaskConfig`:

```
read, write, brk, fstat, uname, mmap,
exit_group, exit,
readlinkat, faccessat,
mprotect, set_tid_address, set_robust_list,
rseq, prlimit64, getrandom, rt_sigreturn
```

### One-time calls (default)

These are consumed on first use and then denied:

```
execve
```

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
