# CPL — CLI Protocol Language v1.0.0

> **CPL (Command Protocol Language)** 是 UniCLI OS 的核心协议，定义了如何描述 CLI 命令、如何链式调用、以及如何在沙箱中执行。

---

## 1. 概述

CPL 定义了三个层次：

| 层次 | 文件 | 说明 |
|------|------|------|
| **描述层** | `.cpl.json` manifest | 描述一个命令的输入/输出/资源/镜像 |
| **协议层** | `cpl.proto` | Protobuf 序列化格式，用于管道数据传输 |
| **执行层** | Runner | Docker 沙箱执行器，容器隔离 |

### 核心设计原则

1. **一切皆 CLI** — 任何操作都是命令行命令
2. **按需生成** — AI 编译自然语言为 CPL manifest
3. **用完即焚** — 沙箱自动清理，不留痕迹
4. **安全隔离** — 容器无网络、只读根文件系统、非 root 用户

---

## 2. Manifest 格式 (.cpl.json)

### 2.1 结构

```json
{
  "name": "image.resize",
  "version": "1.0.0",
  "description": "Resize an image to specified dimensions",
  "inputs": [
    {"name": "input", "type": "FILE", "required": true, "description": "Input image"},
    {"name": "width", "type": "INT", "required": false, "default": "800"}
  ],
  "outputs": [
    {"name": "output", "type": "FILE", "description": "Resized image"}
  ],
  "resources": {"cpu_cores": 0.5, "memory_mb": 128},
  "network": "none",
  "image_ref": "ghcr.io/unicli/image.resize:1.0.0",
  "entrypoint": "/bin/resize",
  "workdir": "/workspace"
}
```

### 2.2 字段说明

| 字段 | 必需 | 类型 | 说明 |
|------|------|------|------|
| `name` | ✅ | string | 命令名，点号分隔，如 `image.resize` |
| `version` | ✅ | string | 语义版本号 SemVer |
| `description` | ✅ | string | 人类可读的描述 |
| `inputs` | - | array | 输入参数列表 |
| `outputs` | - | array | 输出列表 |
| `resources` | - | object | 资源需求 |
| `network` | - | enum | `none`/`outbound`/`full`，默认 `none` |
| `image_ref` | ✅ | string | OCI 容器镜像引用 |
| `entrypoint` | ✅ | string | 容器入口点路径 |
| `workdir` | - | string | 工作目录，默认 `/workspace` |
| `env` | - | object | 环境变量 |
| `mounts` | - | array | 卷挂载声明 |
| `signature` | - | object | Ed25519 签名 |
| `metadata` | - | object | 作者、许可证等 |

### 2.3 Input Type 枚举

| 类型 | 说明 | 示例值 |
|------|------|--------|
| `STRING` | 字符串 | `"hello"` |
| `INT` | 整数 | `"800"` |
| `FLOAT` | 浮点数 | `"3.14"` |
| `BOOL` | 布尔 | `"true"` |
| `FILE` | 文件路径 | `"/workspace/input.jpg"` |
| `ENUM` | 枚举 | 需要 `enum_values` 字段 |
| `STREAM` | 流输入 | 管道数据输入 |

### 2.4 Output Type 枚举

| 类型 | 说明 |
|------|------|
| `TEXT` | 文本输出（stdout） |
| `FILE` | 文件输出 |
| `STREAM` | 流式输出 |
| `STRUCT` | Protobuf 结构化数据 |
| `EVENT` | 事件流（SSE） |

---

## 3. 管道协议 (Pipeline)

### 3.1 基本原理

两个 CLI 之间通过 `|` 管道连接时，数据使用 Protobuf 序列化的 `PipeMessage` 帧进行传输。

```
CLI A (stdout) ───→ Protobuf PipeMessage ───→ CLI B (stdin)
```

### 3.2 帧格式

每帧采用 **varint length-prefixed** 编码：

```
┌─────────────────────────────────────────────┐
│ Varint Length (4 bytes max)                 │
├─────────────────────────────────────────────┤
│ Protobuf PipeMessage (serialized)           │
│   ├── PipeData (output_name + content)      │
│   ├── PipeSignal (progress/started/done)    │
│   └── PipeError (error code + message)      │
└─────────────────────────────────────────────┘
```

### 3.3 Go SDK 编解码

```go
// 编码 PipeMessage 帧
func EncodeFrame(w io.Writer, msg *cplpb.PipeMessage) error {
    data, err := proto.Marshal(msg)
    if err != nil {
        return err
    }
    buf := make([]byte, binary.MaxVarintLen64)
    n := binary.PutUvarint(buf, uint64(len(data)))
    if _, err := w.Write(buf[:n]); err != nil {
        return err
    }
    _, err = w.Write(data)
    return err
}

// 解码 PipeMessage 帧
func DecodeFrame(r io.Reader) (*cplpb.PipeMessage, error) {
    length, err := binary.ReadUvarint(r)
    if err != nil {
        return nil, err
    }
    data := make([]byte, length)
    if _, err := io.ReadFull(r, data); err != nil {
        return nil, err
    }
    msg := &cplpb.PipeMessage{}
    if err := proto.Unmarshal(data, msg); err != nil {
        return nil, err
    }
    return msg, nil
}

// 从 Manifest 文件到执行
func RunFromManifest(path string, args map[string]string) error {
    m, err := cpl.LoadFile(path)
    if err != nil {
        return err
    }
    // 校验兼容性
    for _, in := range m.Inputs {
        if in.Required {
            if _, ok := args[in.Name]; !ok {
                return fmt.Errorf("required input %q not provided", in.Name)
            }
        }
    }
    // ... 执行逻辑
    return nil
}
```

---

## 4. 沙箱执行模型

```
用户请求: unicli run image.resize --input photo.jpg --width 800
                              │
                              ▼
  1. 解析表达式 → 查找 manifest → 解析参数
                              │
                              ▼
  2. 创建临时工作目录 (mktemp)
                              │
                              ▼
  3. Docker 创建沙箱容器:
     - 镜像: ghcr.io/unicli/image.resize:1.0.0
     - 挂载: /tmp/xxxx → /workspace
     - 网络: --network none
     - 内存: --memory 128m
     - CPU:  --cpus 0.5
     - 只读根文件系统
     - 非 root 用户 (uid=1000)
     - 自动删除 (--rm)
                              │
                              ▼
  4. 执行 entrypoint + 参数
                              │
                              ▼
  5. 等待退出 → 获取日志 → 复制输出文件
                              │
                              ▼
  6. 自动清理容器 + 工作目录
```

---

## 5. 错误码

| 代码 | 错误 | 阶段 | 重试策略 |
|------|------|------|---------|
| 1 | 通用错误 | 所有 | 检查参数 |
| 2 | 输入文件不存在 | 启动 | 重试 + 检查路径 |
| 3 | 输入类型不匹配 | 解析 | 修复参数类型 |
| 4 | 资源不足（OOM） | 运行 | 重试 + 更多资源 |
| 5 | 超时 | 运行 | 检查性能 + 增大超时 |
| 6 | 镜像拉取失败 | 启动 | 重试 + 检查网络 |
| 7 | 容器创建失败 | 启动 | 重试 |
| 8 | 管道解码错误 | 管道 | 重试 |
| 9 | 签名校验失败 | 验证 | 检查签名 |
| 10 | 内部错误 | 任意 | 报告开发者 |

---

## 6. 合规性检查清单

实现者逐条确认：

- [ ] Manifest JSON 符合 Schema 校验
- [ ] 所有 required 输入在运行前校验
- [ ] 容器以 non-root 用户运行
- [ ] 网络默认隔离 (--network none)
- [ ] 只读根文件系统 (ReadonlyRootfs)
- [ ] 容器自动清理 (AutoRemove)
- [ ] 临时工作目录清理
- [ ] 超时控制（默认 5 分钟）
- [ ] 管道帧的 varint length-prefixed 编码
- [ ] 管道数据正确序列化/反序列化
- [ ] 签名验证（如果 manifest 包含 signature）
- [ ] 环境变量不得泄露敏感信息

---

## 7. 版本兼容性策略

| 版本号变更 | 含义 | 示例 |
|-----------|------|------|
| MAJOR | 不兼容的协议变更 | 1.x → 2.0 |
| MINOR | 向下兼容的新增功能 | 1.0 → 1.1 |
| PATCH | 向下兼容的问题修正 | 1.0.0 → 1.0.1 |

Runner 必须兼容同一个 MAJOR 版本内的所有 MINOR/PATCH 变更。
