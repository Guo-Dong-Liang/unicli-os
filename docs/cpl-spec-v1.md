# CPL Protocol Specification v1.0

> **CPL = Command Pipeline Language**
> 定义：一种用于描述、组合和编排 CLI 工具的结构化协议标准。
> 协议版本：1.0.0 | 状态：草案 | 最后更新：2025-05-21

---

## 1. 设计目标

1. **可描述性** — 任何 CLI 工具都可以用 CPL manifest 描述其输入、输出、资源需求和执行入口
2. **可组合性** — CLI 工具可以通过结构化管道链式调用，数据以 protobuf 格式在管道中传递
3. **沙箱隔离** — 每个 CLI 在独立的容器沙箱中运行，默认无网络、只读文件系统
4. **类型安全** — 输入输出类型由 manifest 声明，运行时校验类型匹配
5. **零依赖部署** — SDK 提供 Go/Python 两种实现，Go SDK 编译为单一二进制

---

## 2. 核心概念

### 2.1 Manifest (CLI 描述文件)

每个 CLI 工具由一个 `.cpl.json` 文件描述，包含：
- 元数据（name, version, description, author）
- 协议版本声明（cpl_version）
- 输入声明（inputs）— 参数名、类型、默认值、标志
- 输出声明（outputs）— 输出名、类型、MIME
- 资源需求（resources）— CPU、内存、网络、超时
- 容器引用（image — ref, entrypoint, workdir）
- 镜像签名（signature）— digest 校验

### 2.2 结构化管道

管道通信使用 protobuf 序列化 + stdio 传输：

```
CLI A (stdout) --[StructuredMessage frames over stdout]--> CLI B (stdin)
```

每条消息是一个 `StructuredMessage` protobuf 消息，包含：
- 消息类型（DATA / ERROR / METADATA / EOS）
- 输出名称（对应 manifest 中声明的 output name）
- 实际数据（payload）
- MIME 类型指示
- 序列号（用于有序重组）

### 2.3 沙箱运行时

每个 CLI 在 Docker 容器中运行：
- 默认 `--network none`（除非 manifest 声明需要网络）
- 运行在临时工作目录，映射宿主机的输入/输出路径
- 以非 root 用户运行（默认 `nobody:nogroup`）
- 超时控制（默认 30 秒，可配置）
- 容器自动清理（无论成败）

---

## 3. 数据类型系统

### 3.1 输入类型

| 类型 | 描述 | JSON 示例 | Protobuf 类型 |
|------|------|-----------|---------------|
| FILE | 文件路径，宿主机会挂载到容器内 | `"/path/to/input.jpg"` | string |
| STRING | 字符串值 | `"hello"` | string |
| INT | 整数 | `800` | int64 |
| FLOAT | 浮点数 | `3.14` | double |
| BOOLEAN | 布尔值 | `true` | bool |
| ENUM | 枚举值 | `"high"` | string (with enum constraint) |
| STREAM | 流式数据（stdin） | `-` | bytes |

### 3.2 输出类型

| 类型 | 描述 | 管道行为 |
|------|------|----------|
| FILE | 输出文件路径 | 管道模式：传递文件路径字符串，由下游自行读取 |
| TEXT | 文本输出 | 作为 protobuf 消息的 payload 传递 |
| STREAM | 流式输出 | 逐块发送 StructuredMessage，下游实时处理 |
| STRUCT | 结构化数据（JSON/protobuf） | 完整的 protobuf 结构化消息 |

### 3.3 资源声明

| 字段 | 类型 | 默认值 | 范围 | 说明 |
|------|------|--------|------|------|
| cpu | float | 1.0 | 0.1 - 64 | CPU 核心数 (可以小数) |
| memory | int | 128 | 16 - 65536 | 内存限制 (MB) |
| network | bool | false | - | 是否允许网络访问 |
| gpu | bool | false | - | 是否需要 GPU |
| timeout | int | 30 | 1 - 3600 | 超时时间 (秒) |
| disk | int | 256 | 16 - 65536 | 临时磁盘空间 (MB) |

---

## 4. Protobuf 消息定义

完整的 protobuf 定义见 `protos/cpl.proto`（包：`cpl.v1`，Go 包：`github.com/Guo-Dong-Liang/unicli-os/pkg/cpl/v1`）。

### 4.1 Manifest (JSON 序列化)

序列化为 JSON 格式的 Manifest，存储为 `.cpl.json` 文件。JSON Schema 见 `schemas/cpl-manifest.schema.json`。

```json
{
  "cpl_version": "1.0.0",
  "name": "image.resize",
  "version": "1.0.0",
  "description": "Resize an image to specified dimensions",
  "author": "UniCLI Team",
  "inputs": [
    {
      "name": "input",
      "type": "FILE",
      "required": true,
      "description": "Input image path"
    }
  ],
  "outputs": [
    {
      "name": "output",
      "type": "FILE",
      "description": "Resized image"
    }
  ],
  "resources": {
    "cpu": 0.5,
    "memory": 256,
    "network": false,
    "timeout": 60
  },
  "image": {
    "ref": "ghcr.io/Guo-Dong-Liang/image.resize:1.0.0",
    "entrypoint": "/app/resize.sh",
    "workdir": "/workspace",
    "user": "nobody:nogroup"
  },
  "signature": {
    "algorithm": "SHA256",
    "digest": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
  }
}
```

表 4-1: Manifest JSON 字段说明

| 字段 | 类型 | 必须 | 说明 |
|------|------|------|------|
| cpl_version | string | 是 | CPL 协议版本，格式：`MAJOR.MINOR.PATCH` |
| name | string | 是 | 命令名称，点分隔命名空间（如 `image.resize`） |
| version | string | 是 | CLI 工具版本，semver 格式 |
| description | string | 否 | 人类可读的描述 |
| author | string | 否 | 作者/维护者 |
| inputs | array[Input] | 否 | 输入声明（可为空，最小 0） |
| outputs | array[Output] | 是 | 输出声明（至少 1 个） |
| resources | ResourceRequirements | 否 | 资源需求 |
| image | ImageSpec | 是 | 容器镜像规格 |
| signature | Signature | 否 | 镜像签名 |

### 4.2 StructuredMessage (protobuf 序列化，线格式)

这是管道通信的核心消息格式。完整的 protobuf 定义见 `protos/cpl.proto`。

```protobuf
message StructuredMessage {
  MessageType type = 1;         // DATA | ERROR | METADATA | EOS
  string output_name = 2;       // 对应 manifest outputs 中的 name
  bytes payload = 3;            // 实际数据
  string mime_type = 4;         // MIME 类型指示
  uint64 sequence = 5;          // 序列号，用于有序重组
  map<string, string> metadata = 6; // 附加元数据
}

enum MessageType {
  MESSAGE_TYPE_UNSPECIFIED = 0;
  MESSAGE_TYPE_DATA = 1;        // 数据块
  MESSAGE_TYPE_ERROR = 2;       // 错误信息
  MESSAGE_TYPE_METADATA = 3;    // 元数据
  MESSAGE_TYPE_EOS = 4;         // 流结束信号
}
```

### 4.3 线格式 (Wire Format) — Varint Length-Prefixed Frames

StructuredMessage 通过 stdio 传输时，采用 **varint length-prefixed** 帧格式：

```
+----------------+----------------------------+
| Varint Length  | Protobuf 序列化消息体       |
| (1-10 bytes)   | (N bytes)                  |
+----------------+----------------------------+
```

**编码规则：**
1. 将 StructuredMessage 序列化为 protobuf 二进制
2. 计算消息体长度 N（bytes）
3. 将 N 编码为 Base128 Varint（Protocol Buffers 标准 varint 编码）
4. 写入 varint + 消息体到 stdout（或 stdin）

**解码规则：**
1. 从 stdin 读取 varint 编码的长度 N
2. 读取接下来的 N 个字节
3. 反序列化为 StructuredMessage
4. 检查 type 字段：若为 EOS，结束读取

**Go SDK 实现参考：**
```go
// 编码
func WriteMessage(w io.Writer, msg *cplpb.StructuredMessage) error {
    data, _ := proto.Marshal(msg)
    varint := proto.EncodeVarint(uint64(len(data)))
    w.Write(varint)
    w.Write(data)
    return nil
}

// 解码
func ReadMessage(r io.Reader) (*cplpb.StructuredMessage, error) {
    length, _ := proto.DecodeVarint(r) // 读取 varint
    data := make([]byte, length)
    io.ReadFull(r, data)
    msg := &cplpb.StructuredMessage{}
    proto.Unmarshal(data, msg)
    return msg, nil
}
```

### 4.4 CommandResult

CLI 执行完成后的结果摘要：

```protobuf
message CommandResult {
  int32 exit_code = 1;
  string stdout_summary = 2;          // stdout 截断摘要
  string stderr_summary = 3;          // stderr 截断摘要
  map<string, string> outputs = 4;    // output name → 实际路径/值
  double duration_seconds = 5;
  bool timed_out = 6;
  bool oom_killed = 7;                // 是否被 OOM killer 终止
  int64 output_bytes = 8;             // 输出数据总大小
}
```

### 4.5 PipeSpec (管道描述)

PipeSpec 是链式调用内部表示的标准格式，用于描述多阶段管道的拓扑结构：

```protobuf
message PipeSpec {
  repeated Stage stages = 1;    // 按顺序执行的阶段
}

message Stage {
  string command_name = 1;          // CLI 命令名
  string version = 2;               // 版本约束（可选）
  map<string, string> args = 3;     // 传递给该阶段的参数 {name: value}
  map<string, string> input_mapping = 4; // 上游输出 → 本阶段输入的映射
}
```

管道的解析规则：
1. `unicli run "A | B | C"` 解析为 3 个 Stage
2. 第一个 Stage 的 input_mapping 为空或映射自 CLI 入参
3. 后续 Stage 的 input_mapping 默认将上游第一个输出映射到本阶段的第一个输入
4. 显式映射语法：`A.output_name > B.input_name`（未来扩展）

---

## 5. 管道协议细节

### 5.1 单 CLI 执行流程

```
unicli run image.resize --input photo.jpg --width 800
```

1. **解析 manifest** — 从 registry 加载 `image.resize` 的 manifest
2. **类型校验** — 检查 `--input` 类型匹配 `FILE`，`--width` 匹配 `INT`
3. **默认值填充** — 未提供 `--quality` 则使用 manifest 中的 default
4. **Pull 镜像** — 如果本地缓存未命中 `image.resize:1.0.0`
5. **创建挂载** — 创建临时工作目录，解析 input FILE 路径为容器内路径
6. **启动容器** — `docker run --network none --read-only --cap-drop=ALL ...`
7. **等待完成** — 超时控制（默认 30s）
8. **收集输出** — 将 output FILE 从容器复制到宿主机
9. **清理** — 删除临时容器和目录
10. **返回 CommandResult**

### 5.2 链式管道执行流程

```
unicli run "image.resize --width 800 | image.grayscale" --input photo.jpg
```

1. **解析管道表达式** — 按 `|` 拆分左右两侧
2. **解析两侧 manifest** — 分别加载 `image.resize` 和 `image.grayscale`
3. **类型兼容性校验** — 左侧输出类型必须与右侧输入类型匹配
4. **创建管道通道** — 创建 FIFO (named pipe) 作为中间缓冲区
5. **并行启动沙箱** — 同时启动两个容器
6. **数据流** — 左侧容器 stdout → StructuredMessage 帧 → FIFO → 右侧容器 stdin
7. **收集结果** — 两侧完成后收集最终输出
8. **返回最终的 CommandResult**（最后一个 Stage 的结果）

### 5.3 管道类型兼容性矩阵

| 左侧输出 \\ 右侧输入 | FILE | TEXT | STREAM | STRUCT |
|----------------------|------|------|--------|--------|
| FILE | 传递文件路径字符串 | 自动读取文件内容为 TEXT | -- | 自动读取文件转为 STRUCT |
| TEXT | 写入临时文件 | ✅ 直接传递 | ✅ 直接传递 | 尝试反序列化 JSON |
| STREAM | -- | ✅ 逐块传递 | ✅ 逐块传递 | -- |
| STRUCT | 写入临时文件 | 序列化为 JSON 文本 | ✅ 逐字段序列化 | ✅ 直接传递 protobuf |

**规则说明：**
- ✅ = 直接兼容，无损转换
- 文字描述 = 有损转换，运行时发出 WARNING
- `--` = 不兼容，抛出错误 PIPE_TYPE_MISMATCH

---

## 6. 安全规范

### 6.1 沙箱安全策略

1. **默认无网络** — `--network none`，除非 manifest 显式声明 `resources.network = true`
2. **只读根文件系统** — `--read-only`，仅工作目录可写
3. **非 root 用户** — 容器内以 `nobody:nogroup` (uid=65534) 运行
4. **资源限制** — 限制 CPU、内存、磁盘（依据 manifest 声明）
5. **Capability 剥离** — `--cap-drop=ALL`
6. **自动清理** — 无论执行成功失败，容器和临时目录在超时后自动清理
7. **禁止特权容器** — 不允许 `--privileged` 模式
8. **只读 /etc 和 /var** — 防止修改系统配置

### 6.2 镜像签名

每个镜像 Manifest 应包含 SHA256 digest：
- Runner 在 pull 镜像后验证 digest 匹配
- Digest 不匹配时拒绝执行
- 支持 GPG 签名扩展（`algorithm: "GPG"`）

---

## 7. 错误处理

### 7.1 错误码表

| 错误场景 | 错误码 | 阶段 | 行为 |
|----------|--------|------|------|
| Manifest JSON 格式错误 | EXIT_MANIFEST_PARSE (1) | 解析 | 验证失败，输出 JSON 解析错误详情 |
| Manifest 字段缺失 | EXIT_MANIFEST_VALIDATE (2) | 解析 | 输出字段路径+期望值 |
| 输入类型不匹配 | EXIT_INPUT_TYPE (3) | 校验 | 声明期望类型 vs 实际类型 |
| 缺少必需输入 | EXIT_MISSING_INPUT (4) | 校验 | 输出缺少的参数名列表 |
| 容器镜像拉取失败 | EXIT_IMAGE_PULL (10) | 运行时 | 自动重试 3 次（指数退避），全部失败后报错 |
| 管道类型不兼容 | EXIT_PIPE_TYPE_MISMATCH (11) | 管道 | 输出 incompatible 的输出-输入类型对 |
| 容器超时 | EXIT_TIMEOUT (20) | 运行时 | Kill 容器，返回 partial CommandResult |
| 容器 OOM | EXIT_OOM (21) | 运行时 | 检测 OOM kill 信号 |
| 容器非零退出 | EXIT_NONZERO (30+) | 运行时 | 返回 exit code + stderr |
| 镜像 digest 不匹配 | EXIT_SIGNATURE (40) | 安全 | 输出期望 digest vs 实际 digest |

### 7.2 重试策略

| 操作 | 重试次数 | 退避策略 | 超时 |
|------|----------|----------|------|
| 容器镜像拉取 | 3 | 指数退避（1s, 2s, 4s） | 120s |
| 容器创建 | 2 | 固定 1s | 10s |
| 文件挂载 | 0 | - | - |

### 7.3 错误输出格式

CLI 错误信息统一通过 stderr 输出，格式为：

```
[ERROR] <错误码> <错误描述> | <详细信息>
```

示例：
```
[ERROR] PIPE_TYPE_MISMATCH Cannot chain image.resize:output(FILE) with text.upper:input(STRING) | FILE→STRING requires file content reading, use '--force' to override
```

---

## 8. 目录结构

```
~/.unicli/
├── registry/                    # 本地注册的 CLI manifests
│   ├── image.resize@1.0.0.cpl.json
│   └── image.grayscale@0.1.0.cpl.json
├── cache/
│   ├── images/                  # Docker 镜像层缓存
│   │   ├── sha256:abc.../
│   │   └── sha256:def.../
│   └── workdirs/               # 运行时挂载的临时目录
│       └── unicli-xxxxx/       # 自动清理
└── config.yaml                 # 全局配置
```

---

## 9. SDK 快速开始 (Go)

### 9.1 解析 Manifest

```go
package main

import (
    "os"
    "google.golang.org/protobuf/encoding/protojson"
    cplpb "github.com/Guo-Dong-Liang/unicli-os/pkg/cpl/v1"
)

func main() {
    data, _ := os.ReadFile("image.resize.cpl.json")
    manifest := &cplpb.Manifest{}
    protojson.Unmarshal(data, manifest)
    // 使用 manifest.Name, manifest.Inputs 等
}
```

### 9.2 构造并发送管道消息

```go
import (
    "os"
    "google.golang.org/protobuf/proto"
    cplpb "github.com/Guo-Dong-Liang/unicli-os/pkg/cpl/v1"
)

// 编码到 stdout
msg := &cplpb.StructuredMessage{
    Type:       cplpb.MessageType_MESSAGE_TYPE_DATA,
    OutputName: "output",
    Payload:    []byte("processed data"),
    MimeType:   "text/plain",
    Sequence:   1,
}
data, _ := proto.Marshal(msg)
varint := proto.EncodeVarint(uint64(len(data)))
os.Stdout.Write(varint)
os.Stdout.Write(data)
// 发送 EOS
eos, _ := proto.Marshal(&cplpb.StructuredMessage{Type: cplpb.MessageType_MESSAGE_TYPE_EOS})
ev := proto.EncodeVarint(uint64(len(eos)))
os.Stdout.Write(ev)
os.Stdout.Write(eos)
```

### 9.3 管道兼容性校验

```go
func CheckPipeCompatibility(left *cplpb.Output, right *cplpb.Input) error {
    // 判断 left.Type → right.Type 的兼容性
    switch left.Type {
    case cplpb.OutputType_OUTPUT_TYPE_TEXT:
        if right.Type == cplpb.InputType_INPUT_TYPE_FILE {
            return nil // 自动写入临时文件
        }
    // ... 实现完整兼容性矩阵
    }
}
```

---

## 10. 版本控制

- **当前版本**：`1.0.0`
- **协议版本独立于 SDK 版本** — manifest 中通过 `cpl_version` 字段声明
- **兼容性策略**：
  - PATCH 升级（1.0.0 → 1.0.1）：完全向后兼容，仅修正/澄清
  - MINOR 升级（1.0.0 → 1.1.0）：新增字段或类型，旧版 runner 忽略未知字段
  - MAJOR 升级（1.0.0 → 2.0.0）：不兼容变更，manifest 声明新版本
- **Runner 策略**：Runner 支持解析 `cpl_version >= 1.0.0` 的旧版本

---

## 附录 A：规范合规性检查清单

CPL v1.0.0 实现必须通过以下检查点：

- [ ] Manifest JSON 解析支持字段见 4.1
- [ ] 对未知字段静默忽略（forward compatibility）
- [ ] Varint length-prefixed framed 消息编码/解码（见 4.3）
- [ ] 支持全部 7 种输入类型（见 3.1）
- [ ] 支持全部 4 种输出类型（见 3.2）
- [ ] 管道兼容性矩阵实现（见 5.3）
- [ ] EOS 信号终止管道
- [ ] 默认无网络沙箱
- [ ] 非 root 用户运行
- [ ] 超时控制 + kill
- [ ] digest 校验
- [ ] 错误码表实现（见 7.1）

---

## 附录 B：变更记录

| 版本 | 日期 | 变更内容 |
|------|------|----------|
| 1.0.0 | 2025-05-21 | 初始版本 — Manifest 定义、StructuredMessage 管道协议、沙箱安全规范 |

---

*本文档对应 Sprint 1 Task 1 交付物。配套文件：`protos/cpl.proto`、`schemas/cpl-manifest.schema.json`、`examples/image.resize.cpl.json`。*
