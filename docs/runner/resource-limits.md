# 运行期资源限制契约

本文说明 `runner` 在 Linux 上如何解释 `case.json` 里的资源限制字段，以及这些字段和最终判题结果之间的关系。

## 1. 配置字段语义

- `CPU`
  - 单位：秒
  - 语义：判题使用的 CPU 时间上限
- `Memory`
  - 单位：MB
  - 语义：判题使用的内存上限
  - 判题口径是 RSS 类指标，不是虚拟地址空间总量
- `MemoryReserve`
  - 单位：MB
  - 默认值：`32`
  - 语义：附加到 `RLIMIT_DATA` / `RLIMIT_AS` 的固定余量
  - 目的：给地址空间、堆分配和运行时开销留出缓冲，避免硬限制早于判题逻辑触发
- `Stack`
  - 单位：MB
  - 语义：独立的栈上限，直接映射到 `RLIMIT_STACK`
- `Output`
  - 单位：MB
  - 语义：输出文件大小上限

## 2. 判题层与内核兜底层

`runner` 同时存在两套限制：

1. 判题层
   - 决定最终是否返回 `TIME_LIMIT` / `MEMORY_LIMIT`
   - 指标来自 `wait4` 和 `/proc/<pid>/status`
2. 内核兜底层
   - 由 `setrlimit` 和 `alarm` 实现
   - 负责在判题层来不及介入时兜住失控进程

这两层故意不是同一数值。

## 3. 具体落点

### CPU

- 判题口径：`utime + stime`
- 结果判定：超过 `CPU` 后记为 `TIME_LIMIT`
- 内核兜底：
  - `RLIMIT_CPU.Cur = CPU + 1`
  - `RLIMIT_CPU.Max = CPU + 3`
  - `alarm = CPU + 5`

### Memory

- 判题口径：
  - `PeakMemory`：`/proc/<pid>/status` 中的 `VmHWM`
  - `RusageMemory`：`wait4(...).ru_maxrss`
- 两者都按 `KB` 记录，且都偏向 RSS / 常驻内存口径
- 结果判定：
  - `PeakMemory > Memory`
  - 或 `RusageMemory > Memory`
  - 任一满足即记为 `MEMORY_LIMIT`
- 内核兜底：
  - `RLIMIT_DATA = Memory + MemoryReserve`
  - `RLIMIT_AS = Memory + MemoryReserve`

### Stack

- 内核兜底：
  - `RLIMIT_STACK = Stack`
- 当前判题逻辑不会把所有栈相关失败自动归类为 `MEMORY_LIMIT`
- 如果程序因栈扩展失败触发 `SIGSEGV`，当前通常会落到 `RUNTIME_ERROR`

### Output

- 内核兜底：
  - `RLIMIT_FSIZE.Cur = Output`
  - `RLIMIT_FSIZE.Max = Output * 2`
- `SIGXFSZ` 会被映射为 `OUTPUT_LIMIT`

## 4. 为什么 `MemoryReserve` 用固定余量

旧实现会把 `RLIMIT_DATA` / `RLIMIT_AS` 绑定到 `Memory` 的倍率值。这个模型有两个问题：

- 倍率缺少明确契约，外部无法推导真实硬限制
- 内存题越大，地址空间硬限制被放得越夸张

固定余量更符合当前实现的职责划分：

- `Memory` 负责判题
- `MemoryReserve` 负责兜底

这和 CPU 路径里“判题值 + 固定缓冲”的设计是一致的。

## 5. 事件归类

当前 signal 到结果码的直接映射如下：

- `SIGALRM` / `SIGXCPU` -> `TIME_LIMIT`
- `SIGXFSZ` -> `OUTPUT_LIMIT`
- 其他 signal -> `RUNTIME_ERROR`

因此内存和栈相关事件要分两种情况看：

- 如果判题层已经观测到 `PeakMemory` 或 `RusageMemory` 超出 `Memory`，最终会记为 `MEMORY_LIMIT`
- 如果内核硬限制先触发，而判题层尚未观测到 RSS 超限，程序可能表现为分配失败、`SIGSEGV` 或非零退出码，最终记为 `RUNTIME_ERROR`

`MemoryReserve` 的存在，就是为了尽量减少第二类“硬限制过早触发”的情况。

## 6. 调整建议

- C / C++：默认 `MemoryReserve = 32` 一般足够
- 运行时较重的语言：可以按语言特性适当增大 `MemoryReserve`
- 如果要把地址空间失败、栈溢出等事件也统一归到 `MEMORY_LIMIT`，需要额外设计更细的事件归类逻辑，不能只靠 `setrlimit`
