# Commit Message Guide

Single source of truth for commit message rules. Referenced by slash commands and autonomous agents.

## The Seven Rules

1. Separate subject from body with a blank line
2. Limit the subject line to 50 characters
3. Use conventional commit spec for subject line
4. Do not end the subject line with a period
5. Use the imperative mood in the subject line
6. Wrap the body at 72 characters
7. Use the body to explain what and why vs. how

## Format

```txt
<type>: <subject - max 50 chars total, imperative mood>

<Body - wrapped at 72 chars, explains what and why>
```

No em dashes (`—`). Use commas, periods, or parentheses instead.

## Conventional Commits

- Types: feat, fix, refactor, docs, test, chore, style, perf, ci, build
- Format: `<type>: <description>`
- Use "feat" only for user-facing changes

## Subject Line

- Write as if completing: "If applied, this commit will..."
- Total length max 50 chars including type
- Examples: "refactor: simplify cache handling", "fix: prevent race condition"
- NOT: "refactored cache", "fixed the race"

## Body

- Explain the motivation for the change
- Explain what and why, not how:
  - How things worked before (and what was wrong)
  - How they work now
  - Why you chose this approach
  - If you're not sure about the "why" — ask the user
- Keep it short if there aren't many changes
- Don't overuse bullet points

### Example

From [Bitcoin Core](https://github.com/bitcoin/bitcoin/commit/eb0b56b19017ab5c16c745e6da39c53126924ed6):

```txt
refactor: simplify serialize.h's exception handling

Remove the 'state' and 'exceptmask' from serialize.h's stream
implementations, as well as related methods.

As exceptmask always included 'failbit', and setstate was always
called with bits = failbit, all it did was immediately raise an
exception. Get rid of those variables, and replace the setstate
with direct exception throwing (which also removes some dead
code).
```

## Authorship

Two modes depending on who drives the work:

**Tandem (working together with user):** User is the git author. Append trailer:

```txt
Co-Authored-By: Claude <noreply@anthropic.com>
```

**Autonomous (agent working alone):** Agent is the git author:

```bash
git commit --author="Claude <noreply@anthropic.com>" -m "<message>"
```

Default to tandem if the user is present in the conversation.

## Changing Commit Ownership

If a human takes over an agent's commit (amends, modifies), change authorship:

```bash
git commit --amend --reset-author
```
