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

1. **Plan** — read `docs/ARCHITECTURE.md` + `docs/STYLEGUIDE.md` + `docs/TESTING.md`, then use `/brainstorming` to design the chosen feature from `docs/features.json`, then use `/writing-plans` to create implementation plan → `docs/plans/active/<NNN>-<type>-<short-name>.md`
2. **Branch** — create feature branch per [Branching](#branching) convention
3. **Implement** — using `/executing-plans` skill
4. **Review** — code review using `/requesting-code-review` skill, fix found issues
5. **Manual Test** — use browser automation and validate it works in the browser
6. **Complete** — flip `passes` to `true` in [`docs/features.json`](references/features-json-readme.md), update [`docs/claude-progress.txt`](references/claude-progress-readme.md), move plan to `docs/plans/completed/`, commit
7. **PR** — push branch, open PR per [Pull Requests](#pull-requests) convention
8. Stop — wait for PR review. Rework per [Received PR Reviews](#received-pr-reviews) when asked.

## Received PR Reviews

For each comment: read the full text and its diff hunk context, make the fix, then re-read the comment and verify your change actually matches what was asked (placement, naming, scope — not just compilation). Reply in the thread stating what changed.

## Branching

Create a feature branch per plan: `<NNN>-<type>-<short-name>` where `<NNN>` matches the plan number and `<type>` the conventional commit type as per our [Git Commit Guide](references/commit-message-guide.md#conventional-commits). Example: `001-feat-function-list-empty-state`. If we're on a feature branch already do nothing.

## Pull Requests

Open PRs via `gh pr create` using the template at `.github/pull_request_template.md`.

**Title format:** `<Type>: <Sentence ending with a period.>` — capitalize the type and the first word, end with a period. Example: `Feat: Add function list page with empty state.`

Types are the same as [conventional commits](references/commit-message-guide.md#conventional-commits) but capitalized.

No em dashes (`—`) in PR titles or descriptions. Use commas, periods, or parentheses instead.

## Session Rules

- One feature at a time
- Clean state at end (code suitable for merging to main)
- Update [`docs/claude-progress.txt`](references/claude-progress-readme.md) before session ends
- Commit work to git before ending — follow [`docs/references/commit-message-guide.md`](references/commit-message-guide.md) strictly

## Continuous Improvement

When you struggle, don't silently work around it. Log it to `docs/agent-struggles.json` — see [`docs/references/agent-struggles-readme.md`](references/agent-struggles-readme.md) for format.
