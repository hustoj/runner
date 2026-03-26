# H3 — 内存限制 4 倍放大 & Stack 未独立设置

## 问题摘要

`runner/exec_linux.go` 的 `limitResource()` 函数存在两个关联问题：

1. **4× 放大**：OS 层面的 `RLIMIT_AS/DATA/STACK` 被设为配置值的 **4 倍**。
2. **Stack 未独立**：`TaskConfig.Stack` 字段存在但被完全忽略，`RLIMIT_STACK` 复用了放大后的内存值。

## 原始代码

```go
// runner/exec_linux.go (修复前)
memoryLimit := uint64(task.memoryLimit<<10) * 4      // ← 硬编码 4 倍
rLimit := &syscall.Rlimit{
    Max: memoryLimit + 5<<20,
    Cur: memoryLimit,
}
syscall.Setrlimit(syscall.RLIMIT_STACK, rLimit)      // ← 共用同一个 rLimit
syscall.Setrlimit(syscall.RLIMIT_DATA, rLimit)
syscall.Setrlimit(syscall.RLIMIT_AS, rLimit)
```

而 `config.go` 中明确定义了独立的 Stack 字段：

```go
Stack  int `default:"8"`   // Stack size limit in MB
```

该字段在 runner 中**从未被使用**。作为对比，编译器 (`cmd/compiler/compiler_linux.go`) 的实现是正确的——Stack 和 Memory 各自独立，无放大。

## 数值推导

以默认配置 `Memory: 256, Stack: 8` 为例：

| 阶段 | 值 | 单位 |
|------|-----|------|
| `setting.Memory` | 256 | MB |
| `task.memoryLimit` = 256 × 1024 | 262,144 | KB |
| rlimit (修复前) = 262144 << 10 × 4 | 1,073,741,824 | bytes = **1 GB** |
| rlimit Max (修复前) = 1 GB + 5 MB | ~1.005 GB | bytes |

**配置 256 MB，实际 OS 允许 1 GB。**

修复后：

| Limit | 公式 | 值 (Memory=256, Stack=8) |
|-------|------|--------------------------|
| RLIMIT_STACK | `Stack << 20` | **8 MB** |
| RLIMIT_DATA | `memoryBytes + 16 MB` | **272 MB** (Cur=256, Max=272) |
| RLIMIT_AS | `memoryBytes + max(memoryBytes, 64MB) + stackBytes` | **528 MB** |

## 风险分析

### 风险 1：资源滥用 / DoS

进程实际可使用 4× 内存。在多用户 OJ 环境中，恶意用户可消耗远超配额的内存，挤占其他评测进程资源。

### 风险 2：判定与 OS 限制的时间窗口不一致

```
软判定 (ptrace 循环): PeakMemory(VmHWM) > memoryLimit   → 256 MB
硬限制 (RLIMIT_AS):   虚拟地址空间上限                    → 1 GB (修复前)
```

ptrace 循环在每次 syscall 时采样 VmHWM。两次采样之间，进程可一次性 `mmap()` 大量内存而不被 OS 拒绝。若进程在下次采样前退出，可能逃过软判定。

### 风险 3：Stack 无实际约束

配置 `Stack: 8` 期望限制 8 MB 栈空间，但 `RLIMIT_STACK` 实际被设为 1 GB（以 Memory=256 为例）。深递归程序可使用远超预期的栈空间。

### 风险 4：配置欺骗

`Stack` 字段存在但无效，给运维人员"可控栈大小"的假象。

## 解决方案

### 1. RLIMIT_STACK — 独立使用 Stack 配置

```go
stackBytes := uint64(task.setting.Stack) << 20 // MB → bytes
syscall.Setrlimit(RLIMIT_STACK, &Rlimit{Max: stackBytes, Cur: stackBytes})
```

直接从 `TaskConfig.Stack` 读取，与编译器行为一致。

### 2. RLIMIT_DATA — 去掉 4× 放大，保留小余量

```go
const dataOverhead = 16 << 20 // 16 MB
syscall.Setrlimit(RLIMIT_DATA, &Rlimit{
    Max: memoryBytes + dataOverhead,
    Cur: memoryBytes,
})
```

16 MB 余量覆盖 libc malloc arena、TLS 等运行时元数据开销。

### 3. RLIMIT_AS — 合理余量，兼顾重型运行时

```go
asOverhead := memoryBytes
if asOverhead < 64<<20 {
    asOverhead = 64 << 20 // 最低 64 MB
}
asLimit := memoryBytes + asOverhead + stackBytes
syscall.Setrlimit(RLIMIT_AS, &Rlimit{Max: asLimit, Cur: asLimit})
```

**设计考量：**

- **为何需要额外虚拟地址空间？** 虚拟地址空间 ≠ 物理内存。`mmap` 映射的共享库、JVM/Go 运行时预留的虚拟 arena、guard page 等均计入 `RLIMIT_AS` 但不实际消耗物理内存。
- **为何用 `max(memoryBytes, 64 MB)` 作为 overhead？** 小内存配置（如 4 MB）下，64 MB 最低保障让 Java/Go 等重型运行时有足够空间启动。大内存配置下，`1×` 的额外空间已经足够。
- **对比修复前的 4×**：以 Memory=256 为例，旧值 1 GB → 新值 528 MB，缩减约 50%，同时仍然宽裕。
- **Stack 独立计入**：`RLIMIT_AS` 包含所有虚拟映射（堆 + 栈 + 库），因此将 Stack 加入 AS 总额是正确的。

### 各配置值对比表

| Memory (MB) | Stack (MB) | RLIMIT_AS 修复前 | RLIMIT_AS 修复后 | 缩减比 |
|-------------|------------|-------------------|-------------------|--------|
| 4           | 8          | 21 MB             | 76 MB*            | ↑ 更宽 |
| 32          | 8          | 133 MB            | 104 MB            | -22%   |
| 64          | 8          | 261 MB            | 200 MB            | -23%   |
| 128         | 8          | 517 MB            | 392 MB            | -24%   |
| 256         | 8          | 1,029 MB          | 528 MB            | -49%   |
| 512         | 8          | 2,053 MB          | 1,032 MB          | -50%   |
| 1024        | 8          | 4,101 MB          | 2,056 MB          | -50%   |

\* 小内存配置受益于 64 MB 最低保障，修复后反而比原来更宽裕（原来 4×4=16 MB 对 Java 根本不够）。

## 影响范围

- **runner/exec_linux.go** — `limitResource()` 函数（唯一改动点）
- **编译器不受影响** — `cmd/compiler/compiler_linux.go` 已经是正确实现
- **判定逻辑不受影响** — `outOfMemory()` 仍用 `task.memoryLimit`（原始 KB 值）对比 `VmHWM`

## 测试验证

- `go test ./...` 全部通过
- `tests/stack/`：Stack 溢出测试，修复后 RLIMIT_STACK=8 MB，行为正确（RUNTIME_ERROR）
- `tests/mle*/`：内存超限测试，ptrace 判定逻辑未变，行为不受影响
