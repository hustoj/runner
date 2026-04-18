# docker

该目录保存 Docker 镜像构建上下文。镜像构建前，`make` 目标会先把对应二进制编译到各自目录，再由 Dockerfile 复制进去。

## 子目录速览

- `runner/`：运行时镜像，安装 Java 运行时，复制 `runner` 二进制，默认 `CMD ["runner"]`
- `compiler/`：编译镜像，安装 `gcc`、`g++`、`fp-compiler`、`openjdk-8-jdk-headless`，复制 `compiler` 二进制，默认 `CMD ["compiler"]`

## 常见检索入口

- 想改镜像依赖：看对应子目录下的 `Dockerfile`
- 想看镜像如何与构建流程衔接：看仓库根目录 `makefile` 中的 `build-docker-runner` / `build-docker-compiler`
- 想确认容器内工作目录或卷：看 `WORKDIR /data` 与 `VOLUME` 定义

## 关键约定

- `/data` 是默认工作目录
- `/var/log/runner` 用于日志卷
- 这里的 `runner` / `compiler` 文件通常是构建产物，不是手写源码

## 相关目录

- 运行时入口在 [`../cmd/runner/`](../cmd/runner/)
- 编译入口在 [`../cmd/compiler/`](../cmd/compiler/)
