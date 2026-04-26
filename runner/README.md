# runner

该目录是仓库核心库，负责运行 `case.json` 描述的任务，并在 Linux 上完成资源限制、沙箱隔离、ptrace syscall 检查、时间内存采集与结果归类。

## 主要职责

- 解析并校验运行配置
- 启动子进程并设置资源限制、工作目录、IO 重定向
- 应用沙箱隔离能力，如 `chroot`、namespace、`no_new_privs`、降权
- 使用 ptrace 跟踪 syscall，并结合 allowlist / one-time policy 做拦截
- 采集时间与内存数据，输出统一 `Result`

## 关键文件

- `config.go`：`TaskConfig`，即 `case.json` 的主要字段定义与校验逻辑。
- `exec.go`：`RunningTask` 生命周期，串起启动、trace、限额检查和结果判定。
- `exec_linux.go`：Linux 下子进程启动细节，包括资源限制、IO 重定向与沙箱应用。
- `cgroup_linux.go`：Linux 下的 cgroup v2 memory controller 接入，包括父 cgroup 选择、task cgroup 创建、指标读取与清理。
- `process*.go`：对子进程 `wait`、ptrace 选项、状态判断的封装。
- `tracer*.go`：syscall 读取与判定逻辑，核心标识符是 `TracerDetect`。
- `sec.go`：把 syscall 名称列表转换成可执行的 `CallPolicy`。
- `sandbox_linux.go`：namespace、`chroot`、`no_new_privs`、降权顺序的统一入口。
- `memory*.go`：内存采集逻辑。
- `result.go`：评测状态码及 signal 到状态码的映射。
- `*_darwin.go`：macOS 占位实现，方便开发时通过编译，但不提供真正的 Linux 沙箱语义。

## 资源限制契约

- `Memory` 是 Linux task cgroup 的总内存预算，直接映射到 cgroup v2 `memory.max`。
- `MemoryReserve` 已废弃，只为兼容旧 `case.json` 保留，Linux runtime 不再使用。
- `Stack` 独立映射到 `RLIMIT_STACK`，不再与 `Memory` 复用同一上限。
- 更完整的字段语义、单位和 signal 归类说明见 [`../docs/runner/resource-limits.md`](../docs/runner/resource-limits.md)。

## 常见检索入口

- 想新增 `case.json` 配置项：先看 `config.go`
- 想修改资源限制或 IO 文件：先看 `exec_linux.go`
- 想调整 syscall 允许策略：先看 `sec.go`、`tracer_linux.go`，再看 [`../sec/`](../sec/)
- 想调整沙箱执行顺序：先看 `sandbox_linux.go`
- 想调整结果状态或信号映射：先看 `result.go`

## 关键标识符

- `TaskConfig`
- `RunningTask`
- `Result`
- `TracerDetect`
- `CallPolicy`
- `SandboxConfig`

## 相关目录

- syscall 名称表在 [`../sec/`](../sec/)
- 设计分析文档在 [`../docs/`](../docs/)
- 集成运行用例在 [`../tests/`](../tests/)
