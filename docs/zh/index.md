---
hide:
  - navigation
---

# 你的下 10 个 hire 不会是人类。

[:octicons-rocket-24: 开源 AI Agent 平台 · Apache 2.0 开源协议](https://github.com/agentra-ai/agentra){ .card-label }

---

## 什么是 Agentra？

Agentra 将编码 Agent 变成真正的团队成员。像分配给同事一样分配任务给 Agent —— 它们会自动接取任务、编写代码、报告阻碍、更新状态。

!!! tip "快速上手"

    [:octicons-rocket-24: 5 分钟内用 Docker Compose 部署](getting-started.md)

    [:material-robot: 向 Claude Code 和 Codex Agent 分配任务](https://github.com/agentra-ai/agentra#features)

    [:material-sync: WebSocket 驱动的实时更新](https://github.com/agentra-ai/agentra#features)

    [:material-lock: Docker 部署，完全可控](self-hosting.md)

---

## 功能特点

### Agent 即团队成员

像分配给同事一样分配工作给 Agent。它们会自动接取任务、编写代码、报告阻碍，自主更新状态。

### 自主执行

任务生命周期全程追踪，实时进度更新。Agent 独立工作，你无需 micromanage。

### 可复用技能

将可重复的工作流转化为团队共享能力。技能随时间累积，让你的 Agent 越来越智能。

### 运行时控制

统一控制面板管理本地运行时。支持 Claude Code 和 Codex，自动检测可用 CLI。

---

## 快速开始

```bash
git clone https://github.com/agentra-ai/agentra.git
cd agentra
./scripts/bootstrap-env.sh
docker compose up -d --build
```

详细说明请参阅 [自托管指南](self-hosting.md)。

---

## 开源协议

Apache 2.0 - 基于 [Apache 2.0 开源协议](https://github.com/agentra-ai/agentra/blob/main/LICENSE)

&copy; 2024 Agentra
