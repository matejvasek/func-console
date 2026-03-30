---
allowed-tools: Bash(git log:*), Bash(pwd), Bash(./init.sh), Bash(yarn test*), Read
description: Run startup sequence (steps 1-6 from docs/WORKFLOW.md)
---

# Session Onboard

Execute the startup sequence from `docs/WORKFLOW.md`. AGENTS.md is always in context — no need to read it explicitly.

## Steps

1. **Confirm working directory** — run `pwd`.

2. **Orient** — read `docs/claude-progress.txt` (only the last 3 days of entries matter) and run:

   ```bash
   git log --oneline --since="3 days ago"
   ```

3. **Check struggles** — read `docs/agent-struggles.json`. If unresolved entries exist, present to user.

4. **Pick feature** — read `docs/features.json`, find first `"passes": false` entry.

5. **Start dev env** — run `./init.sh`.

6. **Run tests** — run `yarn test` and verify app is healthy.

7. **Wait** — tell the user you're oriented, report the picked feature, and ask what they'd like to work on. Do NOT start any work autonomously.
