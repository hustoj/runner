# cmd/runner

该目录是运行期评测二进制的最薄入口层，职责只有一件事：读取 `case.json`，调用核心 `runner` 包执行任务，然后把结果序列化成 JSON 输出。

## 当前文件

- `main.go`：程序主入口，不承载业务策略，主要负责配置加载、日志初始化、调用 `RunningTask`。

## 适合来这里排查的问题

- 想确认最终 stdout 的输出格式
- 想看 `case.json` 是从哪里开始加载的
- 想确认主流程如何进入 `runner` 包

## 不适合在这里找的内容

- 资源限制、ptrace、沙箱、结果判定都不在这里，统一看 [`../../runner/`](../../runner/)

## 关键标识符

- `runner.LoadConfig`
- `runner.InitLogger`
- `RunningTask`
