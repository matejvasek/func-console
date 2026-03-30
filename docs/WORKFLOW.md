# Workflow — func-console

## Startup Sequence

Every session, before doing any work:

1. `pwd` — confirm working directory
2. Read `docs/claude-progress.txt` + `git log --oneline -10` — orient
3. Read `docs/agent-struggles.json` — if unresolved entries exist, present to user
4. Read `docs/features.json` — pick first `"passes": false` entry
5. Run `./init.sh` — start dev env
6. Run tests — verify app is healthy
7. If broken → fix first. If clean → start [Feature Development Sequence](#feature-development-sequence).

## Feature Development Sequence

After [Startup Sequence](#startup-sequence), work through the picked feature:

1. **Plan** — read `docs/ARCHITECTURE.md` + `docs/STYLEGUIDE.md` + `docs/TESTING.md`, then create implementation plan → `docs/plans/active/<incremented-number>-feature-<short-name>.md`
2. **Branch** — create feature branch per [Branching](#branching) convention
3. **Implement** — using `/executing-plans` skill
4. **Review** — code review using `/requesting-code-review` skill, fix found issues
5. **Manual Test** — use browser automation and validate it works in the browser
6. **Complete** — flip `passes` to `true` in [`docs/features.json`](references/features-json-readme.md), update [`docs/claude-progress.txt`](references/claude-progress-readme.md), commit
7. **PR** — push branch, open PR per [Pull Requests](#pull-requests) convention
8. Move plan to `docs/plans/completed/`
9. Stop, wait for user command

## Branching

Create a feature branch per plan: `<NNN>-<type>-<short-name>` where `<NNN>` matches the plan number and `<type>` the conventional commit type as per our [Git Commit Guide](references/commit-message-agent-readme.md#conventional-commits). Example: `001-feat-function-list-empty-state`.

## Pull Requests

Open PRs via `gh pr create` using the template at `.github/pull_request_template.md`.

**Title format:** `<Type>: <Sentence ending with a period.>` — capitalize the type and the first word, end with a period. Example: `Feat: Add function list page with empty state.`

Types are the same as [conventional commits](references/commit-message-agent-readme.md#conventional-commits) but capitalized.

## Session Rules

- One feature at a time
- Clean state at end (code suitable for merging to main)
- Update [`docs/claude-progress.txt`](references/claude-progress-readme.md) before session ends
- Commit work to git before ending — follow [`docs/references/commit-message-agent-readme.md`](references/commit-message-agent-readme.md) strictly

## Continuous Improvement

When you struggle, don't silently work around it. Log it to `docs/agent-struggles.json` — see [`docs/references/agent-struggles-readme.md`](references/agent-struggles-readme.md) for format.
