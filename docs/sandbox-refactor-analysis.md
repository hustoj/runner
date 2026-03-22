# 沙箱重构方案分析与改进

## 当前方案评估

### ✅ 优点

1. **职责收敛清晰**
   - 把散落的降权逻辑统一到 `applySandbox()` 单一入口
   - 各子步骤职责明确：namespace、rootfs、no_new_privs、drop privileges

2. **执行顺序正确**
   - namespace → rootfs → no_new_privs → drop privileges
   - 符合 Linux 安全模型的依赖关系

3. **错误传播完整**
   - 各步骤都返回 `error`，可追溯失败原因
   - 使用 `fmt.Errorf` 包装，保留调用栈

4. **配置明确化**
   - 通过 `SandboxConfig` 显式声明所有沙箱参数
   - 默认值保守（namespace 默认关闭，no_new_privs 默认开启）

---

## 解决方案：统一沙箱入口

### 设计原则

**核心思想**：把安全步骤从"散落的可选 helper"改成"主路径强制执行"

```
Before:
fork() → limitResource() → redirectIO() → ptraceTraceme() → exec()
         ❌ 没有任何降权/隔离

After:
fork() → limitResource() → redirectIO() → applySandbox() → ptraceTraceme() → exec()
                                            ✅ 统一安全入口
```

### 实现要点

#### 1. 创建统一配置结构

```go
// 文件：runner/sandbox_linux.go

type SandboxConfig struct {
    UID        int     // 运行用户 (-1 表示不降权)
    GID        int     // 运行组
    ChrootDir  string  // chroot 根目录
    WorkDir    string  // 工作目录
    NoNewPrivs bool    // 是否禁止提权
    UseMountNS bool    // 是否使用 mount namespace
    UsePIDNS   bool    // 是否使用 PID namespace
    UseIPCNS   bool    // 是否使用 IPC namespace
    UseUTSNS   bool    // 是否使用 UTS namespace
    UseNetNS   bool    // 是否使用 network namespace
}
```

#### 2. 定义执行顺序

```go
func applySandbox(cfg SandboxConfig) error {
    // 步骤1: 创建 namespace (需要特权)
    if err := setupNamespaces(cfg); err != nil {
        return err
    }

    // 步骤2: 切换 rootfs (需要 CAP_SYS_CHROOT)
    if err := setupRootFS(cfg); err != nil {
        return err
    }

    // 步骤3: 设置 no_new_privs (不可逆)
    if err := setNoNewPrivs(cfg); err != nil {
        return err
    }

    // 步骤4: 降权 (不可逆)
    if err := dropPrivileges(cfg); err != nil {
        return err
    }

    return nil
}
```

**为什么这个顺序？**

| 步骤 | 必须在之前 | 原因 |
|------|-----------|------|
| namespace | chroot | `unshare()` 需要特权 |
| chroot | 降权 | `chroot()` 需要 CAP_SYS_CHROOT |
| no_new_privs | 降权 | 一旦设置不可撤销，降权前设置可防止意外提权 |
| 降权 | 最后 | `setuid` 后失去所有特权，不能再做其他操作 |

#### 3. 实现各子步骤

**降权（最核心）**：

```go
func dropPrivileges(cfg SandboxConfig) error {
    if cfg.UID < 0 && cfg.GID < 0 {
        return nil  // 不需要降权
    }

    // 1. 清空附加组（防止通过组权限访问资源）
    if err := syscall.Setgroups([]int{}); err != nil {
        return err
    }

    // 2. 切换 GID
    if err := syscall.Setgid(cfg.GID); err != nil {
        return err
    }

    // 3. 切换 UID（必须最后，不可逆）
    if err := syscall.Setuid(cfg.UID); err != nil {
        return err
    }

    return nil
}
```

**no_new_privs（防止提权）**：

```go
func setNoNewPrivs(cfg SandboxConfig) error {
    if !cfg.NoNewPrivs {
        return nil
    }

    // PR_SET_NO_NEW_PRIVS = 38
    _, _, errno := syscall.Syscall6(
        syscall.SYS_PRCTL,
        38, 1, 0, 0, 0, 0,
    )
    if errno != 0 {
        return errno
    }
    return nil
}
```

**namespace 隔离**：

```go
func setupNamespaces(cfg SandboxConfig) error {
    flags := 0
    if cfg.UseMountNS { flags |= syscall.CLONE_NEWNS }
    if cfg.UsePIDNS   { flags |= syscall.CLONE_NEWPID }
    if cfg.UseIPCNS   { flags |= syscall.CLONE_NEWIPC }
    if cfg.UseUTSNS   { flags |= syscall.CLONE_NEWUTS }
    if cfg.UseNetNS   { flags |= syscall.CLONE_NEWNET }

    if flags == 0 {
        return nil
    }

    return syscall.Unshare(flags)
}
```

#### 4. 接入主执行路径

```go
// 文件：runner/exec_linux.go

func (task *RunningTask) runProcess() {
    pid := fork()
    if pid == 0 {
        // 子进程
        task.limitResource()
        task.redirectIO()

        // ✅ 新增：强制执行沙箱设置
        if err := applySandbox(task.sandboxConfig()); err != nil {
            // ⚠️ fork 后不能用 panic，只能用 syscall.Exit
            fmt.Fprintf(os.Stderr, "Sandbox failed: %v\n", err)
            syscall.Exit(1)
        }

        ptraceTraceme()
        syscall.Exec(...)
    }
    // 父进程继续...
}
```

---

## 改进效果

### 安全边界建立

| 维度 | 改进前 | 改进后 |
|------|--------|--------|
| **UID/GID** | 继承父进程 | 配置专用低权限账户 |
| **附加组** | 继承所有组 | 清空 `setgroups([])` |
| **提权防护** | ❌ 无 | ✅ `no_new_privs` 阻止 setuid binary |
| **文件系统** | 可见宿主机全部文件 | 可选 `chroot` 隔离 |
| **挂载表** | 可见宿主机挂载 | 可选 mount namespace |
| **进程树** | 可见所有进程 | 可选 PID namespace |
| **网络** | 可访问宿主网络 | 可选 network namespace |

### 配置灵活性

**默认配置（保守安全）**：
```go
RunUID: -1              // 不降权，适用于已在容器内运行
RunGID: -1
NoNewPrivs: true        // ✅ 默认开启，防止提权
UseMountNS: false       // 默认关闭，避免复杂性
UsePIDNS: false
UseIPCNS: false
UseUTSNS: false
UseNetNS: false
```

**生产推荐（完整隔离）**：
```json
{
  "RunUID": 65534,        // nobody
  "RunGID": 65534,
  "NoNewPrivs": true,
  "WorkDir": "/judge",
  "ChrootDir": "/judge/root",
  "UseMountNS": true,
  "UsePIDNS": true,
  "UseNetNS": true
}
```

### 代码质量提升

1. ✅ **降权步骤强制执行** - 不会被遗忘
2. ✅ **顺序固定** - 不会被错误配置
3. ✅ **失败即退出** - fail-closed 原则
4. ✅ **可测试** - 各步骤独立，可单元测试
5. ✅ **跨平台兼容** - Darwin 存根避免编译失败

---

## 与其他方案的对比

### 方案对比

| 方案 | 核心思想 | 优点 | 缺点 | 适用场景 |
|------|----------|------|------|----------|
| **A. 散落 helper** | 提供可选的降权函数，调用方自己决定是否用 | 灵活 | ❌ 容易被遗忘<br>❌ 步骤不完整<br>❌ 顺序可能错误 | ❌ 不推荐 |
| **B. 统一入口**<br>（当前方案） | 主路径强制调用 `applySandbox()`，步骤固定 | ✅ 不会被遗忘<br>✅ 顺序正确<br>✅ 简单易懂 | 扩展需修改代码 | ✅ **推荐** |
| **C. 步骤链接口** | `SandboxStep` 接口，可插拔步骤 | 高度可扩展<br>符合开闭原则 | ❌ 过度设计<br>❌ 步骤顺序可能被错配 | 仅当需要频繁变更步骤时 |
| **D. exec.Cmd** | 废弃 fork，改用 Go 标准库 | ✅ 解决 Go fork 问题<br>✅ 更符合 Go 习惯 | 大规模重构 | 长期目标 |

**结论**：当前方案 B 是最佳选择，方案 D 可作为长期演进方向。

---

## 未来演进方向

### 短期（1 周内）

1. **🔥 P0：清理环境变量和 FD**
   ```go
   // 只传必要环境变量
   env := []string{"PATH=/usr/bin:/bin"}

   // 关闭继承的 FD
   for fd := 3; fd < 256; fd++ {
       syscall.Close(fd)
   }
   ```

2. **✅ P1：添加集成测试**
   - 在 Docker 容器中测试实际沙箱行为
   - 验证降权、namespace、no_new_privs 效果
   - 测试常见的沙箱逃逸场景

### 中期（1 个月内）

3. **⬆️ P2：引入 seccomp-bpf**
   ```go
   func applySandbox(cfg SandboxConfig) error {
       setupNamespaces(cfg)
       setupRootFS(cfg)
       setNoNewPrivs(cfg)
       dropPrivileges(cfg)
       installSeccomp(cfg)  // 新增：内核态 syscall 过滤
       return nil
   }
   ```

4. **🔧 P2：补充 capability drop**
   ```go
   // 在 dropPrivileges 前清空所有 capability
   syscall.Prctl(PR_CAPBSET_DROP, ...)
   ```

### 长期（3-6 个月）

5. **🏗️ P3：重构到 exec.Cmd**
   - 彻底解决 Go raw fork 问题
   - 使用 `SysProcAttr` 设置所有沙箱参数
   - 代码更符合 Go 习惯

6. **📊 P3：引入 cgroup v2**
   - 替代 rlimit 做资源限制
   - 支持更精确的内存/CPU/IO 控制
关键技术细节

### 为什么 setgroups → setgid → setuid 的顺序不能变？

```go
// ✅ 正确顺序
syscall.Setgroups([]int{})  // 1. 先清空附加组
syscall.Setgid(gid)         // 2. 设置组 ID
syscall.Setuid(uid)         // 3. 最后设置用户 ID

// ❌ 如果反过来会怎样？
syscall.Setuid(uid)         // 1. 设置 UID 后失去特权
syscall.Setgid(gid)         // 2. ❌ 没有权限修改 GID（EPERM）
syscall.Setgroups([]int{})  // 3. ❌ 没有权限修改附加组（EPERM）
```

**原理**：
- `setuid(non-root)` 是**不可逆的**，执行后进程失去所有特权
- `setgid` 和 `setgroups` 都需要 `CAP_SETGID` 权限
- 因此必须在 `setuid` 前完成所有需要特权的操作

### 为什么 no_new_privs 必须在降权前设置？

```go
// ✅ 正确顺序
prctl(PR_SET_NO_NEW_PRIVS, 1)  // 先设置 no_new_privs
setuid(nobody)                  // 再降权

// ⚠️ 反过来也能工作，但没必要
setuid(nobody)                  // 先降权
prctl(PR_SET_NO_NEW_PRIVS, 1)  // 再设置（不需要特权）
```

**原理**：
- `PR_SET_NO_NEW_PRIVS` 不需要特权，任何时候都能设置
- 但通常在降权前设置，因为这样逻辑更清晰："先禁止提权，再放弃权限"
- 一旦设置，子进程永远无法通过 setuid binary 或 file capability 获得额外权限

### 为什么 namespace 必须在 chroot 前创建？

```go
// ✅ 正确顺序
unshare(CLONE_NEWNS)  // 1. 先创建 mount namespace
chroot("/jail")       // 2. 再 chroot

// ❌ 如果反过来
chroot("/jail")       // 1. 先 chroot
unshare(CLONE_NEWNS)  // 2. ❌ 可能失败（需要 CAP_SYS_ADMIN）
```

**原理**：
- `unshare()` 需要相应的 capability（如 `CAP_SYS_ADMIN`）
- `chroot()` 本身需要 `CAP_SYS_CHROOT`
- 如果先 chroot 再降权，可能失去创建 namespace 的权限

### Go fork 后为什么不能用 panic？

```go
pid := fork()
if pid == 0 {
    // ❌ 危险：fork 后 Go runtime 状态不一致
    panic("error")           // 可能死锁或崩溃
    fmt.Println("...")       // 可能死锁
    log.Error("...")         // 可能死锁

    // ✅ 安全：只用 async-signal-safe 函数
    fmt.Fprintf(os.Stderr, "error\n")  // 部分安全
    syscall.Exit(1)                     // 完全安全
}
```

**原理**：
- Go 是多线程运行时，`fork()` 只复制调用线程
- 子进程中其他 goroutine 全部消失，但它们持有的锁仍存在
- 如果 `panic` 或 `fmt` 需要某个锁，会永久死锁
- 最安全的做法是只调用 Linux `async-signal-safe` 函数

---

## 经验教训

### 1. **安全步骤不能是"可选能力"，必须是"强制策略"**

❌ **错误做法**：提供 helper 函数，让调用方决定是否使用
```go
// 提供了函数但没人调用 → 等于没有
func ChangeRunningUser(uid int) { ... }
```

✅ **正确做法**：封装成必经入口，失败即退出
```go
// 主路径强制调用 → 不会被遗忘
if err := applySandbox(cfg); err != nil {
    syscall.Exit(1)
}
```

### 2. **沙箱步骤顺序是有依赖的，不能随意调整**

依赖关系图：
```
namespace (需要特权)
    ↓
chroot (需要 CAP_SYS_CHROOT)
    ↓
no_new_privs (不需要特权，但逻辑上应先设置)
    ↓
setgroups → setgid → setuid (不可逆)
```

### 3. **配置验证要前移，不要等到运行时才发现问题**

❌ **错误做法**：
```go
// 运行时才发现 UID/GID 配置不匹配
setgroups([]int{})
setgid(-1)  // ❌ 失败：EINVAL
```

✅ **正确做法**：
```go
// 配置加载时就验证
func (tc *TaskConfig) Validate() error {
    if (tc.RunUID >= 0) != (tc.RunGID >= 0) {
        return errors.New("UID and GID must both be set")
    }
}
```

### 4. **错误消息要包含上下文，方便调试**

❌ **不好的错误消息**：
```go
return errors.New("setuid failed")
```

✅ **好的错误消息**：
```go
return fmt.Errorf("setuid(%d): %w", cfg.UID, err)
// 输出：setuid(65534): operation not permitted
```

---

## 总结

这次重构的核心价值在于：

1. **解决了根本性的安全问题**：子进程从"无隔离"变成"有完整的降权和可选的 namespace 隔离"
2. **建立了正确的架构**：从"散落的可选 helper"变成"主路径强制入口"
3. **留下了清晰的知识沉淀**：
   - 为什么需要降权/隔离
   - 各步骤为什么必须这个顺序
   - 如何安全地在 Go fork 后执行代码
   - 未来如何继续改进（环境变量、FD、seccomp）

**最重要的一点**：安全不是"可选功能"，而是"默认策略"。这次重构把安全步骤从"可能被遗忘的 helper"变成了"不可绕过的门禁"，这才是真正解决问题的方式
