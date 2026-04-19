# ptrace 多 tracee 演进说明

> 文件名仍保留为 `ptrace-minimal-fix.md`，用于承接早期修复背景；当前内容已经覆盖后续的多 tracee 演进。

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

## 当前实现边界

当前实现已经在原有最小修复基础上继续向前推进了一步：

1. 仍然使用 ptrace 做 syscall 过滤。
2. 已经开始跟踪 `clone/fork/vfork` 自动附着出来的 tracee，并为每个 tracee 分别维护 syscall enter/exit 相位。
3. 主循环不再只围绕单个 pid `Wait4`，而是会消费父进程 event-stop 和新子进程的 attach-stop。

也就是说，runner 现在不再局限于“只跟踪单个 pid 的主流程”，但它仍然不是一个通用调试器，也还没有把所有 ptrace event 和资源统计语义都做到完全精细化。

## 剩余风险

这个最小修复之后，仍然有几类问题没有从根上解决：

1. 过滤逻辑仍在用户态，复杂度和性能都不如 seccomp-bpf。
2. 多 tracee 下的资源统计仍然是“按 thread group 去重采样内存 + 按 tracee 汇总 rusage”的近似模型，不是完整的进程树资源审计。
3. 如果后续要支持更复杂的 ptrace event（例如更细的 exec / seccomp 联动），仍然需要继续把 stop 分类收敛成更显式的状态机，而不是继续堆条件分支。

## 后续建议

下一阶段建议把 syscall allowlist 下沉到 seccomp-bpf，把 ptrace 从“安全边界”降级为“调试和观测工具”，或者直接移除。

建议顺序：

1. 先保持当前最小修复稳定运行。
2. 评估 seccomp-bpf 规则生成和平台兼容性。
3. 再决定是否保留 ptrace 作为辅助观测能力。
