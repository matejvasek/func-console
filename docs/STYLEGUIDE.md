# Style Guide — func-console

## Coding Guidelines

1. Understand: Clarify the task and requirements
2. Plan: Break down complex tasks
3. Execute: We are using red/green/refactor TDD. Read `docs/TESTING.md` for details.
4. Document: Capture decisions and outcomes

## Code Style

- **Minimize visibility**: Make everything private by default. Only expose when external packages require access. This applies to all programming language constructs that can be encapsulated.
- **Use early returns**: Check error conditions first and return early. Avoid deeply nested if-else chains.
- **Avoid nesting**: Reduce nesting depth by using early returns, guard clauses, and extracting complex conditions into well-named boolean variables or functions.
- **No unnecessary boilerplate**: Only write code that directly helps pass tests or improves maintainability. Skip ceremonial code, unused helper functions, or defensive checks that tests don't require.
- **Write minimal code**: Satisfy the requirements with the least code necessary. If a simple solution works, use it—don't add abstractions, helpers, or extra structure unless complexity demands it. When requirements force more code, that's fine, but always start with the simplest approach that could work.
- **No `any` type**: Use proper TypeScript types.
- **No `console.log`**: Use structured approach if logging needed.
- **Naming**: `use*` for hooks, `*Service` for services, PascalCase for components/types.

## Documentation

- **No em dashes (`—`)**. Use commas, periods, or parentheses instead.

## OCP Plugin Styling Constraints

The `.stylelintrc.yaml` enforces strict rules to prevent breaking the OpenShift Console:

- **No hex colors** — use PatternFly CSS variables (e.g., `var(--pf-v6-global-palette--blue-500)`). Hex colors break dark mode.
- **No naked element selectors** (like `table`, `div`) — prevents overwriting console styles.
- **No `.pf-` or `.co-` prefixed classes** — these are reserved for PatternFly and console.
- **Prefix all custom classes** with the plugin name (e.g., `func-console__my-component`).
