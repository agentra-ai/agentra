# Force PR Workflow

**Purpose:** Enforce that all agent code changes go through pull request review before merging.

## Instructions for Agents

### 1. Branch Naming Convention
All changes MUST be on a branch named: `agent/{issue-id}-{short-description}`

- Example: `agent/PROJ-42-fix-auth-bug`
- Example: `agent/PROJ-17-add-rate-limiter`
- Never commit directly to `main` or `master` branches.

### 2. PR Creation
Before marking a task as complete:
- Create a pull request from your working branch
- PR title format: `[{issue-id}] {task summary}`
- PR description must include:
  - **What**: Summary of changes made
  - **Why**: Reason for the change
  - **Test Plan**: How the changes were verified
  - **Checklist**: Verification steps (`make check` results)

### 3. CI Gate Requirements
ALL pull requests must pass before requesting review:
- `make check` (typecheck + tests + lint)
- All Go tests pass
- TypeScript typecheck clean

### 4. Review Required
- Request review from at least one team member
- Address all review comments
- Do NOT self-merge — a reviewer must approve and merge

### 5. Commit Standards
Follow conventional commit format:
- `feat(scope): description of new feature`
- `fix(scope): description of bug fix`
- `refactor(scope): description of refactoring`
- `docs(scope): description of documentation change`
- `test(scope): description of test addition`
- `chore(scope): description of maintenance task`

Use present tense, imperative mood (e.g., "add feature" not "added feature").

### 6. Anti-Patterns (Do NOT)
- Commit directly to `main` or protected branches
- Skip CI checks to "save time"
- Self-approve or self-merge PRs
- Leave PR descriptions empty
- Create PRs with untested code
- Merge with failing checks