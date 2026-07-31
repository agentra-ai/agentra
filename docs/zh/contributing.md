---
title: 贡献指南
description: 为 Agentra 开发做贡献
---

# 为 Agentra 做贡献

感谢你对贡献 Agentra 的兴趣！

## 开发环境配置

### 前置要求

- Node.js v20+
- pnpm v10.28+
- Go v1.26+
- Docker

### 快速开始

```bash
pnpm install
make setup  # 缺少 .env 时安全生成
make start
```

### 测试

```bash
# 所有检查
make check

# 单独运行
pnpm typecheck
pnpm test
make test
```

## Pull Request

1. Fork 仓库
2. 创建功能分支
3. 完成修改
4. 运行 `make check`
5. 提交 PR

## 代码规范

- Go: 标准 `gofmt`
- TypeScript: 启用严格模式
- 提交信息: conventional 格式

## 开源协议

贡献即表示你同意你的代码基于 Apache 2.0 开源协议。
