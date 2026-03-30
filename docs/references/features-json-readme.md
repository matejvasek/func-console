# features.json

Inviolable. The categories are `functional` or `technical`. You may only change the `passes` field to `true` — and only after:

1. The corresponding e2e test passes
2. You have validated the feature in a real browser via browser automation

Never remove, reorder, or edit feature entries. Work on the first entry where `passes` is `false`.

## Format

```json
{
  "category": "functional",
  "description": "...",
  "steps": ["..."],
  "passes": false
}
```
