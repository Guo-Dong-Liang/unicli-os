# UniCLI OS — Universal Containerized Language Interface

> **Write once, run everywhere. A CLI platform for containerized tools.**  
> ⚡ Zero dependency | 🐳 Docker sandbox | 🔌 Extensible | 🌐 Remote registry

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)
[![CI](https://github.com/Guo-Dong-Liang/unicli-os/actions/workflows/ci.yml/badge.svg)](https://github.com/Guo-Dong-Liang/unicli-os/actions/workflows/ci.yml)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey)](https://github.com/Guo-Dong-Liang/unicli-os)

---

## What is UniCLI?

UniCLI OS is an open standard and runtime for **containerized CLI tools**. Each tool is described by a **CPL manifest** (Containerized Pipeline Language) — a JSON file that defines inputs, outputs, resource requirements, and how to run it inside a sandboxed container.

```
                    ┌──────────────────────────────────────┐
                    │          unicli run image.resize      │
                    │              │                        │
     ┌──────────────┴──────────────┴──────────────┐        │
     │         CPL Manifest (image.resize)        │        │
     │  ┌──────────┐  ┌──────────┐  ┌──────────┐  │        │
     │  │  Inputs  │→│  Engine  │→│ Outputs  │  │        │
     │  │ --width  │  │ Docker   │  │ FILE     │  │        │
     │  │ --format │  │ exec     │  │          │  │        │
     │  └──────────┘  └──────────┘  └──────────┘  │        │
     └────────────────────────────────────────────┘        │
                                                           │
     unicli run "tool1 | tool2"  ← pipeline chaining       │
     unicli run --image alpine  ← direct Docker mode       │
                    └──────────────────────────────────────┘
```

### Why UniCLI?

- **🌍 Cross-platform** — Same tool runs on Linux, macOS, Windows (Docker sandbox)
- **🔒 Secure by default** — Each tool runs in an isolated container (no network, read-only FS, non-root)
- **📦 Zero dependency** — Single binary, no runtime required beyond Docker
- **🔌 Extensible** — Write tools in any language (Python, Shell, Go, ...), publish to a registry
- **🪶 Pipeable** — Chain tools together: `unicli run "tool1 | tool2"`

---

## Quick Start

### Install

**Linux / macOS:**

```bash
curl -fsSL https://raw.githubusercontent.com/Guo-Dong-Liang/unicli-os/main/install.sh | sh
```

Or download a pre-built binary from [Releases](https://github.com/Guo-Dong-Liang/unicli-os/releases).

**Windows (PowerShell):**

```powershell
powershell -c "irm https://raw.githubusercontent.com/Guo-Dong-Liang/unicli-os/main/install.ps1 | iex"
```

### Run a Tool

```bash
unicli run hello.say --name 世界
```

### Create Your Own Tool

```bash
unicli init my-tool
# → Creates: my-tool/my-tool.cpl.json + my-tool/tool.sh

unicli registry install ./my-tool/
unicli run my-tool --name test
```

### Search Remote Registry

```bash
unicli registry search image
# → Found: image.resize (v1.0.0) - Resize an image using ImageMagick
```

---

## CLI Reference

| Command | Description |
|---------|-------------|
| `unicli run <tool> [args..]` | Run a tool from local registry |
| `unicli run --image <ref> [-- cmd..]` | Run a Docker image directly |
| `unicli init [name]` | Scaffold a new tool (interactive) |
| `unicli registry list` | List installed tools |
| `unicli registry install <dir>` | Install a tool from a local directory |
| `unicli registry inspect <name>` | View tool manifest details |
| `unicli registry remove <name>` | Remove an installed tool |
| `unicli registry search <query>` | Search the remote registry |
| `unicli registry login gitea <token>` | Authenticate for publishing |
| `unicli registry publish [dir]` | Publish a tool to remote registry |

### Pipe Mode

Tools automatically detect pipe mode (non-TTY stdout / piped stdin):

```bash
unicli run tool1 | unicli run tool2     # Chain outputs
unicli run tool1 -q | unicli run tool2  # Quiet mode
```

---

## CPL Manifest Format

A tool is defined by a simple JSON manifest. Example (`hello.say.cpl.json`):

```json
{
  "cpl_version": "1.0.0",
  "name": "hello.say",
  "version": "1.0.0",
  "description": "Say hello to someone",
  "inputs": [
    { "name": "name", "type": "STRING", "default": "World", "description": "Who to greet" }
  ],
  "outputs": [
    { "name": "greeting", "type": "TEXT", "capture_stdout": true }
  ],
  "resources": { "cpu": 0.1, "memory": 16 },
  "image": {
    "ref": "ghcr.io/Guo-Dong-Liang/hello.say:1.0.0",
    "entrypoint": "/app/say.sh"
  }
}
```

Full spec: [CPL v1.0 Specification](docs/cpl-spec-v1.md)

---

## Tools Available in the Registry

| Tool | Description | Language |
|------|-------------|----------|
| `hello.say` | Say hello to anyone | Shell |
| `image.resize` | Resize images using ImageMagick | Shell |
| `comfyui-gen` | Generate images via ComfyUI | Python |
| `llm` | Chat with local LLM (llama.cpp) | Python |
| `wechat` | WeChat integration demo | Python |

---

## Python SDK

Write tools in Python with a single decorator:

```python
from unicli import tool

@tool
def greet(name: str = "World", greeting: str = "Hello"):
    """Say hello to someone"""
    return f"{greeting}, {name}!"
```

Save as `greet.py`, then `unicli run greet --name 果果`.

See [Python SDK examples](sdk/python/examples/) for more.

---

## Project Structure

```
unicli-os/
├── cmd/
│   ├── unicli/              # Main CLI binary
│   │   ├── main.go          # CLI entrypoint + all commands
│   │   └── publish.go       # Gitea registry publish
│   └── unicli-validate/     # Validation CLI
├── pkg/
│   └── validator/           # Manifest + security + pipe validation
├── sdk/
│   └── python/              # Python @tool decorator SDK
├── registry/
│   ├── index.json           # Remote registry index
│   └── tools/               # Published tool manifests
├── protos/
│   └── cpl.proto            # CPL protobuf definitions
├── schemas/
│   └── cpl-manifest.schema.json
├── docs/
│   └── cpl-spec-v1.md       # Full CPL v1.0 specification
├── examples/                # Example tools
├── tools/                   # Development tools
├── install.sh               # Linux/macOS installer
├── install.ps1              # Windows installer
└── Makefile                 # Build system
```

---

## Security

All tools run in a **sandboxed Docker container** with:
- `--network none` — No network access
- `--read-only` — Read-only root filesystem
- `--cap-drop=ALL` — All Linux capabilities dropped
- `--user nobody:nogroup` — Non-root execution

See `pkg/validator/security.go` for the test suite that verifies these guarantees.

---

## Roadmap

- [x] **CPL v1.0 Spec** — Protocol definition + JSON Schema + Protobuf
- [x] **Local Run** — Read manifest, resolve entrypoint, execute
- [x] **Tool Scaffolding** — Interactive `unicli init`
- [x] **Local Registry** — List, install, inspect, remove tools
- [x] **Remote Registry** — Search, auto-install from remote index
- [x] **Validation** — Manifest semantic checks + security tests
- [x] **Python SDK** — @tool decorator with auto-manifest generation
- [x] **Cross-Platform Build** — Linux/macOS/Windows binaries
- [x] **CI Pipeline** — GitHub Actions (lint, build, test)
- [ ] **Docker Sandbox Runner** — Orchestrated container execution
- [ ] **Pipe Chaining** — `unicli run "A | B | C"` pipeline syntax
- [ ] **Protobuf Codegen** — Generate Go code from cpl.proto
- [ ] **More SDKs** — Go, Rust, TypeScript bindings
- [ ] **Plugin System** — Custom engines beyond Docker
- [ ] **Web UI** — Browse + run tools from a browser

---

## Development

```bash
make build          # Build both binaries
make test           # Run tests
make lint           # Run golangci-lint
make validate       # Validate all example manifests
make proto          # Generate protobuf Go code
make clean          # Clean build artifacts
```

Requires Go 1.22+ and optionally Docker for sandbox tests.

---

## License

[MIT](LICENSE)

---

> Built with ❤️ by the UniCLI Team
