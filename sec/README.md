# sec

该目录提供 syscall 名称和编号之间的映射表，是运行期 syscall 白名单机制的基础数据层。

## 文件说明

- `syscalls.go`：Linux amd64 下的 syscall 名称表，`runner/sec.go` 会基于它把字符串名单转换成编号。
- `syscalls_test.go`：名称与编号映射的基本测试。
- `stub_other.go`：非目标平台的占位实现。

## 常见检索入口

- `case.json` 里增加了一个 syscall 名称但程序启动时报未知：先看 `syscalls.go`
- 想确认某个 syscall 名称是否存在：先查 `syscalls.go`
- 想看这张表在哪里被消费：看 [`../runner/sec.go`](../runner/sec.go)

## 关键标识符

- `SCTbl`
- `GetID`
- `GetName`

## 风险提示

这里的修改会直接影响 syscall 允许/拒绝判定，属于高风险变更。修改后通常需要同时检查：

- [`../runner/`](../runner/) 中的策略构建逻辑
- [`../tests/`](../tests/) 中的相关集成用例
- [`../docs/runner/`](../docs/runner/) 中的参考说明
