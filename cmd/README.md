# cmd

该目录存放仓库的可执行入口，每个子目录通常对应一个二进制。

## 子目录速览

- `runner/`：评测执行入口。读取 `case.json`，调用 `runner` 包执行沙箱与 syscall 跟踪，最终输出 JSON 结果。
- `compiler/`：编译入口。读取 `compile.json`，设置编译资源限制，把输出重定向到 `compile.out` / `compile.err`，最终输出 JSON 编译结果。
- `test/`：集成测试辅助入口。读取 `case.json` 后执行 `runner`，并把实际结果与预期 `Result` 做比较。

## 常见检索入口

- 想看评测主入口：`runner/main.go`
- 想看编译主入口：`compiler/main.go`
- 想看 `compile.json` 配置结构：`compiler/cfg.go`
- 想看测试辅助程序如何判定通过/失败：`test/main.go`

## 关键标识符

- `runner.LoadConfig`
- `RunningTask`
- `CompileConfig`
- `RunResult`

## 相关目录

- 核心运行逻辑在 [`../runner/`](../runner/)
- 编译器 smoke fixture 在 [`compiler/tests/`](compiler/tests/)
- 运行期集成用例在 [`../tests/`](../tests/)
