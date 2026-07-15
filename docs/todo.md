# TODO / 待完成事项

来自文档一致性审查中需要进一步验证或需要在合适条件下处理的事项。

## Sandbox 行为级集成测试（原 #12）— 主体已完成

**当前状态**：已在 `runner/sandbox_behavior_linux_test.go` 增加一组 Linux 行为测试，直接通过 `runProcess()` 启动真实 child path，并在 trace loop 之前检查 `/proc/<pid>` 状态。CI 中 GitHub Actions 默认用户跑 `make test-unit`，额外用 privileged Docker 跑 `make test-sandbox-behavior-root`。

已覆盖：

- [x] `no_new_privs` 生效验证：通过读取 `/proc/<pid>/status` 断言 `NoNewPrivs: 1`
- [x] non-root 失败路径验证：普通用户启用 `chroot` / mount namespace 时，`runProcess()` 应返回明确的 `EPERM`
- [x] `chroot + workdir` 验证：通过 `/proc/<pid>/root` 和 `/proc/<pid>/cwd` 确认已进入 jail 和目标工作目录
- [x] Namespace 创建验证：验证 child 的 mount namespace 与父进程不同；当前环境若无 `CAP_SYS_ADMIN` 会显式 skip
- [x] 凭据切换验证：`setuid/setgid` 后确认 `/proc/<pid>/status` 中的 `Uid/Gid` 已切换
- [x] task cgroup 放置验证：child 进入独立 task cgroup，并确认 `memory.max` / `pids.max` 已按配置写入
- [x] 在具备 root 或等效 capability 的 Linux CI / 容器环境里稳定执行这组测试

仍开放：

- [ ] 如后续引入更多 namespace 组合（IPC / UTS / Net 等），补充更细粒度的隔离断言

## 内存限制实现验证（原待验证项 A）— 部分完成

**当前状态**：Linux runtime 已切换到 cgroup v2 memory controller，`Memory` 直接映射到 `memory.max`，最终 MLE 依赖 `memory.events` / `memory.peak`。`MemoryReserve` 字段已从 `TaskConfig` 中删除。

已完成：

- [x] 为 managed runtime（尤其 JVM）提供“总预算 vs 堆参数”示例与文档
  - 集成用例：`tests/java`（及 `java-ole` / `java-tle`）在 `Memory` 总预算下显式配置 `-Xmx` / metaspace / code cache / 栈等参数
  - 契约说明：`docs/runner/resource-limits.md`、`docs/runner/cgroup-v2-memory.md`
  - 相关 follow-up：`docs/runner/java-runtime-follow-up.md`（`MaxProcs -> pids.max` 等）
- [x] 父 cgroup 自动选择算法的单元覆盖与 CI 集成路径
  - 单元：`findDelegatedCgroupParent` / `resolveConfiguredCgroupParent` / `isUsableCgroupParent`（`runner/cgroup_linux_test.go`）
  - CI：`integration-testall` 在 privileged 容器中创建可委派父 cgroup，并设置 `RUNNER_CGROUP_PARENT`
- [x] “仅有 cgroup OOM 事件也应记 MLE”的核心判定单测
  - `TestOutOfMemoryUsesMemoryControllerEvents`：`oom > 0` 且 peak 未超限仍记 MLE
  - `TestCgroupTaskControllerMemoryStatusUsesPeakAndEvents`：`memory.events` → `Exceeded()`
  - `TestApplyTerminationSignalTreatsSIGKILLAsMemoryLimitWhenControllerExceeded` 等终止路径

仍开放：

- [ ] 在更多 systemd / 容器 delegation 形态下做真实环境矩阵确认（当前主要是 fake FS 单测 + 一种 CI 容器路径）
- [ ] 为 cgroup `oom` 但进程自行处理 `ENOMEM` 并正常退出的场景补一条**真实进程行为测试**，确认最终仍记 `MEMORY_LIMIT`（逻辑已有单测，缺 end-to-end fixture）
- [ ] 评估是否需要额外暴露 `memory.current` / `memory.stat` 诊断信息，帮助分析复杂语言运行时的预算分布（当前仅 `PeakMemory` / `RusageMemory` / OOM 计数）

**验证方式**：开放项仍需在 Linux 真机或具备 cgroup v2 delegation 的 CI / 容器环境里补测。

## Compiler 默认日志路径（原待验证项 B）— 已完成

**背景**：`cmd/compiler/cfg.go` 默认日志路径曾为 `/var/log/runner/compiler.log`。

已处理：

- [x] 默认值已改为空字符串 `""`，与 runner 保持一致
- [x] 不再硬编码到可能不存在的系统目录
