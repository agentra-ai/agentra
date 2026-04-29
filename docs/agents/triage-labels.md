# Triage Labels

The triage state machine uses five labels. Default vocabulary:

| Label | Purpose |
|---|---|
| `needs-triage` | Maintainer needs to evaluate the issue |
| `needs-info` | Waiting on reporter for more information |
| `ready-for-agent` | Fully specified, ready for an agent to pick up |
| `ready-for-human` | Needs human implementation |
| `wontfix` | Will not be actioned |

## Consumer Rules

- `triage` skill applies these labels as it moves issues through the state machine.
- Do not create duplicate labels — use the names above exactly.
- If relabeling from one triage state to another, remove the old label first.
