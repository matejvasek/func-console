---
allowed-tools: Bash(git add:*), Bash(git status:*), Bash(git log:*), Bash(git diff:*), Bash(git branch:*)
description: Draft a git commit message (interactive — does NOT commit)
---

# Commit (Interactive)

Draft a commit message following `docs/references/commit-message-guide.md`. Do NOT commit — show the message and the terminal command only.

## Steps

1. Run in parallel:
   - `git status`
   - `git diff HEAD`
   - `git branch --show-current`
   - `git log --oneline -10` (style reference — only match commits from Stanislav Jakuschevskij)
2. Analyze the diff and draft a message following all seven rules
3. New features and fixes take priority over refactoring for the commit type
4. Use tandem authorship (Co-Authored-By trailer)
5. If $ARGUMENTS is provided, add "Issue $ARGUMENTS" at the bottom
6. Show the message and the `git commit` command — do NOT execute it
