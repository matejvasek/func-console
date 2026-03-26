# Style Guide — func-console

## Code Style (TypeScript/React)

- **Use early returns**: Check error conditions first. Avoid deep nesting.
- **No unnecessary boilerplate**: Only code that helps pass tests or improves maintainability.
- **Write minimal code**: Simplest approach that works. No premature abstractions.
- **No `any` type**: Use proper TypeScript types.
- **No `console.log`**: Use structured approach if logging needed.
- **Naming**: `use*` for hooks, `*Service` for services, PascalCase for components/types.
- **Red/green/refactor TDD**: Write tests first, implement to pass, refactor if applicable. See `docs/TESTING.md` for details.
