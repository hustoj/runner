# 系统架构总览

本文是 `runner` 仓库的**入口文档**,提供全局视角:模块如何划分、三个可执行入口各自做什么、一次评测的数据流如何穿过整条调用链、安全机制建立在哪几层之上。

本文不重复各专题文档的深度细节,只在相关位置链接到它们。**设计文档不是唯一事实来源,真正的行为以 [`../runner/`](../runner/) 与 [`../sec/`](../sec/) 的源码为准。**

## 1. 项目定位

`github.com/hustoj/runner` 是 [HUSTOJ](https://github.com/hustoj/runner) 在线评测系统的**判题沙箱运行器**,用 Go 1.25 编写。核心职责是在强隔离、强资源限制的 Linux 环境中运行用户提交的程序,并以 [`case.json`](../tests/general/case.json) 描述的约束为依据输出判定结果(AC / TLE / MLE / OLE / RE / CE)。

- **完整运行平台**:仅 `linux/amd64` 与 `linux/arm64`。ptrace 跟踪器与系统调用表只在两个平台可用。
- **Darwin 仅用于开发**:`*_darwin.go` 文件保证包能在 macOS 上编译、做类型检查,运行时一律 panic 或返回 `not-supported`。
- **关键依赖**(见 [`../go.mod`](../go.mod)):`koding/multiconfig`(配置加载)、`google/shlex`(命令行切分)、`go.uber.org/zap`(日志)、`golang.org/x/sys/unix`(系统调用)、`stretchr/testify`(测试)。

## 2. 模块结构

```
cmd/runner      生产评测主入口
cmd/test        本地测试入口(对比期望结果)
cmd/compiler    编译沙箱入口(bootstrap 自举)
runner/         核心库 (package runner)
sec/            系统调用名↔号双向映射(纯数据层)
tests/          集成测试用例(每个目录一份 case.json + 源码 + makefile)
docker/         runner / compiler 镜像构建上下文
docs/           设计与迁移文档
```

`runner/` 内部按职责分组(平台相关文件以 build tag 区分,见下一节):

| 分组 | 文件 | 职责 |
|---|---|---|
| 配置 | [config.go](../runner/config.go) | `TaskConfig` 结构体、`Validate`、`LoadConfig` |
| | [config_linux.go](../runner/config_linux.go) / [config_darwin.go](../runner/config_darwin.go) | syscall 名称校验(Linux 实际校验 / Darwin no-op) |
| | [defaults_linux_amd64.go](../runner/defaults_linux_amd64.go) / [defaults_linux_arm64.go](../runner/defaults_linux_arm64.go) / [defaults_darwin.go](../runner/defaults_darwin.go) | 平台默认追加的 syscall |
| 编排 | [exec.go](../runner/exec.go) | `RunningTask`、`Init`/`Run`/`trace` 主循环、TLE/MLE 判定 |
| | [result.go](../runner/result.go) | `Result`、状态码常量、信号到结果码映射 |
| | [sec.go](../runner/sec.go) | `CallPolicy` 白名单策略 |
| | [tracer.go](../runner/tracer.go) | `TracerDetect`、每 tracee 相位状态 |
| 进程管理 | [process.go](../runner/process.go) | `Process`、多 tracee 的 `Wait`/`Kill`/`Memory` |
| | [process_linux.go](../runner/process_linux.go) / [process_darwin.go](../runner/process_darwin.go) | ptrace 选项与 stop 分类 |
| 子进程启动 | [exec_linux.go](../runner/exec_linux.go) / [exec_darwin.go](../runner/exec_darwin.go) | `runProcess`、`prepareChildProcessSpec`、`runChildProcess`、rlimit 设置 |
| | [fork_linux_amd64.go](../runner/fork_linux_amd64.go) / [fork_linux_arm64.go](../runner/fork_linux_arm64.go) | `fork()`(arm64 用 `clone` 模拟) |
| 沙箱 | [sandbox_linux.go](../runner/sandbox_linux.go) / [sandbox_darwin.go](../runner/sandbox_darwin.go) | namespace / chroot / 降权(**不是 seccomp**) |
| 内存采样 | [memory.go](../runner/memory.go) / [memory_linux.go](../runner/memory_linux.go) / [memory_darwin.go](../runner/memory_darwin.go) | `/proc/<pid>/status` 的 `VmHWM` 解析 |
| 寄存器 | [regs_linux_amd64.go](../runner/regs_linux_amd64.go) / [regs_linux_arm64.go](../runner/regs_linux_arm64.go) | 按架构读取 syscall 号与返回值 |
| | [tracer_linux.go](../runner/tracer_linux.go) / [tracer_darwin.go](../runner/tracer_darwin.go) | `checkSyscall` 拦截入口 |
| 工具 | [utils.go](../runner/utils.go) / [utils_linux.go](../runner/utils_linux.go) / [utils_darwin.go](../runner/utils_darwin.go) | IO 重定向、最小环境、bootstrap 启动 |
| 日志 | [log.go](../runner/log.go) / [global.go](../runner/global.go) | 包级 zap 句柄 |

## 3. 三层构建结构

`runner` 包通过 build tag 切成三层,让同一套平台无关骨架可以同时承载 Linux 实现与 Darwin 占位桩:

| 文件后缀 | build 条件 | 角色 |
|---|---|---|
| `*.go`(无后缀) | 全平台 | 平台无关骨架(结构体定义、主循环、解析逻辑) |
| `*_linux.go` | `linux` | Linux ptrace 实现 |
| `*_darwin.go` | `darwin` | panic / not-supported 占位,仅为可编译 |
| `*_linux_amd64.go` | `linux && amd64` | amd64 特定(寄存器、默认 syscall、fork) |
| `*_linux_arm64.go` | `linux && arm64` | arm64 特定(同上) |

Darwin 占位桩统一返回同一条消息(见 [utils_darwin.go](../runner/utils_darwin.go)):`darwin is supported only for development, type-checking, and builds; runtime execution requires linux`。

## 4. 三个可执行入口

| 入口 | 配置文件 | IO 来源 | 输出 |
|---|---|---|---|
| [cmd/runner](../cmd/runner/main.go) | `case.json` | 工作目录预先准备好的 `user.in` | `Result` 的 JSON 到 stdout |
| [cmd/test](../cmd/test/main.go) | `case.json` | 把 `Input`/`InputFile` 物化成 `user.in` | 与期望 `Result` 对比,打印 `passed!` / `failed!` |
| [cmd/compiler](../cmd/compiler/main.go) | `compile.json`(见 [cfg.go](../cmd/compiler/cfg.go)) | `compile.in/out/err` | `RunResult{Success}` |

`cmd/runner` 是生产评测主入口,33 行 main 串起 `LoadConfig → InitLogger → RunningTask.Init → Run → GetResult`,把 [`Result`](../runner/result.go) 序列化为 JSON。

`cmd/compiler` 采用 **bootstrap 自举模式**:父进程读取宿主侧 `compile.json` 后,把已解析配置序列化到 `RUNNER_COMPILER_BOOTSTRAP_CONFIG`,再通过 `os.StartProcess` 重新执行自身二进制并 `Setpgid`。子进程检测环境变量 `RUNNER_COMPILER_BOOTSTRAP=1` 后,不再二次读取 `compile.json`,而是从环境变量恢复配置,在 [compiler_linux.go](../cmd/compiler/compiler_linux.go) 里 `unshare namespaces` → `chroot` → `chdir` → `prctl(PR_SET_NO_NEW_PRIVS)` → `setgroups/setgid/setuid` → `setrlimits` → IO 重定向 → `syscall.Exec` 最终编译器,避免把 compiler 自身和配置文件复制进 jail。编译器的 rlimit(NPROC=32 / NOFILE=64 / CORE=0)与 runner(NPROC=16 / NOFILE=16 / CORE=0)不同。

编译器 sandbox 隔离由 bootstrap 子进程在 exec 编译器前自行完成:namespace(`CLONE_NEWNS/IPC/UTS/NET`)、chroot、WorkDir、`NoNewPrivs`、credential drop。配置 `ChrootDir` 且 `WorkDir` 为空时默认进入 chroot 内 `/`;非空 `WorkDir` 必须是绝对路径。**chroot 契约**:jail 内不需要包含 compiler 二进制或 `compile.json`,但必须包含最终编译命令所需的源码、`gcc/g++/fpc/javac`、动态链接器、标准库/头文件以及必要设备文件。`compile.json` 中通过 `RunUID`/`RunGID`/`ChrootDir`/`WorkDir`/`NoNewPrivs`/`UseNetNS` 等字段配置,默认全部关闭以保持向后兼容。Docker 镜像([docker/compiler/Dockerfile](../docker/compiler/Dockerfile))预创建了 `judger` 用户(UID=1536),生产环境应将 `RunUID`/`RunGID` 设为该用户以降权运行 gcc。

## 5. 执行主流程

以 `cmd/runner` 为例的完整调用链(行号见源码):

```
LoadConfig                  config.go:174    读 case.json → 追加平台默认 syscall → Validate
RunningTask.Init            exec.go:18       timeLimit = CPU×1e6µs, memoryLimit = Memory×1024 KB
RunningTask.Run             exec.go:30       runtime.LockOSThread → runProcess → trace
  runProcess                exec_linux.go:157  prepareChildProcessSpec + Pipe2 + fork + 读启动失败码
  trace                     exec.go:45      多 tracee 主循环(见下)
GetResult                   exec.go:39      返回 *Result,JSON 序列化到 stdout
```

### 5.1 子进程内部固定执行顺序

子进程在 fork 之后,按 [`childStartupStage`](../runner/exec_linux.go) 枚举(共 25 个阶段)依次执行,任何一步失败都通过启动管道回传 8 字节(stage + errno)给父进程后 `_exit(127)`,由 [`runChildProcess`](../runner/exec_linux.go) 实现:

```
setrlimit(CPU) → setitimer(alarm, CPU+5)
→ setrlimit(FSIZE / STACK / DATA / AS / NPROC / NOFILE / CORE)
→ dup(user.in → stdin, user.out → stdout, user.err → stderr)
→ setpgid(0, 0)
→ sandbox: unshare(namespaces) → chroot+chdir → no_new_privs → setgroups → setgid → setuid
→ PTRACE_TRACEME
→ execve
```

为什么这个顺序不可调整、以及为什么降权必须用 `RawSyscall` 而不是 Go 的 `syscall.Setuid`,详见 [sandbox-refactor-analysis.md](sandbox-refactor-analysis.md)。

### 5.2 trace 主循环的 stop 分类

[`trace()`](../runner/exec.go) 是多 tracee 主循环,每次 `Wait4` 拿到一个 stop 后按四类分支处理:

1. **进程退出**([exec.go:119](../runner/exec.go)):只有根 pid 的退出码决定结果;子线程非零退出码被忽略(兼容 JVM 守护线程)。
2. **attach-stop**([exec.go:134](../runner/exec.go)):fork/clone 自动附着的子进程,首次 `SIGSTOP`,给它设置 ptrace 选项后继续。
3. **ptrace event-stop**([exec.go:149](../runner/exec.go)):fork/clone/vfork 事件,注册新 tracee。
4. **signal-delivery-stop**([exec.go:161](../runner/exec.go)):用 `ContinueWithSignal` 转发信号(JVM 等运行时内部使用 SIGSEGV,必须转发)。
5. **syscall-stop**([exec.go:182](../runner/exec.go)):调 `checkSyscall`,不通过则 `Kill` + `RUNTIME_ERROR`。

ptrace 状态机的演进背景、相位模型和剩余风险详见 [ptrace-minimal-fix.md](ptrace-minimal-fix.md)。

## 6. 安全机制:hybrid syscall 过滤与三段式沙箱

当前默认仍是 ptrace-only；当 `SyscallBackend` 配置为 `hybrid` 时，runner 会在 Linux 子进程完成沙箱和 `ptrace(TRACEME)` 后、`execve` 前安装 seccomp-BPF 过滤器。seccomp 负责把普通 runtime syscall 过滤下沉到内核态，ptrace 保留 bootstrap `execve` 一次性语义、多 tracee 生命周期和按名单审计职责。Phase 2 已新增 `SyscallPolicy` 归一与编译层，把 legacy 白名单和结构化 `Allow` / `Deny` / `Trace` / `Audit` 合成同一份 effective policy，再编译成 ptrace 与 seccomp 各自消费的后端输入。迁移背景见 [seccomp-bpf-migration.md](seccomp-bpf-migration.md)。

### 6.1 syscall 白名单

- [`TracerDetect`](../runner/tracer.go) 为每个 tracee 维护 enter/exit 相位;只在 **syscall-enter stop** 查白名单。
- [`EffectiveSyscallPolicy`](../runner/syscall_policy.go) 先把 legacy `AllowedCalls` / `AdditionCalls` / `OneTimeCalls` 和结构化 `SyscallPolicy` 归一；`compileSyscallPolicy()` 再生成 ptrace 与 seccomp 的后端输入；[`CallPolicy`](../runner/sec.go) 只消费 ptrace 需要的具名 `callPolicySpec`。
- ptrace 选项(`process_linux.go`):默认是 `PTRACE_O_TRACESYSGOOD | PTRACE_O_EXITKILL | TRACE{CLONE,FORK,VFORK}`；hybrid 模式额外打开 `PTRACE_O_TRACESECCOMP`。ptrace-only 用 `PTRACE_SYSCALL` 获得 syscall enter/exit stop；hybrid 为性能和简单性使用 `PTRACE_CONT`，只接收 ptrace event、信号停靠和退出事件。
- hybrid 模式下，effective `Allow` 会生成 seccomp `ALLOW` 规则；`Deny` 先从 allow 结果里做减法，因此可覆盖默认或语言 fixture 继承来的 syscall；`OneTimeCalls + Trace + Audit` 生成 `SECCOMP_RET_TRACE` 规则，由 ptrace 审批或审计。普通禁止 syscall 会在内核态执行前被杀死，不再依赖 ptrace enter/exit 相位。

### 6.2 rlimit 内核兜底层

子进程在 exec 前设置一系列 rlimit(见 [第 5.1 节](#51-子进程内部固定执行顺序)与 [resource-limits.md](runner/resource-limits.md))。`prlimit64` 保留在默认白名单里([config.go:42](../runner/config.go))是为了兼容运行时查询 limits；runner 会拒绝 `new_rlim != NULL` 的 SET 操作，hybrid 模式也会把该调用转成 `SECCOMP_RET_TRACE` 后交给 ptrace 做参数检查。OpenJDK 21 默认提升 `RLIMIT_NOFILE` soft limit 的行为由 Java 命令参数 `-XX:-MaxFDLimit` 关闭，不通过放宽 runner 的 `prlimit64` SET 策略兼容。

### 6.3 namespace / chroot / 降权

[sandbox_linux.go](../runner/sandbox_linux.go) 实现的隔离:支持 `CLONE_NEWNS/IPC/UTS/NET`(PIDNS 当前被显式拒绝),chroot 到指定根目录,`PR_SET_NO_NEW_PRIVS`,以及 `setgroups → setgid → setuid` 降权。Linux 上 root 启动且未配置 `RunUID`/`RunGID` 时默认 fail-closed；测试或兼容场景必须显式设置 `AllowPrivilegedChild: true` 才能保留 root 子进程。详细设计与顺序依赖见 [sandbox-refactor-analysis.md](sandbox-refactor-analysis.md)。

## 7. 资源判杰契约

`runner` 同时维护**判题层**和**内核兜底层**,两层故意不是同一数值——判题层决定最终 TLE/MLE,内核兜底层在判题层来不及介入时兜住失控进程。

| 资源 | 判杰口径 | 内核兜底 |
|---|---|---|
| CPU | 所有 tracee 的 `utime + stime` 累加(**总 CPU,非 wall-clock**) | `RLIMIT_CPU = CPU+1`,`alarm = CPU+5` |
| Memory | `/proc/<pid>/status` 的 `VmHWM` 按 thread group 去重汇总 + rusage 兜底 + 扣除 root tracee 的 bootstrap RSS 基线 | `RLIMIT_DATA = RLIMIT_AS = Memory+MemoryReserve` |
| Output | — | `RLIMIT_FSIZE = Output`,SIGXFSZ → OUTPUT_LIMIT |
| Stack | — | `RLIMIT_STACK = Stack` |

- **CPU 是总 CPU 时间**:多线程并行时该值可能明显大于 wall-clock。
- **Memory 双来源**:[`refreshPeakMemoryFromProc`](../runner/exec.go) 遍历 active pid 读 `/proc/status` 的 `VmHWM`,按 Tgid 去重取每组峰值再求和;[`process.Memory`](../runner/process.go) 来自 `wait4` 的 `ru_maxrss`,按 thread group 聚合并扣除 bootstrap 基线偏移。两者取 `max` 作为 `PeakMemory`。
- **hybrid event-only 采样粒度较粗**:hybrid 不再在每个普通 syscall enter/exit stop 醒来，因此 `checkLimit` 只会在 ptrace event、信号停靠、退出等路径运行；最终判定仍依赖 `RLIMIT_CPU` / `alarm` / cgroup v2 memory 状态兜底。

完整字段语义、双层关系与事件归类见 [runner/resource-limits.md](runner/resource-limits.md)。

## 8. 系统调用表与白名单策略

[`sec/`](../sec/) 是**纯数据层**,只做名称↔编号双向映射,不含任何 allow/deny 逻辑:

- **amd64**([syscalls.go](../sec/syscalls.go)):字符串表,从 `read`=0 到 `rseq_slice_yield`=471,覆盖 Linux 6.x。
- **arm64**([syscalls_arm64.go](../sec/syscalls_arm64.go)):直接用 `unix.SYS_*` 常量 map,末尾几个 6.x 新号因 `x/sys` 尚未定义而硬编码。
- 其它平台([stub_other.go](../sec/stub_other.go)):返回 "only available on linux/amd64 or linux/arm64"。

策略语义先在 [`runner/syscall_policy.go`](../runner/syscall_policy.go) 归一，再交给 [`runner/sec.go`](../runner/sec.go) 的 `CallPolicy` 或 [`runner/seccomp_linux.go`](../runner/seccomp_linux.go) 的 seccomp filter 生成器:

- 默认 `AllowedCalls`([config.go:42](../runner/config.go))非常保守:`read,write,brk,fstat,uname,mmap,exit_group,exit,readlinkat,faccessat,mprotect,set_tid_address,set_robust_list,rseq,prlimit64,getrandom,rt_sigreturn`；其中 `prlimit64` 是参数过滤调用，仅允许查询，拒绝 SET。
- 默认 `OneTimeCalls`:仅 `execve`。
- Java 等运行时通过 `AdditionCalls` 追加约 40 个,示例见 [`tests/java/case.json`](../tests/java/case.json)。
- amd64 平台默认追加 `arch_prctl/readlink/access`,arm64 不追加。
- `SyscallPolicy.Allow` 追加 runtime allow；`SyscallPolicy.Deny` 覆盖 allow 结果；`SyscallPolicy.Trace` 在 hybrid 下转成 seccomp trace 规则；`SyscallPolicy.Audit` 同样转成 trace 规则，并额外输出 audit 日志。`Deny` 不能和 `OneTimeCalls` / `Trace` / `Audit` 重叠；hybrid 启动协议保留 `write` / `close` / `exit` / `exit_group`，这些 syscall 不能放入 `Deny` / `Trace` / `Audit` / `OneTimeCalls`。

可读的 syscall 清单与 Java 案例见 [runner/syscalls.md](runner/syscalls.md)。

## 9. 测试体系

### 9.1 单元测试(Go `testing` + testify)

- `runner/*_test.go`:覆盖 tracer 相位、process 的多 tracee 竞态、memory 解析、config 校验、sandbox 配置。
- [process_linux_test.go](../runner/process_linux_test.go):pendingStops 竞态(clone event 子进程 stop 可能在注册前到达)、thread group 聚合、bootstrap offset 扣减。
- [sec/syscalls_test.go](../sec/syscalls_test.go):交叉校验表与内核常量、新内核 syscall 覆盖、架构差异。
- [sandbox_behavior_linux_test.go](../runner/sandbox_behavior_linux_test.go):行为级集成测试,通过 `runProcess()` 启动真实子进程读 `/proc/<pid>/status` 断言 NoNewPrivs、UID/GID 切换、mount namespace、chroot(部分用例需 root)。

### 9.2 集成测试(`tests/`,23 个 case)

每个 case = `case.json` + 源码(`.c`/`.java`)+ `makefile`(`gcc -static → ../../bin/test → clean`)。`make testall`([Makefile](../Makefile))依次进入每个目录执行。分类:

| 分类 | cases |
|---|---|
| 正常 | `general` |
| 资源限制 | `tle` `tle2` `mle` `mle2` `mle21` `mle3` `ole` `prlimit-ole` `stack` |
| 运行时异常 | `segmentfault` `sigtrap` `zero` |
| 安全/多进程 | `clone-syscall-phase` `clone-syscall-phase-hybrid` `fork` `socket` `syscall-policy-deny-hybrid` `syscall-policy-trace-audit-hybrid` `thread` |
| Java | `java` `java-tle` `java-mle` |

`make testall` 需要 C/C++ 工具链(静态链接)、GNU Make,可选 Java(`tests/java*`)。完整前提与平台支持矩阵见 [README.md](../README.md)。

## 10. 构建与部署

- [`make`](../Makefile) → `bin/runner`;`make compiler` → `bin/compile`;`make prepare` → `bin/test`。
- Docker(见 [docker/README.md](../docker/README.md)):
  - `docker/runner`:Ubuntu + openjdk-8-jre + runner 二进制,`WORKDIR /data`,`VOLUME /var/log/runner`。
  - `docker/compiler`:Ubuntu + gcc/g++/fp-compiler/openjdk-8-jdk + compiler 二进制。
- pre-commit([`.pre-commit-config.yaml`](../.pre-commit-config.yaml)):`go-fmt` + `go-mod-tidy` + `golangci-lint v2.11.3` + 标准 hooks。GitHub Actions 在 push/PR 时自动运行。

## 11. 关键类型与函数索引

### 核心类型

| 类型 | 定义位置 | 用途 |
|---|---|---|
| `TaskConfig` | [config.go:14](../runner/config.go) | 评测任务配置 |
| `RunningTask` | [exec.go:10](../runner/exec.go) | 一次运行的编排器 |
| `Result` | [result.go:26](../runner/result.go) | 输出结果(JSON 序列化) |
| `Process` | [process.go](../runner/process.go) | 被跟踪进程树状态 |
| `TracerDetect` | [tracer.go](../runner/tracer.go) | syscall 跟踪 / 策略执行器 |
| `CallPolicy` | [sec.go:9](../runner/sec.go) | one-time + allowed 调用集合 |
| `SandboxConfig` | [sandbox_linux.go](../runner/sandbox_linux.go) / [sandbox_darwin.go](../runner/sandbox_darwin.go) | 沙箱参数 |
| `ProcMemoryInfo` | [memory.go](../runner/memory.go) | `/proc/status` 解析结果 |
| `childProcessSpec` | [exec_linux.go:121](../runner/exec_linux.go) | 子进程启动参数包 |
| `childStartupStage` | [exec_linux.go:16](../runner/exec_linux.go) | 25 个启动阶段枚举 |

### 核心函数(按调用顺序)

| 函数 | 位置 |
|---|---|
| `LoadConfig` | [config.go:174](../runner/config.go) |
| `RunningTask.Init` | [exec.go:18](../runner/exec.go) |
| `RunningTask.Run` | [exec.go:30](../runner/exec.go) |
| `runProcess` | [exec_linux.go:157](../runner/exec_linux.go) |
| `prepareChildProcessSpec` | [exec_linux.go:199](../runner/exec_linux.go) |
| `fork` | [fork_linux_amd64.go](../runner/fork_linux_amd64.go) / [fork_linux_arm64.go](../runner/fork_linux_arm64.go) |
| `runChildProcess` | [exec_linux.go:363](../runner/exec_linux.go) |
| `applySandboxInChild` | [sandbox_linux.go](../runner/sandbox_linux.go) |
| `trace` | [exec.go:45](../runner/exec.go) |
| `makeCallPolicy` | [sec.go:14](../runner/sec.go) |
| `checkSyscall` | [tracer_linux.go](../runner/tracer_linux.go) |
| `CallPolicy.CheckID` | [sec.go:38](../runner/sec.go) |
| `refreshPeakMemoryFromProc` | [exec.go:348](../runner/exec.go) |
| `Process.Memory` | [process.go](../runner/process.go) |
| `Process.Kill` | [process.go](../runner/process.go) |
| `detectSignal` | [result.go:42](../runner/result.go) |

## 12. 结果状态码

定义在 [result.go:9-24](../runner/result.go):

```
PENDING=0, PENDING_REJUDGE=1, COMPILING=2, REJUDGING=3,
ACCEPT=4, PRESENTATION_ERROR=5, WRONG_ANSWER=6,
TIME_LIMIT=7, MEMORY_LIMIT=8, OUTPUT_LIMIT=9,
RUNTIME_ERROR=10, COMPILE_ERROR=11, COMPILE_OK=12, TEST_RUN=13
```

信号到结果码的直接映射:`SIGALRM`/`SIGXCPU` → `TIME_LIMIT`,`SIGXFSZ` → `OUTPUT_LIMIT`,其它信号 → `RUNTIME_ERROR`。`TaskConfig.Result` 默认 `4`(期望 ACCEPT)。

## 13. 已知边界与后续方向

以下均为既有文档已记录的事项,此处仅做导航:

- **seccomp 迁移**:当前已支持显式 `SyscallBackend: "hybrid"`，默认仍为 ptrace-only。后续纯 seccomp 方向见 [seccomp-bpf-migration.md](seccomp-bpf-migration.md)。主要难点仍是 `execve once` 语义与 seccomp allowlist 不兼容。
- **ptrace 多 tracee 剩余风险**(资源统计仍是近似模型):见 [ptrace-minimal-fix.md](ptrace-minimal-fix.md)。
- **资源限制待验证项**(`MemoryReserve` 余量是否覆盖各语言初始化、短生命周期 MLE 能否被 `/proc` 采样捕获、Java 多线程下 thread group 聚合准确性、`RLIMIT_AS` 过紧致 mmap 失败):见 [todo.md](todo.md)。
- **平台支持边界**(其它 Linux 架构可编译但启动失败,arm64 集成测试需在目标主机跑):见 [README.md](../README.md) 的 Platform Support 一节。

## 14. 相关文档导航

- 想理解 ptrace 状态机的历史问题与多 tracee 演进 → [ptrace-minimal-fix.md](ptrace-minimal-fix.md)
- 想理解 namespace / chroot / 降权顺序的设计 → [sandbox-refactor-analysis.md](sandbox-refactor-analysis.md)
- 想看 hybrid syscall 过滤和后续纯 seccomp 方向 → [seccomp-bpf-migration.md](seccomp-bpf-migration.md)
- 想查运行期资源限制字段与判题口径 → [runner/resource-limits.md](runner/resource-limits.md)
- 想查运行期 syscall 白名单实践与 Java 案例 → [runner/syscalls.md](runner/syscalls.md)
- 想看待验证事项清单 → [todo.md](todo.md)
