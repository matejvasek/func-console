---
allowed-tools: Bash(git log:*), Read
description: Load recent context and wait for user direction
---

# Session Onboard

Load recent context that isn't in AGENTS.md (which is always in context).

## Steps

1. **Read recent progress** — read `docs/claude-progress.txt` (only the last 3 days of entries matter).

2. **Read recent commits** — run:
   ```bash
   git log --oneline --since="3 days ago"
   ```

3. **Wait** — tell the user you're oriented and ask what they'd like to work on. Do NOT start any work autonomously.
