# Git-Native Hooks 设计规格

**日期**: 2026-05-10
**基于**: agent-tasks 深度分析 (hook 源码研读)
**状态**: Draft

---

## 1. 概述

为 Agentra 添加 git-native hooks，自动将 commits/PRs 关联到 issues，并在 PR merge 时自动完成 issue。

### 1.1 关键设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| Hook 语言 | Node.js (零依赖) | 跟随 agent-tasks 模式: `#!/usr/bin/env node`，只用 `fs`/`path`/`os`/`child_process` |
| Task ID 来源 | Branch name 优先 → commit message → PR metadata | agent-tasks 三层策略 |
| API 协议 | REST (非 MCP) | Hook 在 git 上下文运行，REST 总是可用 |
| 错误处理 | Hook 永不阻断 git (always exit 0) | agent-tasks 模式: 全部 wrap try/catch |
| 安装方式 | `agentra git hooks install` (Go embed.FS) | 匹配 Agentra 的 Go-native 架构 |

---

## 2. Hook 脚本设计

### 2.1 prepare-commit-msg

```javascript
#!/usr/bin/env node
// 自动 prepend [AGENTRA-123] 到 commit message

const { execSync } = require('child_process');

try {
  const branch = execSync('git branch --show-current', { encoding: 'utf8' }).trim();
  const match = branch.match(/(AGENTRA-\d+)/i);
  if (!match) process.exit(0);

  const taskId = match[1].toUpperCase();
  const msgFile = process.argv[2];
  const commitSource = process.argv[3]; // merge|squash|commit|template

  if (['merge', 'squash'].includes(commitSource)) process.exit(0);

  const fs = require('fs');
  let msg = fs.readFileSync(msgFile, 'utf8');
  if (msg.startsWith(`[${taskId}]`)) process.exit(0);

  fs.writeFileSync(msgFile, `[${taskId}] ${msg}`);
} catch (e) {
  process.exit(0); // never block
}
```

### 2.2 post-commit

```javascript
#!/usr/bin/env node
// Link commit SHA to Agentra issue

const { execSync } = require('child_process');

try {
  const branch = execSync('git branch --show-current', { encoding: 'utf8' }).trim();
  const match = branch.match(/(AGENTRA-\d+)/i);
  if (!match) process.exit(0);

  const taskId = match[1].toUpperCase();
  const [sha, message] = execSync('git log -1 --format="%H %s" HEAD', { encoding: 'utf8' }).trim().split(/ (.+)/);

  const apiUrl = process.env.AGENTRA_API_URL || 'http://localhost:8080';
  const token = process.env.AGENTRA_API_TOKEN;

  fetch(`${apiUrl}/api/git/link-commit`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...(token && { Authorization: `Bearer ${token}` }) },
    body: JSON.stringify({ issueId: taskId, sha, message: message || '', branch })
  }).catch(() => {});
} catch (e) {
  process.exit(0);
}
```

### 2.3 post-merge

```javascript
#!/usr/bin/env node
// Auto-transition issue to Done on PR merge

const { execSync } = require('child_process');

try {
  const mergeMsg = execSync('git log -1 --format="%s%n%b" HEAD', { encoding: 'utf8' });
  const prMatch = mergeMsg.match(/#(\d+)/);
  if (!prMatch) process.exit(0);

  const prNumber = prMatch[1];
  let prData;
  try {
    prData = JSON.parse(execSync(`gh pr view ${prNumber} --json number,url,state,title,body,mergedAt,headRefName`, { encoding: 'utf8' }));
  } catch (e) {
    process.exit(0);
  }

  const allText = [prData.title, prData.body, prData.headRefName].join(' ');
  const issueMatch = allText.match(/(AGENTRA-\d+)/i);
  if (!issueMatch) process.exit(0);

  const apiUrl = process.env.AGENTRA_API_URL || 'http://localhost:8080';
  const token = process.env.AGENTRA_API_TOKEN;

  fetch(`${apiUrl}/api/git/link-pr`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...(token && { Authorization: `Bearer ${token}` }) },
    body: JSON.stringify({
      issueId: issueMatch[1].toUpperCase(),
      prNumber, prUrl: prData.url,
      prState: prData.state.toLowerCase(),
      prTitle: prData.title,
      mergedAt: prData.mergedAt || null
    })
  }).catch(() => {});
} catch (e) {
  process.exit(0);
}
```

### 2.4 Claude Code PreToolUse Gate

```javascript
#!/usr/bin/env node
// agent-tasks pattern: block code edits unless active in_progress issue

const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');
const os = require('os');

try {
  if (process.env.SKIP_TASK_GATE === '1') process.exit(0);

  let input;
  try { input = JSON.parse(fs.readFileSync(process.stdin.fd || 0, 'utf8')); }
  catch { process.exit(0); }

  const filePath = input?.tool_input?.file_path;
  if (!filePath) process.exit(0);

  const codeExts = ['.ts','.tsx','.js','.jsx','.py','.go','.rs','.java','.c','.cpp','.h','.sql'];
  const ext = path.extname(filePath);
  if (!codeExts.includes(ext)) process.exit(0);
  if (filePath.includes('scratchpad') || filePath.includes('tmp')) process.exit(0);

  const branch = execSync('git branch --show-current', { encoding: 'utf8' }).trim();
  const match = branch.match(/(AGENTRA-\d+)/i);
  if (!match) {
    console.error('[Agentra] No active issue on branch. Create/claim an issue before editing code.');
    process.exit(2);
  }

  const apiUrl = process.env.AGENTRA_API_URL || 'http://localhost:8080';
  const token = process.env.AGENTRA_API_TOKEN;
  const headers = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch(`${apiUrl}/api/git/active-task?branch=${encodeURIComponent(branch)}`, { headers });
  const data = await res.json();

  if (data?.active && data?.status === 'in_progress') {
    process.exit(0); // allow
  }

  const flagFile = path.join(os.tmpdir(), `.agentra-gate-${process.ppid || 0}`);
  if (!fs.existsSync(flagFile)) {
    fs.writeFileSync(flagFile, '');
    console.error('[Agentra] No in-progress issue found. Start an issue before editing code.');
    console.error('[Agentra] Set SKIP_TASK_GATE=1 to bypass.');
  }
  process.exit(2);
} catch (e) {
  process.exit(0); // permissive on failure
}
```

---

## 3. API Endpoints

| Method | Path | Body | 描述 |
|--------|------|------|------|
| POST | /api/git/link-commit | { issueId, sha, message, branch } | Link commit to issue (idempotent by SHA) |
| POST | /api/git/link-pr | { issueId, prNumber, prUrl, prState, mergedAt } | Link PR + auto-transition to Done if merged |
| POST | /api/git/link-branch | { issueId, branch } | Link branch to issue |
| GET | /api/git/active-task | ?branch=X | Returns active in_progress issue for branch |
| GET | /api/git/issue-links | ?issueId=X | Returns all git links for issue |

---

## 4. 数据库

扩展已有 `github_issue_links` → `issue_git_links`:

```sql
ALTER TABLE github_issue_links RENAME TO issue_git_links;
ALTER TABLE issue_git_links ADD COLUMN link_type TEXT NOT NULL DEFAULT 'pr';
  -- 'commit', 'pr', 'branch'
ALTER TABLE issue_git_links ADD COLUMN sha TEXT;
ALTER TABLE issue_git_links ADD COLUMN message TEXT;
ALTER TABLE issue_git_links ADD COLUMN authored_at TIMESTAMPTZ;
ALTER TABLE issue_git_links ADD COLUMN pr_state TEXT;
ALTER TABLE issue_git_links ADD COLUMN merged_at TIMESTAMPTZ;
ALTER TABLE issue_git_links ADD COLUMN pr_title TEXT;
ALTER TABLE issue_git_links ADD COLUMN branch TEXT;
```

---

## 5. 安装

### `agentra git hooks install` CLI 命令

使用 Go `embed.FS` 嵌入 4 个 hook 脚本:

```go
//go:embed hooks/*
var hookScripts embed.FS

func installHooks(targetDir string) error {
    scripts := []string{"prepare-commit-msg", "post-commit", "post-merge"}
    for _, name := range scripts {
        data, _ := hookScripts.ReadFile("hooks/" + name)
        os.WriteFile(filepath.Join(targetDir, ".git", "hooks", name), data, 0755)
    }
    // Claude Code hook
    os.MkdirAll(filepath.Join(targetDir, ".claude", "hooks"), 0755)
    data, _ := hookScripts.ReadFile("hooks/task-gate.js")
    os.WriteFile(filepath.Join(targetDir, ".claude", "hooks", "task-gate.js"), data, 0644)
    return nil
}
```

---

## 6. 实现优先级

1. API endpoints (link-commit, link-pr, link-branch, active-task)
2. DB migration (rename + extend issue_git_links)
3. Hook scripts (4 files)
4. CLI install command (`agentra git hooks install`)
5. PreToolUse gate validation

---

## 7. 参考资料

- [agent-tasks hook 源码](https://github.com/nash-software/mcp-agent-tasks)
- [Overseer VCS workflow](https://github.com/dmmulroy/overseer)
- [Tasuku per-file locking](https://github.com/iheanyi/tasuku)
- [竞争分析 v2](2026-05-10-competitive-analysis-v2.md)
