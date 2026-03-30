# AGENTS.md — func-console

This is the project map. Read this first, every session.

## Project

FaaS PoC UI for OpenShift Console — React + TypeScript + Webpack + PatternFly 6 + OCP Dynamic Plugin SDK.
See `docs/design/` for full design specs.

## Knowledge Base

| File | Purpose |
|------|---------|
| `docs/design/` | Design specs — "what to build" |
| `docs/plans/active/` | Implementation plans in progress |
| `docs/plans/completed/` | Finished plans |
| `docs/references/` | LLM-friendly reference material |
| `docs/ARCHITECTURE.md` | Layered architecture, dependency rules |
| `docs/STYLEGUIDE.md` | Code style, naming conventions |
| `docs/TESTING.md` | Testing strategy, tools, conventions, mock patterns |
| `docs/features.json` | Inviolable feature list — ground truth |
| `docs/claude-progress.txt` | Session handoff log |
| `docs/references/ocp-plugin-guide.md` | OCP dynamic plugin mechanics, i18n, extension points |
| `docs/agent-struggles.json` | Struggle log — see `docs/references/agent-struggles-readme.md` |

## Startup Sequence

Every session, before doing any work:

1. `pwd` — confirm working directory
2. Read `docs/claude-progress.txt` + `git log --oneline -10` — orient
3. Read `docs/agent-struggles.json` — if unresolved entries exist, present to user
4. Read `docs/features.json` — pick first `"passes": false` entry
5. Run `./init.sh` — start dev env
6. Run tests — verify app is healthy
7. If broken → fix first. If clean → start [Feature Development Sequence](#feature-development-sequence).

## Branching

Create a feature branch per plan: `<NNN>-<type>-<short-name>` where `<NNN>` matches the plan number and `<type>` the
conventional commit type as per our [Git Commit Guide](/docs/references/commit-message-agent-readme.md#conventional-commits). Example: `001-feat-function-list-empty-state`.

## Feature Development Sequence

After [Startup Sequence](#startup-sequence), work through the picked feature:

1. **Plan** — read `docs/ARCHITECTURE.md` + `docs/STYLEGUIDE.md` + `docs/TESTING.md`, then create implementation plan → `docs/plans/active/<incremented-number>-feature-<short-name>.md`
2. **Branch** — create feature branch per [Branching](#branching) convention
3. **Implement** — using `/executing-plans` skill
4. **Review** — code review using `/requesting-code-review` skill, fix found issues
5. **Manual Test** — use browser automation and validate it works in the browser
6. **Complete** — flip `passes` to `true` in `docs/features.json`, update `docs/claude-progress.txt`, commit
7. Move plan to `docs/plans/completed/`
8. Stop, wait for user command

## Session Rules

- One feature at a time
- Clean state at end (code suitable for merging to main)
- Update `docs/claude-progress.txt` before session ends
- Commit work to git before ending — follow `docs/references/commit-message-agent-readme.md` strictly

## features.json

Inviolable. The categories are `functional` or `technical`. You may only change the `passes` field to `true` — and only after:

1. The corresponding e2e test passes
2. You have validated the feature in a real browser via browser automation

Never remove, reorder, or edit feature entries. Work on the first entry where `passes` is `false`.

Format:

```json
{
  "category": "functional",
  "description": "...",
  "steps": ["..."],
  "passes": false
}
```

## claude-progress.txt

Append-only, newest-first. Add your entry at the top after the header. Format:

```txt
---
## YYYY-MM-DD | Session: <brief title>
Worked on: <what you did>
Completed:
- <item>
Left off: <what remains>
Blockers: <any blockers or None>
```

## Continuous Improvement

When you struggle, don't silently work around it. Log it to `docs/agent-struggles.json`:

```json
{
  "date": "YYYY-MM-DD",
  "description": "...",
  "cause": "...",
  "suggestion": "...",
  "resolved": false
}
```
