# Repository Guidelines

## 项目结构与模块组织
`cmd/runner`、`cmd/compiler`、`cmd/test` 是三个可执行入口。`runner/` 是核心库，按能力拆分为 `exec*`、`process*`、`memory*`、`tracer*`、`sandbox*`，并使用 `*_linux.go`、`*_darwin.go` 区分平台实现。`sec/` 维护系统调用表与对应测试。`tests/` 存放集成用例，每个目录通常包含 `case.json`、示例程序和 `makefile`。`docker/` 保存镜像构建上下文，`docs/` 记录沙箱、ptrace 和 seccomp 相关设计说明。

## 构建、测试与开发命令
- `make`：构建 `bin/runner`
- `make compiler`：构建 `bin/compile`
- `make prepare`：构建 `bin/test`
- `go test ./...`：运行 Go 单元测试
- `make testall`：执行 `tests/` 下的集成评测用例
- `make build-docker-runner` / `make build-docker-compiler`：构建 Docker 镜像

如果本地环境受限导致 Go 缓存不可写，可用：
`GOCACHE=/tmp/go-build GOPATH=/tmp/go GOMODCACHE=/tmp/go/pkg/mod go test ./...`

## 编码风格与命名约定
遵循 `gofmt` 默认格式，保持 Go 代码使用制表符缩进。提交前运行 `pre-commit run --all-files`，仓库已启用 `go-fmt`、`go-mod-tidy` 和 `golangci-lint`。包名保持小写；平台相关文件沿用 `_linux.go`、`_darwin.go` 后缀；测试文件使用 `*_test.go`。新增配置项优先放在 `runner/config.go`，同时补充默认值与校验逻辑。

## 测试指南
单元测试使用 Go `testing` 和 `testify/assert`，示例见 `runner/tracer_linux_test.go`、`sec/syscalls_test.go`。新增评测场景时，在 `tests/<case>/` 下补齐 `case.json`、源程序和 `makefile`。修改 `ptrace`、`seccomp`、系统调用白名单或资源限制逻辑时，至少运行相关包的 `go test`，并在 Linux 上补跑 `make testall`。

## 提交与 Pull Request 规范
最近提交以类 Conventional Commits 风格为主，如 `feat: ...`、`fix: ...`、`test(runner): ...`、`refactor: ...`、`chore: ...`。摘要应简短，并在必要时带模块范围。PR 说明应包含影响的平台、执行过的命令、是否改变沙箱/安全行为；若修改 `case.json`、系统调用表或 Docker 构建流程，需在描述中明确指出。仓库当前没有现成的 PR 模板，说明请写完整。

## 安全与配置提示
该仓库以 Linux 评测沙箱为核心，Darwin 文件主要用于跨平台开发占位。不要轻易放宽允许的系统调用；相关变更应同步补测试并更新 `docs/`。运行时默认从工作目录读取 `case.json`，示例配置应保持最小、可复现。
