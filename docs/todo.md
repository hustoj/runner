# TODO / 待完成事项

来自 `doc-consistency.md` 审查中需要进一步验证或需要在合适条件下处理的事项。

## Sandbox 行为级集成测试（原 #12）

**当前状态**：已在 `runner/sandbox_behavior_linux_test.go` 增加一组 Linux 行为测试，直接通过 `runProcess()` 启动真实 child path，并在 trace loop 之前检查 `/proc/<pid>` 状态。

已覆盖：

- [x] `no_new_privs` 生效验证：通过读取 `/proc/<pid>/status` 断言 `NoNewPrivs: 1`
- [x] non-root 失败路径验证：普通用户启用 `chroot` / mount namespace 时，`runProcess()` 应返回明确的 `EPERM`
- [x] `chroot + workdir` 验证：通过 `/proc/<pid>/root` 和 `/proc/<pid>/cwd` 确认已进入 jail 和目标工作目录
- [x] Namespace 创建验证：验证 child 的 mount namespace 与父进程不同；当前环境若无 `CAP_SYS_ADMIN` 会显式 skip
- [x] 凭据切换验证：`setuid/setgid` 后确认 `/proc/<pid>/status` 中的 `Uid/Gid` 已切换

仍需保证的执行条件：

- [x] 在具备 root 或等效 capability 的 Linux CI / 容器环境里稳定执行这组测试（GitHub Actions 默认用户跑 `make test-unit`，额外用 privileged Docker 跑 `make test-sandbox-behavior-root`）
- [ ] 如后续引入更多 namespace 组合，补充更细粒度的隔离断言

## 内存限制实现验证（原待验证项 A）

**当前状态**：Linux runtime 已切换到 cgroup v2 memory controller，`Memory` 直接映射到 `memory.max`，最终 MLE 依赖 `memory.events` / `memory.peak`。

后续仍建议补的回归项：

- [ ] 在不同 systemd / 容器 delegation 形态下确认自动父 cgroup 选择都能稳定命中
- [ ] 为 cgroup `oom` 但进程自行处理 `ENOMEM` 的场景补一条行为测试，确认最终仍记 `MEMORY_LIMIT`
- [ ] 为 managed runtime（尤其 JVM）补一组“总预算 vs 堆参数”示例用例，避免继续依赖旧 `MemoryReserve` 心智模型
- [ ] 评估是否需要额外暴露 `memory.current` / `memory.stat` 诊断信息，帮助分析复杂语言运行时的预算分布

**验证方式**：需准备针对性测试用例，在 Linux 真机或具备 cgroup v2 delegation 的 CI / 容器环境里实测。

## Compiler 默认日志路径（原待验证项 B）

**背景**：`cmd/compiler/cfg.go` 默认日志路径为 `/var/log/runner/compiler.log`。

待处理：

- [ ] 确认生产部署是否依赖该默认路径
- [ ] 如依赖：在 Docker 镜像或部署文档中确保目录存在并可写
- [ ] 如不依赖：考虑改为 `/dev/stderr` 与 runner 保持一致，或在 README 中说明
