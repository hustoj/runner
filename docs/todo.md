# TODO / 待完成事项

来自 `doc-consistency.md` 审查中需要进一步验证或需要在合适条件下处理的事项。

## Sandbox 行为级集成测试（原 #12）

**背景**：当前 sandbox 相关测试只验证逻辑正确性（函数输入/输出），不验证实际行为。

需要在 Linux 环境下补充以下集成测试：

- [ ] `no_new_privs` 生效验证：子进程设置 PR_SET_NO_NEW_PRIVS 后，setuid binary 应被拒绝提权
- [ ] `chroot + workdir` 目录视图验证：chroot 后子进程只能看到 jail 内的文件系统
- [ ] Namespace 创建验证：unshare(CLONE_NEWNS / CLONE_NEWIPC / ...) 后确认隔离效果
- [ ] root / non-root 两条路径的对比测试
- [ ] 凭据切换验证：setuid/setgid 后子进程确认身份变更

**约束**：需要 root 权限或在 Docker/CI 中执行。

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
