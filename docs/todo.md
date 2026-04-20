# TODO / 待完成事项

来自 `doc-consistency.md` 审查中需要进一步验证或需要在合适条件下处理的事项。

## Sandbox 行为级集成测试（原 #12）

**当前状态**：已在 `runner/sandbox_behavior_linux_test.go` 增加一组 Linux 行为测试，直接验证真实内核状态，而不再只停留在 config/spec 层。

已覆盖：

- [x] `no_new_privs` 生效验证：通过读取 `/proc/self/status` 断言 `NoNewPrivs: 1`
- [x] non-root 失败路径验证：普通用户启用 `chroot` / mount namespace 时应收到明确的 `EPERM`
- [x] `chroot + workdir` 目录视图验证：进入 jail 后只能看到 jail 内文件，并确认工作目录切换到指定路径
- [x] Namespace 创建验证：验证 mount namespace 与父进程不同；当前环境若无 `CAP_SYS_ADMIN` 会显式 skip
- [x] 凭据切换验证：`setuid/setgid` 后确认 real/effective uid/gid 已切换

仍需保证的执行条件：

- [ ] 在具备 root 或等效 capability 的 Linux CI / 容器环境里稳定执行这组测试
- [ ] 如后续引入更多 namespace 组合，补充更细粒度的隔离断言

## 内存限制实现验证（原待验证项 A）

**背景**：`RLIMIT_DATA` / `RLIMIT_AS` 使用 `(Memory + MemoryReserve) << 20`，真正判定 MLE 依赖 `/proc/<pid>/status` 的采样。

待验证：

- [ ] 当前 `MemoryReserve` 默认值 (32MB) 是否足够覆盖典型 C/C++/Java 程序的初始化开销
- [ ] 快速分配后立即退出的程序（短生命周期 MLE）是否能被 `/proc` 采样捕获
- [ ] 多进程场景下（Java fork 子线程），`refreshPeakMemoryFromProc` 按 thread group 聚合是否准确
- [ ] 是否存在 RLIMIT_AS 过紧导致 `mmap` 失败的误判场景

**验证方式**：需准备针对性测试用例，在 Linux 真机上实测。

## Compiler 默认日志路径（原待验证项 B）

**背景**：`cmd/compiler/cfg.go` 默认日志路径为 `/var/log/runner/compiler.log`。

待处理：

- [ ] 确认生产部署是否依赖该默认路径
- [ ] 如依赖：在 Docker 镜像或部署文档中确保目录存在并可写
- [ ] 如不依赖：考虑改为 `/dev/stderr` 与 runner 保持一致，或在 README 中说明
