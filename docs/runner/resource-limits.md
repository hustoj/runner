# 运行期资源限制契约

本文说明 `runner` 在 Linux 上如何解释 `case.json` 里的资源限制字段，以及这些字段和最终判题结果之间的关系。

## 1. 配置字段语义

- `CPU`
  - 单位：秒
  - 语义：判题使用的 CPU 时间上限
- `Memory`
  - 单位：MB
  - 语义：任务的**总内存预算**
  - Linux 运行时直接把它映射到 cgroup v2 `memory.max`
- `MemoryReserve`
  - 单位：MB
  - 状态：**已废弃**
  - Linux 运行时不再使用该字段；保留它只是为了兼容旧 `case.json`
- `Stack`
  - 单位：MB
  - 语义：独立的栈上限，直接映射到 `RLIMIT_STACK`
- `MaxProcs`
  - 单位：个
  - 语义：任务可创建的进程 / 线程总数上限，Linux 上映射到 task cgroup v2 `pids.max`
  - Linux 口径是“当前 task 自己的进程 / 线程树”，不再受同 UID 其它桌面进程影响
  - 默认值 `16` 对 C / C++ 通常够用，但 JVM 这类运行时往往需要显式调大
  - 背后的 Java 回归与修复过程见 [`java-runtime-follow-up.md`](./java-runtime-follow-up.md)
- `Output`
  - 单位：MB
  - 语义：输出文件大小上限

## 2. 限制模型总览

Linux 运行期现在分成两类：

1. **统一口径限制**
   - 目前只有 Memory
   - enforcement 与 verdict 都来自 cgroup v2
2. **task cgroup 硬限制**
   - 目前是 MaxProcs
   - enforcement 来自 cgroup v2 `pids.max`
   - 触发后通常表现为新的 `clone()` / `clone3()` / `fork()` 失败
3. **判题层 + 内核兜底**
   - 目前是 CPU / Output / Stack
   - 分别由 trace 结果、signal、`setrlimit` 与 `alarm` 共同决定

这意味着：

- Memory 不再是“采样值 + 地址空间 workaround”的双轨模型
- MaxProcs 不再依赖同 UID 全局共享的 `RLIMIT_NPROC` 语义
- 如果 cgroup v2 memory backend 不可用，runner 会**明确启动失败**
- 不会再静默退回 `RLIMIT_DATA` / `RLIMIT_AS`

## 3. Memory 在 Linux 上的具体落点

### 3.1 enforcement

每次运行会创建一个独立 task cgroup，并写入：

- `memory.max = Memory << 20`
- `memory.oom.group = 1`
- `memory.swap.max = 0`（如果内核暴露该文件）

子进程在 `fork()` 后会先阻塞，父进程先把 child pid 写入 `cgroup.procs`，再放行 child 继续执行沙箱、ptrace 和 `execve()`。这样可以保证后续 `exec` 与运行时分配都记到目标 cgroup 里。

### 3.2 结果采集

- `PeakMemory`
  - 来自 cgroup v2 `memory.peak`
  - runner 对外仍以 `KB` 输出
- `RusageMemory`
  - 仍然来自 `wait4(...).ru_maxrss` 聚合
  - 只用于诊断，不再参与最终 MLE 判定

### 3.3 MLE 判定

最终满足以下任一条件，就会记为 `MEMORY_LIMIT`：

- `memory.events.oom > 0`
- `memory.events.oom_kill > 0`

注意：

- `memory.events.max` 只表示到过 `memory.max` 边界并触发过回收，不单独作为 MLE 判定条件
- 因此 `PeakMemory` 字段现在主要是**展示最终峰值**，不是 verdict 的唯一来源

## 4. 其它资源限制

### MaxProcs

- enforcement：
  - `pids.max = MaxProcs`
- Linux 行为：
  - 当 task 内的进程 / 线程总数达到上限后，新的 `clone()` / `clone3()` / `fork()` / `vfork()` 会失败
  - 这类失败不会自动映射成单独的判题码，结果仍取决于语言运行时或用户程序如何处理创建失败
- 迁移收益：
  - 不再受同 UID 其它进程 / 线程数影响
  - 口径和 `memory.max` 一样，都是 task-local 的 cgroup 预算
- 相关背景：
  - Java 回归与最终修复见 [`java-runtime-follow-up.md`](./java-runtime-follow-up.md)

### CPU

- 判题口径：所有被跟踪 tracee 的 `utime + stime` 累加值
- 结果判定：超过 `CPU` 后记为 `TIME_LIMIT`
- 内核兜底：
  - `RLIMIT_CPU = CPU + 1`
  - `alarm = CPU + 5`

### Output

- 内核兜底：
  - `RLIMIT_FSIZE = Output`
- `SIGXFSZ` 会被映射为 `OUTPUT_LIMIT`

### Stack

- 内核兜底：
  - `RLIMIT_STACK = Stack`
- 当前并不会把所有栈相关失败统一归到 `MEMORY_LIMIT`
- 如果程序因为栈扩展失败触发 `SIGSEGV`，通常仍会落到 `RUNTIME_ERROR`

## 5. 部署要求

Linux runtime 需要：

- cgroup v2 挂载点（默认 `/sys/fs/cgroup`）
- `memory` 与 `pids` controller 已启用
- 一个可写、已委派的父 cgroup，用于创建每次运行的 task cgroup

runner 的父 cgroup 选择顺序：

1. 如果设置了 `RUNNER_CGROUP_PARENT`，优先使用它
2. 否则从当前进程所在 cgroup 往上找最近的、可写的 domain cgroup：
   - `subtree_control` 已包含 `memory` 和 `pids`
   - 没有 internal processes（root cgroup 除外）

如需覆盖挂载点，可设置 `RUNNER_CGROUP_MOUNT`。

## 6. 迁移建议

- 新的 Linux 契约里，`Memory` 是**总预算**，不是 RSS-like 采样阈值
- 管理型运行时要在这个总预算里自己切 heap / metaspace / code cache
- 例如 JVM 不应再把 `-Xmx` 直接顶到 `Memory`
- 旧的 `MemoryReserve` workaround 已经移除；如果旧部署依赖它，需要改成：
  - 提供合适的 cgroup v2 delegation
  - 重新校准语言运行时参数
