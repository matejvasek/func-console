# Agent Struggles Log — README

## Purpose

`docs/agent-struggles.json` is a structured log where agents record difficulties
they encounter during development. It surfaces systemic issues in the harness —
missing docs, unclear architecture, missing tooling — so the user can address
root causes rather than having agents silently work around them.

## Format

Each entry is a JSON object in the array:

```json
{
  "date": "YYYY-MM-DD",
  "description": "What happened — be specific",
  "cause": "A few words describing the root cause",
  "suggestion": "What would fix or prevent this",
  "resolved": false
}
```

## Rules

- **Agents**: Append an entry whenever you encounter ambiguity, repeated failures,
  or missing information that slows you down. Don't silently work around issues.
- **During startup**: Read this file. If any entry has `"resolved": false`,
  present it to the user before starting feature work.
- **Resolution**: Only the user may change `"resolved"` to `true`, after
  addressing the root cause (adding docs, fixing tooling, clarifying architecture).
