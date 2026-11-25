# 构建单个 Linux 静态二进制（前端 + 后端 + 迁移全 embed）。
# 用法：powershell -ExecutionPolicy Bypass -File build.ps1  → 产出 dist\good-habit-linux-amd64
$ErrorActionPreference = "Stop"
Set-Location "$PSScriptRoot"

# ---- 挑 Go ≥1.23 工具链：优先 PATH，其次 ~/sdk 里最新的 ----
$goBin = (Get-Command go -ErrorAction SilentlyContinue).Source
$sdk = Get-ChildItem "$HOME\sdk\go1.*\bin\go.exe" -ErrorAction SilentlyContinue |
  Sort-Object FullName | Select-Object -Last 1
if ($sdk) { $goBin = $sdk.FullName }
if (-not $goBin) {
  Write-Error "找不到 go，需要 Go >= 1.23"
}
Write-Host "使用 $(& $goBin version)"

# ---- 前端 ----
Push-Location "$PSScriptRoot\web"
npm ci
npm run build
Pop-Location

# ---- 后端（纯 Go 无 CGO，交叉编译即静态） ----
New-Item -ItemType Directory -Force -Path "$PSScriptRoot\dist" | Out-Null
$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "amd64"
& $goBin build -trimpath -ldflags='-s -w' -o "dist\good-habit-linux-amd64" ./cmd/good-habit

Write-Host "构建完成: dist\good-habit-linux-amd64"
