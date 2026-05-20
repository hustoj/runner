# docs

该目录存放设计分析、修复说明和迁移草案。这里适合快速理解“为什么这么实现”，但真正的行为仍以源码为准。

## 文档速览

- `ptrace-minimal-fix.md`：记录 ptrace 状态机从最小修复走到多 tracee 跟踪的演进、边界和剩余风险
- `sandbox-refactor-analysis.md`：说明沙箱统一入口和执行顺序的设计思路
- `seccomp-bpf-migration.md`：描述从 ptrace 过滤迁移到 seccomp-bpf 的分阶段方案
- `runner/syscalls.md`：运行期 syscall 参考清单，偏实践侧
- `runner/resource-limits.md`：`case.json` 中各资源限制字段的运行期语义和判题口径
- `runner/cgroup-v2-memory.md`：Linux runtime 的 cgroup v2 内存/进程限制设计与落地
- `runner/java-runtime-follow-up.md`：cgroup 迁移后 Java 用例的回归排查与修复记录

## 建议阅读路径

- 想理解当前 ptrace 逻辑的历史问题和后续扩展：先读 `ptrace-minimal-fix.md`
- 想理解 namespace / chroot / 降权顺序：先读 `sandbox-refactor-analysis.md`
- 想看未来安全演进方向：读 `seccomp-bpf-migration.md`
- 想查运行时 syscall 示例：看 `runner/syscalls.md`
- 想了解资源限制的完整契约：看 `runner/resource-limits.md`
- 想了解 cgroup v2 内存模型的设计与实现：看 `runner/cgroup-v2-memory.md`
- 想回看 Java 回归的排查过程：看 `runner/java-runtime-follow-up.md`

## 注意

- 设计文档不是唯一事实来源，修改实现时请同步核对 [`../runner/`](../runner/) 与 [`../sec/`](../sec/)。
