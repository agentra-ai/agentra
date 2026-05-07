---
title: CLI 参考
description: Agenttra CLI 命令参考
---

# CLI 参考

`agentra` CLI 用于将你的机器连接到 Agenttra。

## 安装

```bash
# 从源码构建
make build
cp server/bin/agentra /usr/local/bin/
```

## 命令

### 认证

```bash
agentra login          # 与服务器认证
agentra logout         # 清除凭证
agentra status         # 检查连接状态
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
agentra workspace switch   # 切换工作空间
```

### Agent

```bash
agentra agent list        # 列出 Agent
agentra agent create      # 创建新 Agent
agentra agent delete      # 删除 Agent
```

## 配置

配置存储在 `~/.agentra/`。

## 另请参阅

- [GitHub 仓库](https://github.com/agentra-ai/agentra)
