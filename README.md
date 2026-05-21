# 🚀 UniCLI OS

> **Unity CLI, Beyond Apps.**
>
> 下一代操作系统 — 一切皆 CLI，按需生成，用完即焚。

## 核心理念

APP 即 CLI + 容器镜像。用户不再安装应用，而是：

- **按需运行**：`unicli run image.resize --input photo.jpg --width 800`
- **链式调用**：`unicli run "image.resize | image.grayscale" < input.jpg`
- **用完即焚**：沙箱退出自动清理，不留痕迹
- **安全隔离**：容器无网络、只读文件系统、非 root 用户
- **可保存为镜像**：`unicli save my-tool:1.0`

## 快速开始

```bash
# 构建
make build

# 注册一个 CLI 命令
unicli registry install examples/image.resize.cpl.json

# 运行
unicli run hello.say --name "World"

# 链式管道
unicli run "hello.say --name World | hello.say --greeting Hi"

# 查看已安装的命令
unicli registry list

# 查看命令详情
unicli inspect hello.say
```

## 项目结构

```
unicli-os/
├── cmd/unicli/             # CLI 入口
├── pkg/
│   ├── cpl/                # CPL 协议解析
│   ├── runner/             # 沙箱运行器（Docker）
│   └── registry/           # 本地 CLI 注册表
├── protos/
│   └── cpl.proto           # CPL Protocol Buffers 定义
├── schemas/
│   └── cpl-manifest.schema.json  # Manifest JSON Schema
├── examples/
│   ├── image.resize.cpl.json  # 图片缩放 CLI 示例
│   └── hello.say.cpl.json     # 测试 CLI 示例
├── docs/
│   └── cpl-spec-v1.md     # CPL 协议规范文档
├── scripts/               # 构建和部署脚本
├── Makefile               # 构建系统
└── Dockerfile             # 基础运行时镜像
```

## 架构

```
┌──────────────┐     ┌──────────────────┐     ┌──────────────┐
│ Thin Client  │────▶│  AI 编译器        │────▶│  CLI Registry│
│ (终端/语音)   │     │ (NL→CPL Manifest)│     │ (.cpl.json)  │
└──────────────┘     └──────────────────┘     └──────┬───────┘
                                                      │
┌──────────────┐     ┌──────────────────┐            │
│ 用户数据保险库 │◀────│  调度器           │◀───────────┘
│ (加密存储)    │     │ (Sandbox Runner)  │
└──────────────┘     └────────┬─────────┘
                               │
                    ┌──────────▼──────────┐
                    │  Docker 沙箱容器池    │
                    │ (gVisor Phase 2)    │
                    └─────────────────────┘
```

## License

MIT
