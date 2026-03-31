# Function List Page — Empty State Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render a Function List Page at `/functions` that shows a PatternFly empty state with a "Create function" button linking to `/functions/create` when no functions exist.

**Architecture:** A `FunctionsListPage` view renders an `EmptyState` component. The view will eventually receive function data via hooks, but for this feature we only implement the empty-state path. The `console-extensions.json` and `package.json` are updated to wire the route and nav item. The existing template `ExamplePage` is removed.

**Tech Stack:** React 17, TypeScript, PatternFly 6, OCP Dynamic Plugin SDK, Jest + React Testing Library

---

## File Structure

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `jest.config.ts` | Jest configuration |
| Create | `src/test-setup.ts` | Jest DOM setup |
| Create | `src/components/EmptyState.tsx` | Empty state UI — icon, title, body, Create button |
| Create | `src/components/EmptyState.test.tsx` | Component test for EmptyState |
| Modify | `src/views/FunctionsListPage.tsx` | Replace template boilerplate with empty state rendering |
| Modify | `src/views/FunctionsListPage.test.tsx` | View test — renders empty state |
| Modify | `console-extensions.json` | Wire `/functions` route + nav item |
| Modify | `package.json` | Update `exposedModules`, add test script + Jest deps |
| Delete | `src/components/ExamplePage.tsx` | Remove template page |
| Delete | `src/components/example.css` | Remove template styles |

---

### Task 1: Set Up Jest Test Infrastructure

**Files:**
- Create: `jest.config.ts`
- Create: `src/test-setup.ts`
- Modify: `package.json`

- [ ] **Step 1: Install Jest and React Testing Library**

```bash
yarn add -D jest ts-jest @types/jest @testing-library/react @testing-library/jest-dom @testing-library/user-event identity-obj-proxy
```

- [ ] **Step 2: Create Jest config**

Create `jest.config.ts`:

```typescript
import type { Config } from 'jest';

const config: Config = {
  testEnvironment: 'jsdom',
  transform: {
    '^.+\\.tsx?$': 'ts-jest',
  },
  moduleNameMapper: {
    '\\.css$': 'identity-obj-proxy',
  },
  setupFilesAfterSetup: ['./src/test-setup.ts'],
};

export default config;
```

- [ ] **Step 3: Create test setup file**

Create `src/test-setup.ts`:

```typescript
import '@testing-library/jest-dom';
```

- [ ] **Step 4: Add test script to package.json**

Add to `scripts` in `package.json`:

```json
"test": "jest"
```

- [ ] **Step 5: Verify Jest runs (no tests yet, should exit cleanly)**

```bash
yarn test --passWithNoTests
```

Expected: exits 0, "No tests found" or similar clean output.

- [ ] **Step 6: Commit**

```bash
git add jest.config.ts src/test-setup.ts package.json yarn.lock
git commit -m "chore: set up Jest with React Testing Library"
```

---

### Task 2: EmptyState Component (TDD)

**Files:**
- Create: `src/components/EmptyState.test.tsx`
- Create: `src/components/EmptyState.tsx`

**References:**
- PatternFly 6 EmptyState: uses `EmptyState`, `EmptyStateHeader`, `EmptyStateBody`, `EmptyStateActions`, `EmptyStateFooter` from `@patternfly/react-core`
- Design doc: empty state with Create function button linking to `/functions/create`

- [ ] **Step 1: Write the failing test**

Create `src/components/EmptyState.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { FunctionsEmptyState } from './EmptyState';

jest.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

afterEach(() => {
  jest.restoreAllMocks();
});

describe('FunctionsEmptyState', () => {
  it('renders a heading with "No functions found"', () => {
    render(
      <MemoryRouter>
        <FunctionsEmptyState />
      </MemoryRouter>,
    );

    expect(
      screen.getByRole('heading', { name: 'No functions found' }),
    ).toBeInTheDocument();
  });

  it('renders a "Create function" link pointing to /functions/create', () => {
    render(
      <MemoryRouter>
        <FunctionsEmptyState />
      </MemoryRouter>,
    );

    const link = screen.getByRole('link', { name: 'Create function' });
    expect(link).toHaveAttribute('href', '/functions/create');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
yarn test src/components/EmptyState.test.tsx
```

Expected: FAIL — cannot find module `./EmptyState`.

- [ ] **Step 3: Write minimal implementation**

Create `src/components/EmptyState.tsx`:

```tsx
import {
  EmptyState,
  EmptyStateActions,
  EmptyStateBody,
  EmptyStateFooter,
} from '@patternfly/react-core';
import { CubesIcon } from '@patternfly/react-icons';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';

export function FunctionsEmptyState() {
  const { t } = useTranslation('plugin__console-functions-plugin');

  return (
    <EmptyState headingLevel="h2" icon={CubesIcon} titleText={t('No functions found')}>
      <EmptyStateBody>
        {t('Create a serverless function to get started.')}
      </EmptyStateBody>
      <EmptyStateFooter>
        <EmptyStateActions>
          <Link to="/functions/create">{t('Create function')}</Link>
        </EmptyStateActions>
      </EmptyStateFooter>
    </EmptyState>
  );
}
```

Note: PatternFly 6 `EmptyState` accepts `headingLevel`, `icon`, and `titleText` as direct props — no separate `EmptyStateHeader` needed.

- [ ] **Step 4: Run test to verify it passes**

```bash
yarn test src/components/EmptyState.test.tsx
```

Expected: PASS — both tests green.

- [ ] **Step 5: Commit**

```bash
git add src/components/EmptyState.test.tsx src/components/EmptyState.tsx
git commit -m "feat: add FunctionsEmptyState component"
```

---

### Task 3: FunctionsListPage View (TDD)

**Files:**
- Modify: `src/views/FunctionsListPage.test.tsx`
- Modify: `src/views/FunctionsListPage.tsx`

- [ ] **Step 1: Write the failing test**

Replace contents of `src/views/FunctionsListPage.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import FunctionsListPage from './FunctionsListPage';

jest.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

jest.mock('@openshift-console/dynamic-plugin-sdk', () => ({
  DocumentTitle: ({ children }: { children: string }) => children,
  ListPageHeader: ({ title }: { title: string }) => title,
}));

afterEach(() => {
  jest.restoreAllMocks();
});

describe('FunctionsListPage', () => {
  it('renders the empty state when no functions exist', () => {
    render(
      <MemoryRouter>
        <FunctionsListPage />
      </MemoryRouter>,
    );

    expect(
      screen.getByRole('heading', { name: 'No functions found' }),
    ).toBeInTheDocument();
  });

  it('renders a "Create function" link to /functions/create', () => {
    render(
      <MemoryRouter>
        <FunctionsListPage />
      </MemoryRouter>,
    );

    expect(
      screen.getByRole('link', { name: 'Create function' }),
    ).toHaveAttribute('href', '/functions/create');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
yarn test src/views/FunctionsListPage.test.tsx
```

Expected: FAIL — current `FunctionsListPage` renders template boilerplate, not the empty state heading.

- [ ] **Step 3: Write minimal implementation**

Replace contents of `src/views/FunctionsListPage.tsx`:

```tsx
import { DocumentTitle, ListPageHeader } from '@openshift-console/dynamic-plugin-sdk';
import { PageSection } from '@patternfly/react-core';
import { useTranslation } from 'react-i18next';
import { FunctionsEmptyState } from '../components/EmptyState';

export default function FunctionsListPage() {
  const { t } = useTranslation('plugin__console-functions-plugin');

  return (
    <>
      <DocumentTitle>{t('Functions')}</DocumentTitle>
      <ListPageHeader title={t('Functions')} />
      <PageSection>
        <FunctionsEmptyState />
      </PageSection>
    </>
  );
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
yarn test src/views/FunctionsListPage.test.tsx
```

Expected: PASS — both tests green.

- [ ] **Step 5: Run all tests**

```bash
yarn test
```

Expected: all tests pass (EmptyState + FunctionsListPage).

- [ ] **Step 6: Commit**

```bash
git add src/views/FunctionsListPage.tsx src/views/FunctionsListPage.test.tsx
git commit -m "feat: FunctionsListPage renders empty state"
```

---

### Task 4: Wire Route and Nav Item, Clean Up Template

**Files:**
- Modify: `console-extensions.json`
- Modify: `package.json` (exposedModules)
- Delete: `src/components/ExamplePage.tsx`
- Delete: `src/components/example.css`

- [ ] **Step 1: Update console-extensions.json**

Replace contents of `console-extensions.json`:

```json
[
  {
    "type": "console.navigation/section",
    "properties": {
      "id": "functions-section",
      "name": "%plugin__console-functions-plugin~Functions%",
      "perspective": "dev"
    }
  },
  {
    "type": "console.navigation/href",
    "properties": {
      "id": "functions-list",
      "name": "%plugin__console-functions-plugin~Functions%",
      "href": "/functions",
      "section": "functions-section",
      "perspective": "dev"
    }
  },
  {
    "type": "console.page/route",
    "properties": {
      "path": "/functions",
      "component": { "$codeRef": "FunctionsListPage" },
      "exact": true
    }
  }
]
```

- [ ] **Step 2: Update exposedModules in package.json**

Change the `exposedModules` in the `consolePlugin` section of `package.json`:

```json
"exposedModules": {
  "FunctionsListPage": "./views/FunctionsListPage"
}
```

(Remove the old `"ExamplePage": "./components/ExamplePage"` entry.)

- [ ] **Step 3: Delete template files**

```bash
rm src/components/ExamplePage.tsx src/components/example.css
```

- [ ] **Step 4: Verify build compiles**

```bash
yarn build
```

Expected: builds without errors.

- [ ] **Step 5: Run all tests**

```bash
yarn test
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add console-extensions.json package.json
git rm src/components/ExamplePage.tsx src/components/example.css
git commit -m "feat: wire /functions route and nav item, remove template page"
```

---

### Task 5: Manual Browser Validation

- [ ] **Step 1: Start dev server**

```bash
yarn start
```

Expected: webpack dev server starts on port 9001 without errors.

- [ ] **Step 2: Verify in browser**

Open the dev server URL and confirm:
1. The Functions nav item appears in the sidebar
2. Clicking it navigates to `/functions`
3. The empty state renders with "No functions found" heading
4. The "Create function" link points to `/functions/create`

- [ ] **Step 3: Stop dev server**
