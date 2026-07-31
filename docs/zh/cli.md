---
title: CLI 参考
description: Agentra CLI 命令参考
---

# CLI 参考

`agentra` CLI 用于将你的机器连接到 Agentra。

## 安装

```bash
# macOS
brew install --cask agentra-ai/tap/agentra

# macOS 或 Linux（先下载以便审阅）
curl -fsSLO https://raw.githubusercontent.com/agentra-ai/agentra/main/scripts/install.sh
sh install.sh
rm install.sh
```

```powershell
# Windows PowerShell
Invoke-WebRequest https://raw.githubusercontent.com/agentra-ai/agentra/main/scripts/install.ps1 -OutFile install.ps1
powershell -ExecutionPolicy Bypass -File .\install.ps1
Remove-Item .\install.ps1
```

脚本会使用 `checksums.txt` 校验 release 归档，不会自动提权，并在完成前执行 `agentra version`。Tag release 还会提供 checksum 的 Cosign keyless bundle、带签名的 SPDX SBOM 与 GitHub provenance；请按[发布验证指南](DEPLOYMENT.md)固定并验证预期的 workflow 身份。

## 命令

### 引导式配置

```bash
agentra setup --deployment self-host
agentra setup --deployment cloud --server-url https://api.example.com --app-url https://app.example.com
```

`setup` 会检查 API readiness 与 Web 应用，确认本机存在 `claude`、`codex` 或 `opencode`，然后完成认证、workspace 发现和 daemon 启动。self-host 默认使用 Compose 发布的 loopback 端口；在公开托管地址发布前，cloud 必须显式指定 HTTPS 端点。PAT 登录使用 `--token`，仅管理机器使用 `--no-daemon`，多个部署使用 `--profile` 隔离。

### 认证

```bash
agentra login          # 与服务器认证
agentra auth logout    # 清除凭证
agentra auth status    # 检查认证状态
```

### 守护进程

```bash
agentra daemon start   # 启动 Agent 守护进程
agentra daemon stop    # 停止守护进程
agentra daemon status  # 查看守护进程状态
```

### 工作空间

```bash
agentra workspace list     # 列出工作空间
agentra workspace watch    # 让本地 daemon 监听工作空间
agentra workspace unwatch  # 停止监听工作空间
```

### Agent

```bash
agentra agent list        # 列出 Agent
agentra agent create      # 创建新 Agent
agentra agent archive     # 归档 Agent
```

### 安装诊断

启动 daemon 前或安装出现异常时运行：

```bash
agentra doctor
agentra doctor --repo /path/to/repository
agentra doctor --output json
agentra doctor --skip-repo-remote   # 离线/仅本地仓库检查
```

`doctor` 会检查当前 profile 的配置文件权限、Web 与 API 可达性、服务 readiness、对象存储、认证、workspace 权限、受支持的 Runtime CLI、daemon 工作目录权限、Git origin 访问、本地 daemon，以及带认证的 WebSocket ping/pong。各检查并发执行，外部操作默认各有 5 秒超时，可通过 `--timeout` 调整。

必需检查失败时命令以非零状态退出；对象存储按需关闭、本地 daemon 尚未启动或当前目录不是 Git 仓库等警告不会导致失败。JSON 输出使用版本化的 `schema_version: 1` 契约且不会包含 access token。WebSocket 诊断通过 `Authorization` Header 发送 PAT，不会把 PAT 写入 URL。

## 配置

默认 profile 存储在 `~/.agentra/config.json`，命名 profile 使用 `~/.agentra/profiles/<name>/config.json`。包含 access token 的配置文件应仅允许所有者访问（macOS/Linux 为 `0600`）。

## 另请参阅

- [GitHub 仓库](https://github.com/agentra-ai/agentra)
