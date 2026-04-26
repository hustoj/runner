# Java runtime follow-up after the cgroup migration

本文记录 cgroup v2 memory redesign 落地后，通用 Java 用例 `tests/java` 一度从 `ACCEPT` 回退成 `RUNTIME_ERROR` 的排查过程、根因和最终修复。

## 1. 背景

内存重构完成后，Linux runtime 的 `Memory` 已经收敛到 task cgroup：

- enforcement：`memory.max`
- verdict：`memory.events` / `memory.peak`

相关内存用例都通过了，但通用 Java 用例仍然失败：

- 期望：`ACCEPT`
- 实际：`RUNTIME_ERROR`

这说明问题已经不在旧的 `MemoryReserve` workaround，而是在 Java 运行时与 Linux task 模型的另一个边界条件上。

## 2. 现象

排查期间确认了几件关键事实：

1. 同一条 Java 命令直接运行是成功的。
2. 同一条 Java 命令放进手工创建、同样 `memory.max=256MB` 的 cgroup 后仍然成功。
3. 在 runner 里，早期失败点出现在 JVM 启动线程创建阶段：第一批 `clone3()` 一度返回 `-EAGAIN`。
4. 把线程预算问题修掉后，Java 仍然可能因为 tracer 在 syscall-stop 上拿不到寄存器而被误判成 `RUNTIME_ERROR`。

因此，最终根因并不是“cgroup memory 改坏了 Java”，而是这次迁移把两个原本隐藏的 Linux 运行期问题暴露了出来。

## 3. 根因一：`MaxProcs` 用 `RLIMIT_NPROC` 表达是错语义

旧实现里，`TaskConfig.MaxProcs` 在 Linux 上通过 `RLIMIT_NPROC` 设置。

这有一个关键问题：`RLIMIT_NPROC` 的计数口径是**同一个 real UID 下的全部进程 / 线程总数**，不是当前 task 自己的进程树。

这会带来两个后果：

1. 它不是 sandbox task-local 限制，而是会和同 UID 的其它终端、桌面进程、后台服务共享额度。
2. 在共享开发环境里，即使当前被评测程序本身只需要几十个线程，只要当前用户已经有大量线程存在，新的 `clone3()` 也会因为 `EAGAIN` 失败。

JVM 对这个问题特别敏感，因为 HotSpot 在启动期就会创建多条内部线程。默认 `MaxProcs: 16` 对简单 C / C++ 程序通常够用，但对现代 JVM 往往不够。

### 最终修复

Linux runtime 不再用 `RLIMIT_NPROC` 实现 `MaxProcs`，而是改成 task cgroup v2：

- `pids.max = MaxProcs`

这样 `MaxProcs` 的语义就和新的 `Memory` 一样，变成了**当前 task 自己的 cgroup 预算**，不再受同 UID 其它进程影响。

对应地，Linux 部署契约也同步收紧：

- 目标父 cgroup 不仅要委派 `memory`
- 还必须委派 `pids`

通用 Java fixture 也显式设置了：

- `MaxProcs: 64`

这不是 workaround，而是把 Java 运行时的真实线程需求写回用例配置。

## 4. 根因二：短生命周期 tracee 的 `ESRCH` 被误判成 syscall 违规

把 `clone3()` 的线程预算问题修掉后，Java 仍然有概率失败。继续追踪发现：

- 某些短生命周期线程已经停在 syscall-stop
- 但在 tracer 调用 `PtraceGetRegs(pid)` 前，这个 tracee 已经退出
- 内核因此返回 `ESRCH`

旧逻辑把任何 `PtraceGetRegs` 失败都当成“syscall 检查失败”，随后直接：

1. 杀掉整棵被评测进程树
2. 把结果写成 `RUNTIME_ERROR`

这对 JVM 这种多线程、短生命周期 internal thread 较多的运行时尤其容易放大。

### 最终修复

syscall 检查现在区分三种结果：

1. `syscallCheckOK`
2. `syscallCheckViolation`
3. `syscallCheckTraceeGone`

其中：

- 未注册 pid
- `PtraceGetRegs(pid)` 返回 `ESRCH`

都按 `tracee gone` 处理，只跳过这次 stop，而不再把它误判成 syscall 违规。

真正的非法 syscall 仍然保持原有行为：直接记为 `RUNTIME_ERROR`。

## 5. 为什么最终方案是对的

这次修复没有停在“把 Java case 的数字调大”：

### 5.1 为什么不是只改 `tests/java/case.json`

如果只把 Java fixture 的 `MaxProcs` 调大，而继续保留 `RLIMIT_NPROC`：

- 行为仍然依赖当前 UID 的系统背景负载
- 仍然不是 task-local 语义
- 在 CI、桌面 session、共享 runner 上都可能继续漂移

也就是说，只改用例只能掩住当前现象，不能修正 Linux runtime 契约本身。

### 5.2 为什么 `pids.max` 更合适

`pids.max` 有几个优点：

- 和 `memory.max` 一样，都是 task cgroup 级别的限制
- 直接覆盖进程和线程总数
- 不与同 UID 的其它任务共享预算
- 更符合“每次评测一个独立 task cgroup”的整体设计

它把 `MaxProcs` 从“用户级全局副作用”改回了“任务级资源预算”。

### 5.3 为什么要修 tracer，而不是给 Java 特判

`PtraceGetRegs -> ESRCH` 是 tracee 生命周期竞态，不是 Java 特有逻辑。

如果用语言特判规避：

- 问题会藏起来
- 其它多线程运行时以后还会再次踩中

把它修成通用的 `tracee gone` 分支，才是可靠的 Linux ptrace 生命周期处理方式。

## 6. 对后续维护的启示

以后遇到 Java / Go / 其它管理型运行时在 Linux runner 里启动失败，建议优先分开看三类问题：

1. **syscall allowlist 不够**
   - 表现通常是明确的 “not allowed syscall”
2. **task 级线程预算不够**
   - 重点看 `MaxProcs` / `pids.max`
   - 典型现象是 `clone3()` / `clone()` 失败
3. **ptrace 生命周期竞态**
   - 重点看线程退出、attach stop、`ESRCH` 这类边界状态

不要再把 `RLIMIT_NPROC` 当作 Linux 上 `MaxProcs` 的正确实现，也不要把短生命周期 tracee 的 `ESRCH` 直接归类为 syscall 违规。

## 7. 相关代码入口

- `runner/cgroup_linux.go`
  - task cgroup 创建、`memory.max` / `pids.max` 配置、父 cgroup 选择
- `runner/exec_linux.go`
  - child 启动路径，Linux 上不再设置 `RLIMIT_NPROC`
- `runner/tracer_linux.go`
  - syscall 检查结果从“二元失败”改成 “OK / violation / tracee gone”
- `runner/exec.go`
  - trace loop 对 `tracee gone` 只跳过当前 stop，不再误杀整棵进程树
- `tests/java/case.json`
  - Java 用例显式声明更合理的 `MaxProcs`

## 8. 结论

这次 Java 回归不是 cgroup memory 方案本身的问题，而是一次有价值的后续清理：

1. 把 `MaxProcs` 从错误的 `RLIMIT_NPROC` 语义迁移到 task cgroup `pids.max`
2. 修正 ptrace 对快速退出线程的误判

做完这两步后，Java 用例重新通过，而且 Linux runtime 的资源模型也比之前更一致：

- `Memory` 看 task cgroup
- `MaxProcs` 也看 task cgroup
- ptrace 生命周期边界不再把正常线程退出误写成 `RUNTIME_ERROR`
