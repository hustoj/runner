# docker/compiler

该目录是编译镜像的构建上下文。镜像目标是提供一个能执行 `compiler` 二进制的最小编译环境。

## 当前文件

- `Dockerfile`：基于 `ubuntu`，安装 `gcc`、`g++`、`fp-compiler`、`openjdk-8-jdk-headless`，复制 `compiler` 到 `/usr/bin/`，默认工作目录是 `/data`。

## 适合来这里排查的问题

- 编译镜像里缺少某个编译器或运行库
- 容器启动后默认执行的命令是什么
- 编译日志和工作目录映射到哪里

## 关键约定

- `COPY compiler /usr/bin/`
- `WORKDIR /data`
- `VOLUME /data`
- `VOLUME /var/log/runner`
- `CMD ["compiler"]`

## 相关目录

- 编译入口源码在 [`../../cmd/compiler/`](../../cmd/compiler/)
- 镜像构建总览在 [`../README.md`](../README.md)
