# cmd/compiler

该目录实现编译阶段的独立入口，职责是读取 `compile.json`、限制编译资源、执行实际编译器，并以 JSON 输出编译是否成功。

## 文件说明

- `main.go`：总入口，负责加载配置、初始化日志、执行编译流程并输出 JSON。
- `cfg.go`：`CompileConfig` 定义，描述 `compile.json` 的字段和默认值。
- `compiler_linux.go`：Linux 实现，负责 `fork`、`setrlimit`、输出重定向、`exec` 真正的编译器。
- `compiler_darwin.go`：macOS 占位实现，仅用于跨平台开发时保持可编译。
- `makefile`：本地 smoke 用法示例。
- `tests/`：编译期 fixture，不同语言或极端场景的 `compile.json` 与样例源码。

## 常见检索入口

- 想改默认编译命令或参数：`cfg.go`
- 想改编译资源限制：`compiler_linux.go` 中的 `setrLimits`
- 想改编译输出文件名：`compiler_linux.go` 中的 `doCompile`
- 想看最终返回结构：`main.go` 中的 `RunResult`

## 关键标识符

- `CompileConfig`
- `loadConfig`
- `setrLimits`
- `doCompile`
- `handle`

## 相关目录

- 编译镜像上下文在 [`../../docker/`](../../docker/)
- 运行期评测入口在 [`../runner/`](../runner/)
