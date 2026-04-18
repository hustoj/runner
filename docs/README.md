# docs

该目录存放设计分析、修复说明和迁移草案。这里适合快速理解“为什么这么实现”，但真正的行为仍以源码为准。

## 文档速览

- `ptrace-minimal-fix.md`：解释 ptrace 状态机的最小修复背景、边界和剩余风险
- `sandbox-refactor-analysis.md`：说明沙箱统一入口和执行顺序的设计思路
- `seccomp-bpf-migration.md`：描述从 ptrace 过滤迁移到 seccomp-bpf 的分阶段方案
- `runner/syscalls.md`：运行期 syscall 参考清单，偏实践侧

## 建议阅读路径

- 想理解当前 ptrace 逻辑的历史问题：先读 `ptrace-minimal-fix.md`
- 想理解 namespace / chroot / 降权顺序：先读 `sandbox-refactor-analysis.md`
- 想看未来安全演进方向：读 `seccomp-bpf-migration.md`
- 想查运行时 syscall 示例：看 `runner/syscalls.md`

## 注意

- 设计文档不是唯一事实来源，修改实现时请同步核对 [`../runner/`](../runner/) 与 [`../sec/`](../sec/)。
