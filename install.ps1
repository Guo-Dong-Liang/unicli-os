# UniCLI OS - Windows 一键安装脚本 (PowerShell)
# 使用方法: powershell -c "irm https://get.unicli.dev/install.ps1 | iex"

$REPO_URL = $env:UNICLI_REPO ? $env:UNICLI_REPO : "https://github.com/unixcli/unicli-os"
$INSTALL_DIR = "$env:USERPROFILE\.unicli\bin"

Write-Host "============================================"
Write-Host "  UniCLI OS - Windows 一键安装"
Write-Host "============================================"
Write-Host ""

# 创建安装目录
New-Item -ItemType Directory -Force -Path $INSTALL_DIR | Out-Null

# 下载预编译二进制
$BIN_URL = "$REPO_URL/raw/main/bin/unicli-windows-amd64.exe"
$OUTPUT = "$INSTALL_DIR\unicli.exe"

Write-Host "  下载: $BIN_URL"
try {
    Invoke-WebRequest -Uri $BIN_URL -OutFile $OUTPUT -TimeoutSec 30
} catch {
    Write-Host "  下载失败，尝试从源码构建..."
    # 检查 Go
    $goVer = go version 2>$null
    if (-not $goVer) {
        Write-Host "  请先安装 Go: https://go.dev/dl/"
        exit 1
    }
    $TMP_DIR = "$env:TEMP\unicli-install"
    New-Item -ItemType Directory -Force -Path $TMP_DIR | Out-Null
    git clone --depth 1 $REPO_URL "$TMP_DIR\unicli-os"
    cd "$TMP_DIR\unicli-os"
    go build -o $OUTPUT ./cmd/unicli
    Remove-Item -Recurse -Force $TMP_DIR
}

# 添加到 PATH
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
Write-Host "    unicli registry search"
Write-Host "    unicli run hello.say --name 果果"
Write-Host ""
