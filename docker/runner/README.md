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

## 相关目录

- 运行入口源码在 [`../../cmd/runner/`](../../cmd/runner/)
- 核心执行逻辑在 [`../../runner/`](../../runner/)
- 镜像构建总览在 [`../README.md`](../README.md)
