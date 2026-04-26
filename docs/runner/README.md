# docs/runner

该子目录存放运行期行为的补充参考文档，包括 syscall 参考、资源限制契约以及 cgroup v2 内存实现说明。

## 当前内容

- `syscalls.md`：给出通用场景和 Java 场景常见的 syscall 清单，适合快速比对语言运行时需要的额外调用。
- `resource-limits.md`：说明 `case.json` 中 CPU / Memory / Stack / Output 的运行期语义，以及 Linux 上的判题口径与硬限制关系。
- `cgroup-v2-memory.md`：说明 Linux runtime 如何用 cgroup v2 统一内存 enforcement 与 verdict。

## 适合来这里解决的问题

- 想快速回忆默认 syscall 白名单大致长什么样
- 想确认 Java 运行时为什么需要额外 syscall
- 想确认 `Memory` 和 `Stack` 在 Linux 上分别落到哪些限制
- 想了解 task cgroup 是怎么创建、加入和清理的
- 想写文档或补测试时找一个简短参考

## 真正的源码入口

- 默认 allowlist 与 `case.json` 配置字段：[`../../runner/config.go`](../../runner/config.go)
- 名称到编号的映射表：[`../../sec/syscalls.go`](../../sec/syscalls.go)
- 运行期策略构建：[`../../runner/sec.go`](../../runner/sec.go)
- 具体用例示例：[`../../tests/`](../../tests/)
