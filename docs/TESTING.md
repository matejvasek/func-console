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
| Unit / Component | Jest + React Testing Library | Hooks, services, component rendering, form logic |
| E2e / Feature validation | Cypress | Validate features.json entries in real browser |
| API mocking | MSW (Mock Service Worker) | GitHub API + K8s API — mock everything first, real cluster later |

## Mock Strategy

Use MSW for all API mocking (GitHub + K8s). If SDK hooks require module-level mocks in unit tests (because they depend on Console runtime internals), fall back to Jest mocks — but try MSW first.

- **Start:** Mock everything via MSW (GitHub API, K8s API)
- **Later:** Real cluster for e2e — SDK hooks work natively, GitHub API remains mocked
- **Fallback:** If SDK hooks can't be driven by MSW alone in unit tests, use Jest module mocks

## File Conventions

| Type | Location |
|------|----------|
| Unit / Component tests | `src/**/*.test.ts\|tsx` |
| E2e specs | `e2e/<feature-name>/*.test.ts` |
| MSW handlers | `testing/msw/handlers.ts` |

## What Gets Tested

| Artifact | Test type | Example |
|----------|-----------|---------|
| Service interfaces | Unit | `FunctionService.generateFunction()` returns expected files |
| React hooks | Unit | `useFunctionService()` returns service instance |
| Components | Component | `CreateForm` renders all fields, validates input |
| Views | Component + E2e | `FunctionListPage` shows empty state, table |
| User flows | E2e | Create form → submit → list shows new function |

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

## Mocking Patterns

Use ESM `import` at top of file. Never use `require('react')` or `React.createElement()` in mocks.
Prefer `jest.mock()` for modules, `jest.fn()` for components. Keep mocks simple.

**Correct patterns:**

```typescript
// Return null
jest.mock('../MyComponent', () => () => null);

// Return string
jest.mock('../LoadingSpinner', () => () => 'Loading...');

// Return children directly
jest.mock('../Wrapper', () => ({ children }) => children);

// Track calls with jest.fn
jest.mock('../ButtonBar', () => jest.fn(({ children }) => children));

// Mock custom hooks
jest.mock('../useCustomHook', () => ({
  useCustomHook: jest.fn(() => [/* mock data */]),
}));
```

**Forbidden patterns:**

```typescript
// NEVER - require() in mocks
jest.mock('../Component', () => {
  const React = require('react');
  return () => React.createElement('div');
});

// NEVER - JSX in mocks
jest.mock('../Component', () => () => <div>Mock</div>);
```

**Clean up mocks:**

```typescript
afterEach(() => {
  jest.restoreAllMocks();
});
```

## Visual Verification with Playwright

Playwright is installed in the project and available for ad-hoc visual checks during development. Use it to verify UI changes render correctly in a real browser, especially during the Manual Test step of the [Feature Development Sequence](WORKFLOW.md#feature-development-sequence).

**When to use:**

- After implementing or modifying UI components
- To verify empty states, error states, modals, and layout
- To check that navigation routes render the expected page
- To debug visual issues that unit/component tests cannot catch

**Setup:**

Read `.dev-env.json` to get the console port. The dev servers must be running (`./init.sh`).

**Ready-to-copy recipe:**

```typescript
// Run with: node -e "$(cat <<'SCRIPT' ... SCRIPT)"
// Or save to a temp file and run with: node /tmp/check-ui.js

const { chromium } = require('playwright');
const { readFileSync } = require('fs');

(async () => {
  // Read console port from dev environment config
  const devEnv = JSON.parse(readFileSync('.dev-env.json', 'utf-8'));
  const baseUrl = `http://localhost:${devEnv.consolePort}`;

  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });

  // Navigate to the route you want to verify
  const url = `${baseUrl}/faas`;
  console.log('Navigating to:', url);

  const response = await page.goto(url, { waitUntil: 'networkidle', timeout: 30000 });
  console.log('Status:', response.status());
  console.log('URL:', page.url());

  // Wait for dynamic content to settle
  await page.waitForTimeout(2000);

  // Take a screenshot for visual inspection
  await page.screenshot({ path: '/tmp/playwright-check.png', fullPage: true });
  console.log('Screenshot saved to /tmp/playwright-check.png');

  // Print page title and visible text
  console.log('Title:', await page.title());
  const text = await page.evaluate(() => document.body.innerText.substring(0, 3000));
  console.log('--- Visible text ---');
  console.log(text);

  await browser.close();
})().catch(err => {
  console.error(err.message);
  process.exit(1);
});
```

**Tips:**

- Use `page.screenshot()` and then read the PNG with the `Read` tool to visually inspect the result.
- Use `page.waitForSelector()` to wait for specific elements before taking a screenshot.
- Use `page.click()`, `page.fill()`, and other Playwright actions to interact with the UI before capturing state.
- Adjust the `viewport` size to test responsive layouts.
- The recipe above uses `networkidle` which waits until no network requests are in flight for 500ms. For pages with long-polling or WebSocket connections, use `domcontentloaded` or `load` instead.

## E2e Conventions

- **Selectors:** Prefer `data-test` attributes (`cy.get('[data-test="create-function"]')`) over CSS/ARIA selectors
- **Async:** Use `cy.intercept` for API mocking and assertions, avoid `cy.wait` with arbitrary timeouts
- **MSW integration:** MSW handlers mock GitHub API responses in standalone mode
