# ptrace 最小修复说明

## 背景

当前 runner 的 syscall 过滤基于 ptrace。原实现把所有 `SIGTRAP` 都当成同一类 stop，并用一个布尔值在每次 stop 时翻转 enter 和 exit 相位。

这个设计有两个直接问题：

1. 没有启用 `PTRACE_O_TRACESYSGOOD`，syscall-stop 和真实 `SIGTRAP` 无法区分。
2. enter 和 exit 相位依赖“每次 trap 都翻转一次”，tracee 可以通过真实 `SIGTRAP` 把状态机拨乱。

一旦相位错位，禁止 syscall 可能在 exit 分支才被检查，等于 syscall 已经执行完才发现。

## 本次最小修复

本次补丁只修正当前 ptrace 状态机，不引入 seccomp-bpf，也不重构为 `exec.Cmd`。

修复内容：

1. 在首次 trace stop 后设置 `PTRACE_O_TRACESYSGOOD | PTRACE_O_EXITKILL`。
2. 把 stop 分类拆成两类：
   - 初始 trace stop：`SIGTRAP`
   - syscall-stop：`SIGTRAP|0x80`
3. 只有 syscall-stop 才会切换 enter 和 exit 相位。
4. 非 syscall 的 ptrace stop 一律按异常处理，不再参与 syscall 状态机。
5. 启动阶段显式消费 bootstrap `execve` 的 one-time 配额，避免第一次 `execve` 漏记。

## 代码落点

- [runner/exec.go](runner/exec.go)
- [runner/tracer.go](runner/tracer.go)
- [runner/tracer_linux.go](runner/tracer_linux.go)
- [runner/process_linux.go](runner/process_linux.go)
- [runner/sec.go](runner/sec.go)

## 为什么这是“最小修复”

这个补丁只解决当前确认存在的绕过面，不改变整体架构：

1. 仍然使用 ptrace 做 syscall 过滤。
2. 仍然只跟踪单个 pid 的主流程。
3. 不处理更完整的 ptrace event 模型，比如 `PTRACE_EVENT_CLONE`、`PTRACE_EVENT_FORK`、`PTRACE_EVENT_EXEC`。

也就是说，这个补丁的目标是把“任意真实 `SIGTRAP` 都会拨动状态机”的问题先关掉，而不是一次性把 runner 升级成完整的 ptrace 监控器。

## 剩余风险

这个最小修复之后，仍然有几类问题没有从根上解决：

1. 过滤逻辑仍在用户态，复杂度和性能都不如 seccomp-bpf。
2. 当前 `Wait4` 仍然围绕单个 pid 设计，不适合扩展到线程和 fork 后代的完整跟踪。
3. 如果后续要支持更复杂的 trace event，仍然需要把 stop 分类重构成显式状态机，而不是继续堆条件分支。

## 后续建议

下一阶段建议把 syscall allowlist 下沉到 seccomp-bpf，把 ptrace 从“安全边界”降级为“调试和观测工具”，或者直接移除。

建议顺序：

1. 先保持当前最小修复稳定运行。
2. 评估 seccomp-bpf 规则生成和平台兼容性。
3. 再决定是否保留 ptrace 作为辅助观测能力。
