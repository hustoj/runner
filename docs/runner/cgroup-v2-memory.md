# cgroup v2 内存实现设计

本文记录 Linux runtime 从 `RLIMIT_DATA` / `RLIMIT_AS` workaround 迁移到 cgroup v2 memory controller 的设计与落地结果。

## 1. 目标

旧实现存在两套不同的内存口径：

- enforcement：`RLIMIT_DATA` / `RLIMIT_AS`
- verdict：`/proc/<pid>/status` 与 `wait4` 采样

这会带来三个问题：

1. `Memory` 和真实硬限制不是一个数
2. `MemoryReserve` 成为长期 workaround
3. 地址空间失败可能先于判题层发生，结果落成 `RUNTIME_ERROR`

这次迁移的目标是把 Memory 收敛成**单一语义**：

- `Memory` = cgroup v2 总内存预算
- enforcement 与 verdict 都基于同一个 task cgroup

## 2. 运行环境契约

Runner 需要运行在 cgroup v2 环境中，并满足：

- cgroup v2 mount 可访问（默认 `/sys/fs/cgroup`）
- `memory` controller 已对目标子树启用
- runner 能在一个已委派的父 cgroup 下创建子 cgroup

可用环境变量：

- `RUNNER_CGROUP_MOUNT`
  - 覆盖 cgroup v2 挂载点
- `RUNNER_CGROUP_PARENT`
  - 显式指定父 cgroup
  - 支持：
    - 挂载点内的绝对文件系统路径
    - 以 `/` 开头的 cgroup 相对路径
    - 相对路径

如果没有配置 `RUNNER_CGROUP_PARENT`，runner 会从当前进程所在 cgroup 往上找最近的可用父 cgroup。

## 3. 父 cgroup 选择规则

自动选择时，候选父 cgroup 必须满足：

- 是可写目录
- `cgroup.type` 是 `domain`（root cgroup 允许为空）
- `cgroup.subtree_control` 已包含 `memory`
- 没有 internal processes（root cgroup 除外）

这样做的目的，是确保新建的 run cgroup 能直接拿到 `memory.max` / `memory.events` / `memory.peak` 这些 controller 文件，而不在 runtime 里去偷偷改上层 `subtree_control`。

## 4. 启动时序

每次运行都创建一个独立的 task cgroup，例如：

```text
<delegated-parent>/runner-<ppid>-<timestamp>
```

创建后立即写入：

- `memory.oom.group = 1`
- `memory.max = Memory << 20`
- `memory.swap.max = 0`（如果该文件存在）

随后进入新的 child 启动时序：

1. parent 创建 startup pipe 与 gate pipe
2. `fork()`
3. child 先阻塞在 gate pipe 上
4. parent 把 child pid 写入 run cgroup 的 `cgroup.procs`
5. parent 放行 gate
6. child 再继续：
   - `setrlimit(cpu/output/stack)`
   - `setpgid`
   - sandbox
   - `ptrace(TRACEME)`
   - `execve`

这个 gate 的目的，是保证被评测程序以及它后续 fork 出来的整个进程树从一开始就记到目标 cgroup 里。

## 5. 结果判定

### 5.1 Memory

- `PeakMemory`
  - 读取 `memory.peak`
  - 以 `KB` 形式回填到结果 JSON
- `MEMORY_LIMIT`
  - 当 `memory.events.oom > 0` 或 `memory.events.oom_kill > 0`

`RusageMemory` 仍然保留，但只做诊断字段。

### 5.2 其它限制

这次迁移没有改动：

- CPU：`RLIMIT_CPU` + `alarm`
- Output：`RLIMIT_FSIZE`
- Stack：`RLIMIT_STACK`
- syscall policy / ptrace 行为

## 6. 失败策略

如果 cgroup v2 memory backend 不可用，runner 会直接报错，而不是回退到旧的 `RLIMIT_AS` / `MemoryReserve` 逻辑。

这是一个有意的 fail-closed 设计，目的是避免不同部署环境下出现两套 Memory 语义。

## 7. 兼容性与迁移

`TaskConfig.MemoryReserve` 仍然保留在结构体里，只为了兼容旧 `case.json`。在 Linux runtime 中：

- 默认值已改成 `0`
- 非零时会给出 warning
- 运行时行为完全忽略它

对于管理型运行时（例如 JVM）：

- `Memory` 现在是**总预算**
- `-Xmx`、metaspace、code cache、线程栈都要在这个预算里自行留余量

## 8. 直接收益

迁移后可以直接消除原设计中的三个问题：

1. 不再依赖 `/proc` 采样捕获短生命周期 MLE
2. 不再需要 `MemoryReserve` 这种地址空间缓冲 workaround
3. Memory 的 enforcement 与 verdict 终于共用同一套内核记账
