# seccomp-bpf 迁移草案

## 目标

把 syscall 过滤的安全边界从用户态 ptrace 下沉到内核态 seccomp-bpf，同时尽量复用当前 runner 的配置模型和测试资产。

这份草案的目标不是直接替换全部运行逻辑，而是给出一条低风险迁移路径。

## 当前现状

当前 runner 的 syscall 控制面有三个关键特点：

1. 白名单来自 [runner/config.go](runner/config.go) 的 `AllowedCalls` 和 `AdditionCalls`。
2. 一次性 syscall 来自 [runner/config.go](runner/config.go) 的 `OneTimeCalls`，默认包含 `execve`。
3. 运行时的最终判定发生在 [runner/tracer_linux.go](runner/tracer_linux.go) 的 ptrace loop 中。

这个模型的主要问题是：

1. syscall 过滤在用户态，性能和复杂度都不理想。
2. 真实 signal 和 ptrace event 容易把状态机搞复杂。
3. `execve` 这种“一次性允许”语义，并不适合直接映射到传统 seccomp allowlist。

## 为什么不建议一步切成纯 seccomp

最大的阻力不是 allowlist 本身，而是 `execve` 启动语义。

当前 runner 在子进程里完成：

1. rlimit
2. IO 重定向
3. sandbox 初始化
4. `execve` 进入用户程序

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
2. 在子进程完成 namespace、rootfs、no_new_privs、凭证切换之后安装 seccomp 过滤器。
3. seccomp 规则负责稳定的 runtime allowlist，例如 `read`、`write`、`mmap`、`brk`、`exit_group` 这类通用 syscall。
4. ptrace 暂时只处理两类场景：
   - bootstrap `execve` 的边界语义
   - 迁移期的审计和对照验证

这样做的好处是：

1. 大部分危险 syscall 在内核态直接被拒绝。
2. 可以逐步收缩 ptrace 的职责，而不是一次性替换所有行为。
3. 现有测试可以复用，并且能做 ptrace/seccomp 双轨对照。

### Phase 2: 配置模型重构

目标：把当前配置从“ptrace 时代的 syscall 名单”重构成“seccomp 时代的策略模型”。

建议新增一个显式策略层，例如：

```json
{
  "SyscallPolicy": {
    "DefaultAction": "errno",
    "Allowed": ["read", "write", "mmap", "brk"],
    "Denied": ["socket", "connect", "bpf", "ptrace"],
    "Trap": ["clone", "fork", "vfork", "execve"]
  }
}
```

这里的含义是：

1. `Allowed` 直接转成 seccomp allowlist。
2. `Denied` 直接返回 `EPERM` 或 `KILL_PROCESS`。
3. `Trap` 在 hybrid 模式下先交给 ptrace 或 `SECCOMP_RET_TRACE` 处理。

这一步的关键收益是把“一次性调用”“需要审计的调用”“可直接拒绝的调用”分层，而不是全部混在一个白名单里。

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
2. 策略归一层：把 `AllowedCalls`、语言差异和平台差异整理成统一策略。
3. 后端生成层：输出 seccomp filter。

这三层分开后，未来如果切换架构或更新 syscall 表，不需要直接动运行时逻辑。

## 安装顺序建议

如果引入 seccomp，推荐子进程顺序调整为：

1. `limitResource()`
2. `redirectIO()`
3. namespace 阶段
4. rootfs 阶段
5. no_new_privs 阶段
6. 凭证切换阶段（`setgroups` → `setgid` → `setuid`）
7. `installSeccomp()`
8. `ptraceTraceme()` 或等价的迁移期观测逻辑
9. `execve()`

这里把 `installSeccomp()` 放在凭证切换之后，是为了避免过滤器误伤前置的特权 syscall。

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

### 3. 对照测试

在 hybrid 阶段，同一组样例分别跑：

1. ptrace only
2. seccomp + ptrace

输出应对齐到同一份判题结果和关键日志指标。

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
