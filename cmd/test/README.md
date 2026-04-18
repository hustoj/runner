# cmd/test

该目录提供集成测试辅助二进制，用来复用 `runner` 的执行流程，并把实际结果与 `case.json` 中的预期 `Result` 做对比。

## 当前文件

- `main.go`：读取 `case.json`，执行 `RunningTask`，输出 `passed` / `failed` 文本结果。

## 适合来这里排查的问题

- 想知道 `tests/*/makefile` 最终调用的是哪个可执行文件
- 想确认测试通过/失败的判定条件
- 想看集成测试入口如何复用 `runner` 主流程

## 关键标识符

- `runner.LoadConfig`
- `RunningTask`
- `GetResult`

## 相关目录

- 运行期用例目录在 [`../../tests/`](../../tests/)
- 核心执行逻辑在 [`../../runner/`](../../runner/)
