# tests

该目录保存运行期集成测试用例。每个子目录通常都是一个最小可复现样例，用于验证 runner 对资源限制、signal、syscall 和语言运行时行为的判定是否正确。

## 用例结构

- `case.json`：运行配置与预期结果，字段定义对应 [`../runner/config.go`](../runner/config.go)
- 源码文件：如 `main.c`、`Main.java`、`thread.cpp`
- `makefile`：编译样例并调用 `bin/test` 或相关入口做验证

## 用例分类

- 基础成功路径：`general/`、`java/`
- 进程与线程：`fork/`、`thread/`
- 内存限制：`mle/`、`mle2/`、`mle21/`、`mle3/`
- 时间限制：`tle/`、`tle2/`、`java-tle/`
- 运行异常：`segmentfault/`、`sigtrap/`、`zero/`
- 系统调用与环境限制：`socket/`、`stack/`
- 输出限制：`ole/`
- Java 内存场景：`java-mle/`

## 常见检索入口

- 想加新的运行期回归用例：参考现有子目录结构，新建一个最小目录即可
- 想修改预期状态码：改对应目录下的 `case.json`
- 想看 Java 需要额外放行哪些 syscall：对比 `java/`、`java-tle/`、`java-mle/` 的 `case.json`
- 想看测试总入口如何串起来：看仓库根目录 `makefile` 的 `testall` 目标，以及 [`../cmd/test/`](../cmd/test/)

## 不要和这个目录混淆

- 编译阶段 fixture 在 [`../cmd/compiler/tests/`](../cmd/compiler/tests/)
- 设计说明文档在 [`../docs/`](../docs/)
