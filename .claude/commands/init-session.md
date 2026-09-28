---
allowed-tools: Bash(git log:*), Bash(pwd), Bash(./init.sh), Bash(yarn test*), Bash(cat .dev-env.json), Read
description: Run startup sequence and pick a story from the Jira epic
---

# Session Onboard

## Steps

1. **Confirm working directory** — run `pwd`.
2. **Orient** — read `docs/claude-progress.txt` (only the last 3 days of entries matter) and run:

   ```bash
   git log --oneline --since="3 days ago"
   ```

3. **Check struggles** — read `docs/agent-struggles.json`. If unresolved entries exist, present to user.
4. **CI check** — run `yarn ci` (lint, test, build) and verify the project is healthy.
5. **Start dev env** — run `./init.sh`. If it fails (e.g. nono sandbox blocks `oc`), tell the user to start it manually from their terminal.
6. **Read ports** — read `.dev-env.json` and note the backend, plugin, and console ports. If init.sh failed, skip this step.
7. **Pick story** — tell the user you're oriented and propose picking a story from the PoC epic: <https://redhat.atlassian.net/browse/SRVOCF-810>. Ask the user to provide the story description (title, acceptance criteria, or a Jira link). The user may pick one story or a few small ones.
8. **Create feature entry** — once the user provides a story, create a new entry in `docs/features.json` for it (append to the array, `"passes": false`).
9. **Branch** — create a feature branch per [Branching](docs/WORKFLOW.md#branching) convention and open a draft PR (`gh pr create --draft`).
10. **Propose planning** — tell the user the branch is ready and propose to start planning (step 2 of the Feature Development Sequence in `docs/WORKFLOW.md`). Do NOT start any work autonomously.
