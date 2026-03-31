---
allowed-tools: Bash(git *), Read, Edit, Write
description: Stage changes, update progress log, and commit (end-of-session)
---

# Session Commit

Automate end-of-session cleanup: stage, log progress, commit.

## Steps

1. **Assess changes** — run these in parallel:
   - `git status`
   - `git diff HEAD`
   - `git branch --show-current`
   - `git log --oneline -10`

2. **Stage everything** — if there are unstaged or untracked changes:
   - Use `git add <specific-files>` (not `git add .`)
   - Never stage files that likely contain secrets (.env, credentials, etc.)
   - Skip if everything is already staged

3. **Update progress log** — read `docs/claude-progress.txt`, then prepend a new entry at the top (after the header line) following the format in `docs/references/claude-progress-readme.md`.
   - Analyze the staged diff to determine what was done
   - Use today's actual date
   - Be concise but complete
   - Stage the updated file: `git add docs/claude-progress.txt`

4. **Migrate blockers** — if the progress entry you just wrote has a `Blockers:` line that is NOT `None`:
   - Read `docs/agent-struggles.json`
   - For each blocker, append an entry:

     ```json
     {
       "date": "<today>",
       "description": "<blocker text from progress entry>",
       "cause": "<your best short assessment>",
       "suggestion": "<what would fix or prevent this>",
       "resolved": false
     }
     ```

   - Stage: `git add docs/agent-struggles.json`

5. **Draft and commit** — follow all rules from `docs/references/commit-message-guide.md`. Determine authorship mode (tandem vs autonomous) from the guide. Use a HEREDOC:

   ```bash
   git commit -m "$(cat <<'EOF'
   <type>: <subject>

   <body>

   <authorship trailer>
   EOF
   )"
   ```

6. **Verify** — run `git status` and `git log --oneline -1` to confirm success.

## Rules

- Analyze the FULL diff for accurate progress and commit message
- Never commit secrets or sensitive files
- If commit fails due to pre-commit hook, fix and create a NEW commit (never amend)
- Do NOT push to remote
