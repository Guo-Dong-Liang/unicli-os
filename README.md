# UniCLI OS

**Universal CLI Operating System** — 一个结构化描述、沙箱运行、管道编排 CLI 工具的开放协议和运行时。

## 核心组件

| 组件 | 目录 | 状态 |
|------|------|------|
| CPL 协议规范 | `docs/cpl-spec-v1.md` | ✅ v1.0.0 |
| Protobuf 定义 | `protos/cpl.proto` | ✅ v1.0.0 |
| JSON Schema | `schemas/cpl-manifest.schema.json` | ✅ v1.0.0 |
| 沙箱运行器 | `pkg/runner/` | 🔧 待实现 |
| CLI Registry | `pkg/registry/` | 🔧 待实现 |
| CPL 解析器 | `pkg/cpl/` | 🔧 待实现 |
| 终端 CLI | `cmd/unicli/` | 🔧 待实现 |
| 验证工具 | `cmd/unicli-validate/` | 🔧 待实现 |

## 快速开始

```bash
# 构建
make build

# 运行一个 CLI
unicli run image.resize --input photo.jpg --width 800

# 链式调用
unicli run "image.resize --width 800 | image.grayscale" --input photo.jpg

# 注册一个 CLI
unicli registry install ghcr.io/unixcli/image.resize:1.0.0

# 查看已注册的 CLI
unicli registry list
```

## 技术栈

- **运行时引擎：** Go
- **沙箱：** Docker (Phase 1) → gVisor (Phase 2)
- **协议格式：** Protobuf + JSON
- **管道通信：** Protobuf over stdio (length-prefixed frames)
- **镜像格式：** OCI-compatible container images

## 项目结构

```
unicli-os/
├── cmd/                  # CLI 入口
│   ├── unicli/           # 主 CLI
│   └── unicli-validate/  # 验证工具
├── pkg/                  # 核心库
│   ├── runner/           # 沙箱运行器
│   ├── registry/         # CLI 注册表
│   ├── cpl/              # CPL 协议解析
│   └── validator/        # 验证器
├── protos/               # Protobuf 定义
├── examples/             # 示例 manifests
├── schemas/              # JSON Schema
├── docs/                 # 文档
├── benchmarks/           # 性能基准
└── scripts/              # 构建脚本
```

## 协议

CPL (Command Pipeline Language) v1.0.0

详细规范见 [docs/cpl-spec-v1.md](docs/cpl-spec-v1.md)
