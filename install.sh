#!/bin/bash
set -euo pipefail

# UniCLI OS - 一键安装脚本
# 使用方法:
#   curl -fsSL https://raw.githubusercontent.com/Guo-Dong-Liang/unicli-os/main/install.sh | sh
#   或从 Releases 下载: https://github.com/Guo-Dong-Liang/unicli-os/releases

REPO_URL="${UNICLI_REPO:-https://github.com/Guo-Dong-Liang/unicli-os}"
RELEASE_VERSION="${UNICLI_VERSION:-v1.0}"
BIN_DIR="${UNICLI_BIN:-/usr/local/bin}"

echo "============================================"
echo "  UniCLI OS - 一键安装"
echo "============================================"
echo ""

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "  不支持的架构: $ARCH"; exit 1 ;;
esac
case "$OS" in linux|darwin) ;; *) echo "  不支持的系统: $OS (Windows 请用 install.ps1 或 WSL)"; exit 1 ;; esac

echo "  系统: $OS ($ARCH)"
echo ""

# 策略1：从 GitHub Release 下载预编译二进制
BINARY="unicli-${OS}-${ARCH}"
RELEASE_URL="https://github.com/${REPO_URL#https://github.com/}/releases/download/${RELEASE_VERSION}/${BINARY}"

echo "  ▶ 尝试从 Release 下载..."
if curl -sL --connect-timeout 15 --max-time 60 -o /tmp/unicli "$RELEASE_URL" 2>/dev/null && [ -s /tmp/unicli ]; then
  echo "  ✓ 下载成功"
  chmod +x /tmp/unicli
else
  echo "  ⚠ Release 下载失败（可能被墙），尝试 ghproxy 代理..."
  PROXY_URL="https://ghproxy.net/${RELEASE_URL}"
  if curl -sL --connect-timeout 15 --max-time 90 -o /tmp/unicli "$PROXY_URL" 2>/dev/null && [ -s /tmp/unicli ]; then
    echo "  ✓ 通过代理下载成功"
    chmod +x /tmp/unicli
  else
    echo "  ⚠ 代理下载也失败，尝试从源码编译..."
    if command -v go &>/dev/null; then
      echo "  Go: $(go version)"
      echo "  从源码构建..."
      TMP_DIR=$(mktemp -d)
      cd "$TMP_DIR"
      
      # 尝试直接 git clone，失败则用 ghproxy
      echo "  克隆仓库..."
      if ! git clone --depth 1 "$REPO_URL" unicli-os 2>/dev/null; then
        echo "  直接克隆失败，尝试代理克隆..."
        PROXY_CLONE_URL="https://ghproxy.net/${REPO_URL}"
        git clone --depth 1 "$PROXY_CLONE_URL" unicli-os 2>/dev/null || {
          echo "  ✗ 克隆仓库失败，请检查网络连接"
          echo "  你可以手动从 Releases 下载二进制:"
          echo "    https://github.com/Guo-Dong-Liang/unicli-os/releases"
          rm -rf "$TMP_DIR"
          exit 1
        }
      fi
      
      cd unicli-os
      go build -o /tmp/unicli ./cmd/unicli
      cd / && rm -rf "$TMP_DIR"
      echo "  ✓ 构建完成"
    else
      echo "  ✗ 未安装 Go，也无法下载预编译二进制"
      echo "  请先安装 Go: https://go.dev/dl/"
      echo "  或从 Releases 手动下载:"
      echo "    https://github.com/Guo-Dong-Liang/unicli-os/releases"
      exit 1
    fi
  fi
fi

# 安装到 PATH
INSTALLED=false
if [ -w "$BIN_DIR" ]; then
  mv /tmp/unicli "$BIN_DIR/unicli"
  INSTALLED=true
elif command -v sudo &>/dev/null; then
  sudo mv /tmp/unicli "$BIN_DIR/unicli" && INSTALLED=true
fi

if [ "$INSTALLED" = false ]; then
  mkdir -p "$HOME/.local/bin"
  mv /tmp/unicli "$HOME/.local/bin/unicli"
  echo "  已安装到: $HOME/.local/bin/unicli"
  echo "  请将 $HOME/.local/bin 添加到 PATH:"
  echo "    echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.zshrc"
  echo "    source ~/.zshrc"
else
  echo "  已安装到: $BIN_DIR/unicli"
fi

echo ""
echo "============================================"
echo "  UniCLI 安装成功!"
echo "============================================"
echo ""
unicli --help 2>/dev/null || "$BIN_DIR/unicli" --help || true
echo ""
echo "  快速开始:"
echo "    unicli run hello.say --name 果果"
echo ""
echo "  更多帮助:"
echo "    unicli --help"
echo ""
