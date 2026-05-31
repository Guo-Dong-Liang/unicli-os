#!/bin/bash
set -euo pipefail

# UniCLI OS - 一键安装脚本
# Usage: curl -fsSL https://get.unicli.dev | sh
# Or:    bash install.sh

REPO_URL="${UNICLI_REPO:-http://192.168.1.87:3000/admin/unicli-os}"
BIN_DIR="${UNICLI_BIN:-/usr/local/bin}"

echo "============================================"
echo "  UniCLI OS - 一键安装"
echo "============================================"
echo ""

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in x86_64|amd64) ARCH="amd64" ;; aarch64|arm64) ARCH="arm64" ;; *) echo "不支持的架构: $ARCH"; exit 1 ;; esac
case "$OS" in linux|darwin) ;; *) echo "不支持的系统: $OS (Windows 请用 WSL)"; exit 1 ;; esac

echo "  系统: $OS ($ARCH)"

if command -v go &>/dev/null; then
    echo "  Go: $(go version)"
    echo "  从源码构建..."
    TMP_DIR=$(mktemp -d)
    cd "$TMP_DIR"
    git clone --depth 1 "$REPO_URL" unicli-os 2>/dev/null || {
        echo "  克隆仓库失败，请检查 Gitea 服务器是否可访问"
        rm -rf "$TMP_DIR"; exit 1
    }
    cd unicli-os
    go build -o /tmp/unicli ./cmd/unicli
    cd / && rm -rf "$TMP_DIR"
    echo "  构建完成"
else
    echo "  未安装 Go，尝试下载预编译二进制..."
    BIN_URL="$REPO_URL/raw/main/bin/unicli-$OS-$ARCH"
    curl -sL -o /tmp/unicli "$BIN_URL" || { echo "  下载失败，请先安装 Go: https://go.dev/dl/"; exit 1; }
    chmod +x /tmp/unicli
fi

# 安装到 PATH
if [ -w "$BIN_DIR" ]; then
    mv /tmp/unicli "$BIN_DIR/unicli"
elif command -v sudo &>/dev/null; then
    sudo mv /tmp/unicli "$BIN_DIR/unicli"
else
    mkdir -p "$HOME/.local/bin"
    mv /tmp/unicli "$HOME/.local/bin/unicli"
    export PATH="$HOME/.local/bin:$PATH"
    echo "  添加到 PATH: export PATH=\"\$HOME/.local/bin:\$PATH\""
fi

echo ""
echo "============================================"
echo "  UniCLI 安装成功!"
echo "============================================"
echo ""
unicli --help 2>/dev/null || "$BIN_DIR/unicli" --help
echo ""
echo "  快速开始:"
echo "    unicli registry search         # 查看远程工具"
echo "    unicli run hello.say --name 果果  # 运行工具"
echo ""
