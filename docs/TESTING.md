# Testing — func-console

## Approach

Red/green/refactor TDD — **one test at a time**:

1. Write one test case (red)
2. Write the minimum implementation to make it pass (green)
3. Refactor if needed
4. Move to the next test case

Do NOT write all test cases first and then implement everything at once.

## Test Layers

| Layer | Tool | Scope |
|-------|------|-------|
| Unit / Component | Vitest + React Testing Library | Hooks, services, component rendering, form logic |
| E2e / Feature validation | Cypress | Validate features.json entries in real browser |
| API mocking | MSW (Mock Service Worker) | GitHub API + K8s API — mock everything first, real cluster later |

## Mock Strategy

MSW is the primary mocking strategy for anything that hits the network (GitHub API, K8s API, Go backend). K8s API mocking uses MSW WebSocket capability.

`vi.mock` is only for framework and library internals that have no external service:

- `react-i18next` (translation hook)
- `@openshift-console/dynamic-plugin-sdk` (console shell runtime components like DocumentTitle, ListPageHeader, consoleFetchJSON)
- `@patternfly/react-icons` (UI library)
- `react-router-dom-v5-compat` (framework routing)
- `libsodium-wrappers` (WASM crypto library)

If it makes an HTTP or WebSocket call, mock it with MSW, not `vi.mock`.

## File Conventions

| Type | Location |
|------|----------|
| Component tests | `src/pages/<name>/components/*.test.ts\|tsx`, `src/common/components/*.test.ts\|tsx` |
| Page tests | `src/pages/<name>/*.test.ts\|tsx` |
| Service / Hook / Util tests | `src/common/**/*.test.ts\|tsx` |
| E2e specs | `e2e/<feature-name>/*.test.ts` |
| MSW handlers | `testing/msw/handlers.ts` |

## What Gets Tested

| Artifact | Test type | Example |
|----------|-----------|---------|
| Service interfaces | Unit | `FunctionService.generateFunction()` returns expected files |
| React hooks | Unit | `useFunctionService()` returns service instance |
| Components | Component | `CreateForm` renders all fields, validates input |
| Pages | Component + E2e | `FunctionsListPage` shows empty state, table |
| User flows | E2e | Create form → submit → list shows new function |

## Component vs. Page Tests

Every component gets its own exhaustive test file. Every page gets its own test file that tests the page's orchestration and integration with its components.

**Component tests** cover:

- Rendering based on props (all states and variants)
- User interactions that trigger callbacks (clicks, input, form validation)
- Internal state (expand/collapse, selection)

**Page tests** cover:

- Component is present on the page and wired correctly
- Data flows from hooks/services to components (correct props)
- User actions that trigger cross-component effects or service calls (e.g., form submit calls service, then navigates)
- Page-level states: loading, error, empty

Overlap between component tests and page tests is expected and acceptable. They test at different levels: component tests verify the component works in isolation, page tests verify the page's orchestration logic works correctly.

## Testing Best Practices

1. **User-Centric Testing** — Test what users see and interact with.
   Do NOT test: internal component state, private methods, props passed to children, CSS class names, component structure.

2. **Accessibility-First** — Prefer role-based queries (`getByRole`) over generic selectors (`getByTestId`).

3. **Async-Aware** — Handle async updates with `findBy*` and `waitFor`.

4. **TypeScript Safety** — Use proper types for props, state, and mock data.

5. **Arrange-Act-Assert (AAA)** — Structure every test:
   - **Arrange:** Render component with mocks
   - **Act:** Perform user actions
   - **Assert:** Verify expected state

6. **Scoping** — Place beforeEach, afterEach, and afterAll inside describe blocks.

## Mocking Patterns

MSW is the primary approach. `vi.mock` is rare (see Mock Strategy above).

Use ESM `import` at top of file. Never use `require('react')` or `React.createElement()` in mocks.
Keep mocks simple.

**Correct patterns (for the rare `vi.mock` cases):**

```typescript
// Return null
vi.mock('../MyComponent', () => () => null);

// Return string
vi.mock('../LoadingSpinner', () => () => 'Loading...');

// Return children directly
vi.mock('../Wrapper', () => ({ children }) => children);

// Track calls with vi.fn
vi.mock('../ButtonBar', () => vi.fn(({ children }) => children));

// Mock framework hooks
vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));
```

**Forbidden patterns:**

```typescript
// NEVER - require() in mocks
vi.mock('../Component', () => {
  const React = require('react');
  return () => React.createElement('div');
});

// NEVER - JSX in mocks
vi.mock('../Component', () => () => <div>Mock</div>);
```

**Clean up mocks:**

```typescript
afterEach(() => {
  vi.restoreAllMocks();
});
```

## E2e Conventions

- **Selectors:** Prefer `data-test` attributes (`cy.get('[data-test="create-function"]')`) over CSS/ARIA selectors
- **Async:** Use `cy.intercept` for API mocking and assertions, avoid `cy.wait` with arbitrary timeouts
- **MSW integration:** MSW handlers mock GitHub API responses in standalone mode
