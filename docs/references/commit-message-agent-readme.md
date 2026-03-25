# Git Commit Guide (Agent)

Stage and commit changes directly. Follow these rules for every commit.

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
<type>: <subject - max 50 chars total, imperative mood, conventional commit spec>

<Body - wrapped at 72 chars, explains what and why>
<Optional bullet points for clarity>
```

## Conventional Commits

- Use conventional commits spec with types: feat, fix, refactor, docs, test, chore, style, perf, ci, build
- Format: `<type>: <description>`
- Use "feat" only on user-facing changes

## Subject Line

- Write as if completing: "If applied, this commit will..."
- Include type prefix (conventional commits)
- Total length max 50 chars including type

## Body

- Explain the motivation for the change
- Explain the "what" and "why" vs. "how"
- Focus on context, not implementation details
- Keep it short if there aren't many changes
- Don't overuse bullet points

## Author

Set git author to Claude for agent commits:

```bash
git commit --author="Claude <noreply@anthropic.com>" -m "<message>"
```

## Steps

1. Run `git diff HEAD` to analyze all changes
2. Stage relevant changes with `git add`
3. Draft a commit message following all seven rules
4. Commit with `git commit --author="Claude <noreply@anthropic.com>" -m "<message>"`

## Changing Commit Ownership

If a human takes over an agent's commit (amends, modifies), change authorship:

```bash
# Amend the last commit and change author to yourself
git commit --amend --author="Your Name <your@email.com>"

# Change author of a specific commit (interactive rebase)
git rebase -i HEAD~N
# Mark the commit with 'edit', then:
git commit --amend --author="Your Name <your@email.com>"
git rebase --continue

# Change author + re-sign
git commit --amend --reset-author
```
