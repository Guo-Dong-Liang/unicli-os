# UniCLI OS - Windows 一键安装脚本 (PowerShell 5.1+)
# 使用方法:
#   powershell -c "irm https://raw.githubusercontent.com/Guo-Dong-Liang/unicli-os/main/install.ps1 | iex"
#   或从 Releases 下载: https://github.com/Guo-Dong-Liang/unicli-os/releases

# 兼容 PowerShell 5.1 (不用 ?: 三目运算符)
if ($env:UNICLI_REPO) { $REPO_URL = $env:UNICLI_REPO } else { $REPO_URL = "https://github.com/Guo-Dong-Liang/unicli-os" }
if ($env:UNICLI_VERSION) { $RELEASE_VERSION = $env:UNICLI_VERSION } else { $RELEASE_VERSION = "v1.0" }
$INSTALL_DIR = "$env:USERPROFILE\.unicli\bin"

Write-Host "============================================"
Write-Host "  UniCLI OS - Windows 一键安装"
Write-Host "============================================"
Write-Host ""

# 创建安装目录
New-Item -ItemType Directory -Force -Path $INSTALL_DIR | Out-Null

# 从 GitHub Release 下载预编译二进制
$BINARY = "unicli-windows-amd64.exe"
$RELEASE_URL = "https://github.com/Guo-Dong-Liang/unicli-os/releases/download/$RELEASE_VERSION/$BINARY"
$OUTPUT = "$INSTALL_DIR\unicli.exe"

Write-Host "  ▶ 从 Release 下载..."
try {
    Invoke-WebRequest -Uri $RELEASE_URL -OutFile $OUTPUT -TimeoutSec 60
    Write-Host "  ✓ 下载成功"
} catch {
    Write-Host "  ⚠ Release下载失败，尝试 ghproxy 代理..."
    $PROXY_URL = "https://ghproxy.net/$RELEASE_URL"
    try {
        Invoke-WebRequest -Uri $PROXY_URL -OutFile $OUTPUT -TimeoutSec 90
        Write-Host "  ✓ 通过代理下载成功"
    } catch {
        Write-Host "  ⚠ 代理下载也失败，尝试从源码构建..."
        $goVer = go version 2>$null
        if (-not $goVer) {
            Write-Host "  ✗ 请先安装 Go: https://go.dev/dl/"
            Write-Host "  或从 Releases 手动下载:"
            Write-Host "    https://github.com/Guo-Dong-Liang/unicli-os/releases"
            exit 1
        }
        $TMP_DIR = "$env:TEMP\unicli-install"
        New-Item -ItemType Directory -Force -Path $TMP_DIR | Out-Null
        Write-Host "  克隆仓库..."
        git clone --depth 1 $REPO_URL "$TMP_DIR\unicli-os"
        cd "$TMP_DIR\unicli-os"
        go build -o $OUTPUT ./cmd/unicli
        Remove-Item -Recurse -Force $TMP_DIR
        Write-Host "  ✓ 构建完成"
    }
}

# 添加到用户 PATH
$currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($currentPath -notlike "*$INSTALL_DIR*") {
    [Environment]::SetEnvironmentVariable("Path", "$currentPath;$INSTALL_DIR", "User")
    $env:Path += ";$INSTALL_DIR"
}

Write-Host ""
Write-Host "============================================"
Write-Host "  UniCLI 安装成功!"
Write-Host "============================================"
Write-Host ""
Write-Host "  已安装到: $INSTALL_DIR"
Write-Host "  已添加到用户 PATH"
Write-Host ""
Write-Host "  快速开始:"
Write-Host "    unicli run hello.say --name 果果"
Write-Host ""
