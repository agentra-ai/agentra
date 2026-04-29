# Issue Tracker

Issues for this repository are tracked in **GitHub Issues**.

## Consumer Rules

- Use `gh issue create` to file new issues.
- Use `gh issue list --label=<label>` to filter by triage label.
- Use `gh issue edit <number> --add-label=<label>` to update triage state.
- Close issues with `gh issue close <number>`.

## Relevant Skills

- `to-issues` — convert comments, messages, or specs into filed GitHub issues
- `triage` — process incoming issues through the triage state machine
- `to-prd` — expand an issue into a full product requirements document
- `qa` — verify issue fix via testing

## Workflow

1. **File issue** → `gh issue create --title="..." --body="..." --label=triage`
2. **Triage** → apply `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, or `wontfix`
3. **Work** → issue is resolved and closed
