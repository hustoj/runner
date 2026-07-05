# seccomp-bpf 迁移草案

> 当前代码已落地 Phase 1 的 hybrid 版本，并推进了 Phase 2 的兼容式策略归一层：默认仍是 `ptrace`，显式配置 `SyscallBackend: "hybrid"` 时会在 Linux child 中安装 seccomp-BPF 过滤器；普通 runtime allowlist 走 `ALLOW`，`OneTimeCalls` 走 `SECCOMP_RET_TRACE` 交给 ptrace 做启动边界判断。`SyscallPolicy` 已提供 `Allow` / `Deny` / `Trace` / `Audit` 四个显式桶，但旧的 `AllowedCalls` / `AdditionCalls` / `OneTimeCalls` 仍保持兼容。纯 seccomp 仍是后续方向。

## 目标

把 syscall 过滤的安全边界从用户态 ptrace 下沉到内核态 seccomp-bpf，同时尽量复用当前 runner 的配置模型和测试资产。

这份草案的目标不是直接替换全部运行逻辑，而是给出一条低风险迁移路径；当前实现覆盖了第一阶段的工程接缝。

## 当前现状

当前 runner 的 syscall 控制面有三个关键特点：

1. 兼容白名单来自 [runner/config.go](../runner/config.go) 的 `AllowedCalls` 和 `AdditionCalls`，结构化新增规则来自 `SyscallPolicy.Allow` / `Deny` / `Trace` / `Audit`。
2. 一次性 syscall 来自 [runner/config.go](../runner/config.go) 的 `OneTimeCalls`，默认包含 `execve`。
3. ptrace-only 模式的最终判定仍发生在 [runner/tracer_linux.go](../runner/tracer_linux.go) 的 ptrace loop 中；hybrid 模式下普通禁止 syscall 由 seccomp 在执行前拒绝，ptrace 只处理 `SECCOMP_RET_TRACE` 事件和生命周期事件。

这个模型的主要问题是：

1. syscall 过滤在用户态，性能和复杂度都不理想。
2. 真实 signal 和 ptrace event 容易把状态机搞复杂。
3. `execve` 这种“一次性允许”语义，并不适合直接映射到传统 seccomp allowlist。

## 为什么不建议一步切成纯 seccomp

最大的阻力不是 allowlist 本身，而是 `execve` 启动语义。

当前 runner 在子进程里完成：

1. 等待父进程将 child 加入 task cgroup（cgroup gate）
2. rlimit（cpu / output / stack / nofile / core）
3. IO 重定向
4. setpgid
5. sandbox 初始化（namespace → rootfs → no_new_privs → 凭证切换）
6. `ptrace(TRACEME)`
7. `execve` 进入用户程序

如果在 `execve` 之前安装 seccomp 过滤器，那么过滤器会穿过 `execve` 继承到用户程序。这意味着：

1. 你必须允许这次 bootstrap `execve`
2. 但一旦允许，用户程序后续再次 `execve` 也会被同一规则放行

经典 seccomp-bpf 适合表达“这个 syscall 永远允许/拒绝”，不擅长表达“只允许这一次 execve”。

所以更稳的迁移路线不是“立刻删掉 ptrace”，而是先进入 hybrid 模式。

## 建议路线

### Phase 1: Hybrid 模式

目标：把大部分 syscall 过滤下沉到 seccomp，ptrace 暂时只保留为启动边界和审计辅助。

建议做法：

1. 保留当前最小修复后的 ptrace 初始化流程。
2. 在子进程完成 namespace、rootfs、no_new_privs、凭证切换之后安装 seccomp 过滤器。当前首版 hybrid 实现将安装点放在 `ptrace(TRACEME)` 之后、`execve` 之前，这样 runtime seccomp allowlist 不需要放行用户态 `ptrace`。
3. seccomp 规则负责稳定的 runtime allowlist，例如 `read`、`write`、`mmap`、`brk`、`exit_group` 这类通用 syscall。
4. ptrace 暂时只处理两类场景：
   - bootstrap `execve` 的边界语义：BPF 对 `OneTimeCalls` 返回 `SECCOMP_RET_TRACE`，parent 在 `PTRACE_EVENT_SECCOMP` 上确认这次启动期 `execve`
   - 迁移期按 `Trace` / `Audit` 名单做审计和抽样对照

这样做的好处是：

1. 大部分危险 syscall 在内核态直接被拒绝。
2. 可以逐步收缩 ptrace 的职责，而不是一次性替换所有行为。
3. 现有测试可以复用，并且能做 ptrace-only 与 hybrid 的结果对照。

当前 hybrid 明确选择 event-only resume：tracee 用 `PTRACE_CONT` 继续运行，ptrace 只接收 fork/clone/vfork 生命周期事件、`PTRACE_EVENT_SECCOMP`、信号停靠和退出事件；它不再通过 `PTRACE_SYSCALL` 观察普通 syscall 的 enter/exit stop。因此，“迁移期对照”不是全量 ptrace 复核 seccomp `ALLOW` 调用，而是通过 `Trace` / `Audit` 把需要观测的 syscall 转成 seccomp trace event。

### Phase 2: 配置模型重构

目标：把当前配置从“ptrace 时代的 syscall 名单”重构成“seccomp 时代的策略模型”。

当前实现已经新增一个兼容式显式策略层：

```json
{
  "SyscallPolicy": {
    "Allow": ["read", "write", "mmap", "brk"],
    "Deny": ["socket", "connect", "bpf", "ptrace"],
    "Trace": ["clone", "fork", "vfork"],
    "Audit": ["getpid"]
  }
}
```

这里的含义是：

1. `Allow` 进入 runtime allowlist；hybrid 模式下生成 seccomp `ALLOW` 规则。
2. `Trace` 在 ptrace-only 模式下作为允许规则通过 ptrace；hybrid 模式下生成 `SECCOMP_RET_TRACE` 规则，交给 ptrace 处理事件。
3. `Audit` 的放行方式和 `Trace` 一样，但会额外输出独立 audit 日志，用于迁移期按名单抽样观测。
4. `Deny` 会从 legacy `AllowedCalls + AdditionCalls` 和 `SyscallPolicy.Allow` 的合并结果中删除对应 syscall；在 hybrid 模式下落到 seccomp 默认 `KILL_PROCESS`，在 ptrace-only 模式下落到默认拒绝。
5. `OneTimeCalls` 仍保留启动边界语义，默认 `execve`；`Deny` 不能和 `OneTimeCalls` / `Trace` / `Audit` 重叠。
6. hybrid 下 `write` / `close` / `exit` / `exit_group` 属于 runner 启动协议保留 syscall。它们可以被 runtime allow，但不能放入 `Deny` / `Trace` / `Audit` / `OneTimeCalls`，否则 child 在 seccomp 安装后可能无法回报启动失败或正常退出。
7. `AllowedCalls` 和 `AdditionCalls` 作为 legacy runtime allowlist 继续兼容，并参与同一份 effective policy。

#### Breaking 行为：syscall ownership fail-loud

当前实现有意收紧 syscall policy 的 ownership 契约：同一个 syscall 不能同时属于 `OneTimeCalls`、runtime allowlist（`AllowedCalls` / `AdditionCalls` / `SyscallPolicy.Allow`）、`SyscallPolicy.Trace`、`SyscallPolicy.Audit`。`SyscallPolicy.Deny` 只允许覆盖 runtime allowlist，不能和 `OneTimeCalls` / `Trace` / `Audit` 重叠。

这个校验在默认 `ptrace` 路径同样生效，不只影响 `hybrid`。因此，旧版本中曾经合法的 `OneTimeCalls` 与 `AllowedCalls` / `AdditionCalls` 重叠配置，升级后会在配置加载阶段失败；旧版 ptrace 的 allow 优先语义会让这种重叠退化为永久 allow，当前版本改为明确拒绝这种模糊配置。

迁移规则：如果某个 syscall 同时出现在 `OneTimeCalls` 和 runtime allowlist，需要按真实意图二选一；需要永久允许时从 `OneTimeCalls` 删除，只允许启动边界或一次性调用时从 `AllowedCalls` / `AdditionCalls` / `SyscallPolicy.Allow` 删除。

这一步的关键收益是把“一次性调用”“永久允许调用”“需要审计的调用”“显式拒绝的调用”分层，而不是全部混在一个白名单里。当前实现采用明确优先级：先合并 allow，再用 deny 做减法；allow/trace/audit/one-time 保持互斥。代码上先得到 `EffectiveSyscallPolicy`，再编译成后端输入：ptrace 消费 `callPolicySpec`，seccomp 消费最终 `ALLOW`/`TRACE` 两组 syscall 名称，调用点不再重复拼装四个桶。

### Phase 3: 启动模型重构

目标：消掉 `execve once` 这个最难和 seccomp 对齐的历史包袱。

这里有两个可选方向：

1. 保守路线：继续保留很薄的一层 ptrace，只负责 `execve` 和少量审计。
2. 激进路线：重构进程启动模型，让 seccomp 在用户程序真正开始执行前完成安装，且不再依赖 runtime `execve once` 配额语义。

如果走激进路线，通常要配合：

1. `exec.Cmd` / `ForkExec` 重构
2. 更清晰的 stage-1 / stage-2 launcher
3. 对 `execve` 语义重新设计，而不是简单照搬当前 one-time 配额

## 规则生成建议

建议不要手写 BPF 指令，直接使用成熟库生成过滤器。优先级建议：

1. libseccomp 封装
2. 纯 Go 的 seccomp 封装库

规则生成逻辑建议分三层：

1. 名称解析层：把 syscall 名称映射到当前架构的 syscall 编号。
2. 策略归一层：把 legacy `AllowedCalls` / `AdditionCalls`、结构化 `SyscallPolicy`、语言差异和平台差异整理成统一 `EffectiveSyscallPolicy`。
3. 策略编译层：把 effective policy 编译成 ptrace 与 seccomp 各自消费的后端输入。
4. 后端生成层：输出 seccomp filter。

这三层分开后，未来如果切换架构或更新 syscall 表，不需要直接动运行时逻辑。

## 安装顺序建议

如果引入 seccomp，推荐子进程顺序调整为：

1. `awaitCgroupGate()`（等待父进程完成 cgroup 加入）
2. `setrlimits()`（cpu / output / stack / nofile / core）
3. `redirectIO()`
4. `setpgid()`
5. namespace 阶段
6. rootfs 阶段
7. no_new_privs 阶段
8. 凭证切换阶段（`setgroups` → `setgid` → `setuid`）
9. `ptraceTraceme()` 或等价的迁移期观测逻辑
10. child 用 `SIGSTOP` 等待 parent 设置 `PTRACE_O_TRACESECCOMP`
11. `installSeccomp()`
12. `execve()`

这里把 `installSeccomp()` 放在凭证切换之后，是为了避免过滤器误伤前置的特权 syscall。
首版实现进一步把它放在 `ptraceTraceme()` 之后，是为了不把 `ptrace` 加进用户程序继承的 runtime allowlist；`execve once` 不进入永久 allowlist，而是通过 `SECCOMP_RET_TRACE` 交给 ptrace 审批。

## 测试计划

迁移时建议把测试拆成三层：

### 1. 规则单元测试

覆盖：

1. syscall 名称到编号映射
2. 每种语言的 allowlist 合并结果
3. deny / trap / allow 三种动作的生成结果

### 2. Linux 集成测试

优先覆盖：

1. 真实 `SIGTRAP` 不应影响 runtime 过滤
2. `socket`、`connect`、`ptrace`、`bpf` 等危险 syscall 被拒绝
3. 正常题解的基础 syscall 仍然放行
4. Java/C++ 等多语言启动路径没有被误杀

### 3. 结果对照测试

在 hybrid 阶段，同一组样例分别跑：

1. ptrace only
2. seccomp + ptrace

输出应对齐到同一份判题结果和关键日志指标。hybrid event-only 模式不会生成普通 syscall enter/exit 日志；需要迁移期观测的 syscall 应显式放入 `Trace` 或 `Audit`。当前集成覆盖包括 `tests/syscall-policy-deny-hybrid` 的 Deny 覆盖继承 allow，以及 `tests/syscall-policy-trace-audit-hybrid` 的 Trace/Audit 端到端审计日志。

## 建议的首批实现范围

第一批不建议碰所有语言，先收敛在 C/C++ 运行路径：

1. 引入 seccomp 安装接口和最小规则生成器
2. 只覆盖当前默认 allowlist 里的 syscall
3. 保留 ptrace 兜底
4. 用 feature flag 控制，例如 `UseSeccomp` 或 `SyscallBackend=ptrace|hybrid`

这样可以先把工程接缝打通，再逐步扩大语言覆盖面。

## 成功标准

可以把迁移完成拆成三个里程碑：

1. C/C++ 在 hybrid 模式下稳定运行，危险 syscall 由 seccomp 拦截。
2. 现有 ptrace 仅保留极少数启动边界和审计职责。
3. 决定是否继续保留 ptrace，或进入完全基于 seccomp 的新启动模型。
