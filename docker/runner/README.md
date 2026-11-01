# docker/runner

该目录是运行期评测镜像的构建上下文。镜像目标是提供一个能执行 `runner` 二进制的最小运行环境。

## 当前文件

- `Dockerfile`：基于 `ubuntu`，安装 `openjdk-8-jre-headless`，复制 `runner` 到 `/usr/bin/`，默认工作目录是 `/data`。

## 适合来这里排查的问题

- 运行镜像里缺少某个语言运行时
- 容器内默认执行的是哪个二进制
- 评测任务目录和日志目录如何挂载

## 关键约定

- `COPY runner /usr/bin/`
- `WORKDIR /data`
- `VOLUME /data`
- `VOLUME /var/log/runner`
- `CMD ["runner"]`
- 镜像默认 `USER judger`（UID/GID 1536）；需要管理 task cgroup 的受信任启动器可显式以 root 启动，但 `case.json` 必须配置非 root `RunUID` / `RunGID`，否则 runner 会拒绝启动
- 与新版 judger 配合时，仅挂载已委派的 cgroup 子树，启动器使用 `SYS_PTRACE` 跟踪已降权的子进程；不需要 privileged 容器或 writable 的宿主 cgroup 根目录

## 相关目录

- 运行入口源码在 [`../../cmd/runner/`](../../cmd/runner/)
- 核心执行逻辑在 [`../../runner/`](../../runner/)
- 镜像构建总览在 [`../README.md`](../README.md)
