# Page Co-location Restructure Implementation Plan

> **REQUIRED SUB-SKILL:** Use the executing-plans skill to implement this plan task-by-task.

**Goal:** Restructure `src/` so page-specific components live in `src/pages/<name>/components/` and shared code lives in `src/common/`.

**Architecture:** Each page gets a directory under `src/pages/` with a `components/` subdir for page-specific components. Shared components, services, utils, and context move to `src/common/`. The ownership rule: if a component is imported by only one page (tests don't count), it lives in that page's `components/` dir. If imported by multiple pages, it lives in `src/common/components/`.

**Tech Stack:** TypeScript, Vitest, Webpack Module Federation (OCP dynamic plugin SDK)

---

## Task 1: Create directory structure and move shared code

**Files:**
- Create: `src/common/components/` (directory)
- Create: `src/common/services/` (directory)
- Create: `src/common/utils/` (directory)
- Create: `src/common/context/` (directory)
- Move: `src/components/UserAvatar.tsx` -> `src/common/components/UserAvatar.tsx`
- Move: `src/components/UserAvatar.test.tsx` -> `src/common/components/UserAvatar.test.tsx`
- Move: `src/services/*` -> `src/common/services/*` (entire directory contents)
- Move: `src/utils/*` -> `src/common/utils/*`
- Move: `src/context/*` -> `src/common/context/*`

**Step 1: Create directories and move files**

```bash
mkdir -p src/common/components src/common/services src/common/utils src/common/context
mkdir -p src/pages

# Shared component
git mv src/components/UserAvatar.tsx src/common/components/UserAvatar.tsx
git mv src/components/UserAvatar.test.tsx src/common/components/UserAvatar.test.tsx

# Services (preserve subdirectory structure)
git mv src/services/types.ts src/common/services/types.ts
git mv src/services/cluster src/common/services/cluster
git mv src/services/function src/common/services/function
git mv src/services/source-control src/common/services/source-control

# Utils
git mv src/utils/utils.ts src/common/utils/utils.ts
git mv src/utils/utils.test.ts src/common/utils/utils.test.ts

# Context
git mv src/context/ForgeConnectionProvider.tsx src/common/context/ForgeConnectionProvider.tsx
```

**Step 2: Update imports in moved files**

All files under `src/common/` that import from siblings need their relative paths updated. Key files:

- `src/common/components/UserAvatar.tsx`: update import of `ForgeConnectionProvider` from `../context/` to `../context/`  (same relative path, no change needed)
- `src/common/services/*/use*.ts`: imports of `types.ts` change from `../types` to `../types` (same, no change)
- `src/common/components/UserAvatar.test.tsx`: update import path from `./UserAvatar` (stays same)

Within `src/common/` the relative paths between services, utils, context stay the same because the entire subtree moved together. No changes needed within common.

**Step 3: Run tests to verify**

```bash
yarn test
```

Expected: FAIL (imports in views/ and remaining components/ still point to old paths). That's expected, we fix those in the next tasks.

**Step 4: Commit**

```bash
git add -A
git commit -m "refactor: create common/ directory and move shared code"
```

---

## Task 2: Move FunctionsListPage and its components

**Files:**
- Create: `src/pages/function-list/components/` (directory)
- Move: `src/views/FunctionsListPage.tsx` -> `src/pages/function-list/FunctionsListPage.tsx`
- Move: `src/views/FunctionsListPage.test.tsx` -> `src/pages/function-list/FunctionsListPage.test.tsx`
- Move: `src/components/FunctionTable.tsx` -> `src/pages/function-list/components/FunctionTable.tsx`
- Move: `src/components/FunctionTable.test.tsx` -> `src/pages/function-list/components/FunctionTable.test.tsx`
- Move: `src/components/EmptyState.tsx` -> `src/pages/function-list/components/EmptyState.tsx`
- Move: `src/components/EmptyState.test.tsx` -> `src/pages/function-list/components/EmptyState.test.tsx`

**Step 1: Create directory and move files**

```bash
mkdir -p src/pages/function-list/components

git mv src/views/FunctionsListPage.tsx src/pages/function-list/FunctionsListPage.tsx
git mv src/views/FunctionsListPage.test.tsx src/pages/function-list/FunctionsListPage.test.tsx
git mv src/components/FunctionTable.tsx src/pages/function-list/components/FunctionTable.tsx
git mv src/components/FunctionTable.test.tsx src/pages/function-list/components/FunctionTable.test.tsx
git mv src/components/EmptyState.tsx src/pages/function-list/components/EmptyState.tsx
git mv src/components/EmptyState.test.tsx src/pages/function-list/components/EmptyState.test.tsx
```

**Step 2: Update imports in FunctionsListPage.tsx**

Old imports like `../components/EmptyState` become `./components/EmptyState`. Old imports like `../components/UserAvatar` become `../../common/components/UserAvatar`. Old imports like `../services/...` become `../../common/services/...`. Old imports like `../context/...` become `../../common/context/...`. Old imports like `../utils/...` become `../../common/utils/...`.

**Step 3: Update imports in FunctionsListPage.test.tsx**

Old imports like `./FunctionsListPage` stay the same. Old imports like `../../testing/msw/server` become `../../../testing/msw/server`.

**Step 4: Update imports in FunctionTable.tsx and FunctionTable.test.tsx**

`FunctionTable.tsx`: old imports like `../services/types` become `../../../common/services/types`.
`FunctionTable.test.tsx`: old imports like `./FunctionTable` stay the same. Old imports like `../services/types` become `../../../common/services/types`.

**Step 5: Update imports in EmptyState.tsx and EmptyState.test.tsx**

Minimal changes, mostly path adjustments for any common imports.

**Step 6: Run tests for this page**

```bash
yarn test src/pages/function-list/
```

Expected: PASS for all FunctionsListPage and component tests.

**Step 7: Commit**

```bash
git add -A
git commit -m "refactor: move FunctionsListPage to pages/function-list/"
```

---

## Task 3: Move FunctionCreatePage and its components

**Files:**
- Create: `src/pages/function-create/components/` (directory)
- Move: `src/views/FunctionCreatePage.tsx` -> `src/pages/function-create/FunctionCreatePage.tsx`
- Move: `src/views/FunctionCreatePage.test.tsx` -> `src/pages/function-create/FunctionCreatePage.test.tsx`
- Move: `src/components/CreateFunctionForm.tsx` -> `src/pages/function-create/components/CreateFunctionForm.tsx`
- Move: `src/components/CreateFunctionForm.test.tsx` -> `src/pages/function-create/components/CreateFunctionForm.test.tsx`

**Step 1: Create directory and move files**

```bash
mkdir -p src/pages/function-create/components

git mv src/views/FunctionCreatePage.tsx src/pages/function-create/FunctionCreatePage.tsx
git mv src/views/FunctionCreatePage.test.tsx src/pages/function-create/FunctionCreatePage.test.tsx
git mv src/components/CreateFunctionForm.tsx src/pages/function-create/components/CreateFunctionForm.tsx
git mv src/components/CreateFunctionForm.test.tsx src/pages/function-create/components/CreateFunctionForm.test.tsx
```

**Step 2: Update imports**

Same pattern as Task 2. Page-specific components: `../components/CreateFunctionForm` -> `./components/CreateFunctionForm`. Common imports: `../services/...` -> `../../common/services/...`, etc.

**Step 3: Run tests for this page**

```bash
yarn test src/pages/function-create/
```

Expected: PASS.

**Step 4: Commit**

```bash
git add -A
git commit -m "refactor: move FunctionCreatePage to pages/function-create/"
```

---

## Task 4: Move FunctionEditPage and extract its components

This is the largest task. FunctionEditPage has EditToolbar, useEditToolbar, and LeaveModal inlined. These need to be extracted into separate files under `components/`.

**Files:**
- Create: `src/pages/function-edit/components/` (directory)
- Move: `src/views/FunctionEditPage.tsx` -> `src/pages/function-edit/FunctionEditPage.tsx`
- Move: `src/views/FunctionEditPage.test.tsx` -> `src/pages/function-edit/FunctionEditPage.test.tsx`
- Move: `src/components/FileTreeView.tsx` -> `src/pages/function-edit/components/FileTreeView.tsx`
- Move: `src/components/FileTreeView.test.tsx` -> `src/pages/function-edit/components/FileTreeView.test.tsx`
- Create: `src/pages/function-edit/components/EditToolbar.tsx` (extracted from FunctionEditPage.tsx)
- Create: `src/pages/function-edit/components/LeaveModal.tsx` (extracted from FunctionEditPage.tsx)

**Step 1: Create directory and move files**

```bash
mkdir -p src/pages/function-edit/components

git mv src/views/FunctionEditPage.tsx src/pages/function-edit/FunctionEditPage.tsx
git mv src/views/FunctionEditPage.test.tsx src/pages/function-edit/FunctionEditPage.test.tsx
git mv src/components/FileTreeView.tsx src/pages/function-edit/components/FileTreeView.tsx
git mv src/components/FileTreeView.test.tsx src/pages/function-edit/components/FileTreeView.test.tsx
```

**Step 2: Extract LeaveModal into its own file**

Create `src/pages/function-edit/components/LeaveModal.tsx` containing the `LeaveModal` function component. Remove it from `FunctionEditPage.tsx`. The component is small (~20 lines) and has no hook.

**Step 3: Extract EditToolbar into its own file**

Create `src/pages/function-edit/components/EditToolbar.tsx` containing the `EditToolbar` component and its `useEditToolbar` hook (hook stays in same file per architecture rules, since it's only used by EditToolbar). Import `LeaveModal` from `./LeaveModal`. Remove both from `FunctionEditPage.tsx`.

**Step 4: Update imports in FunctionEditPage.tsx**

- Import `EditToolbar` from `./components/EditToolbar`
- Import `FileTreeView` from `./components/FileTreeView`
- Remove the inlined EditToolbar, useEditToolbar, LeaveModal, and EditToolbarProps
- Update common imports: `../components/UserAvatar` -> `../../common/components/UserAvatar`, `../services/...` -> `../../common/services/...`, etc.
- Remove PF imports that were only used by EditToolbar/LeaveModal (Toolbar, ToolbarContent, ToolbarGroup, ToolbarItem, Modal, ModalBody, ModalFooter, ModalHeader, ArrowLeftIcon). Keep only imports still used by FunctionEditPage itself.

**Step 5: Update imports in FileTreeView.tsx and FileTreeView.test.tsx**

`FileTreeView.tsx`: `../services/types` -> `../../../common/services/types`.
`FileTreeView.test.tsx`: `../services/types` -> `../../../common/services/types`.

**Step 6: Update imports in FunctionEditPage.test.tsx**

MSW server import path, etc.

**Step 7: Run tests for this page**

```bash
yarn test src/pages/function-edit/
```

Expected: PASS.

**Step 8: Commit**

```bash
git add -A
git commit -m "refactor: move FunctionEditPage to pages/function-edit/, extract EditToolbar and LeaveModal"
```

---

## Task 5: Update webpack exposed modules and clean up

**Files:**
- Modify: `package.json` (exposedModules paths)
- Delete: `src/views/` (should be empty)
- Delete: `src/components/` (should be empty)
- Delete: `src/services/` (should be empty)
- Delete: `src/utils/` (should be empty)
- Delete: `src/context/` (should be empty)

**Step 1: Update package.json exposedModules**

Change the paths in the `consolePlugin.exposedModules` section:

```json
"exposedModules": {
  "FunctionsListPage": "./pages/function-list/FunctionsListPage",
  "FunctionCreatePage": "./pages/function-create/FunctionCreatePage",
  "FunctionEditPage": "./pages/function-edit/FunctionEditPage"
}
```

**Step 2: Remove empty directories**

```bash
rmdir src/views src/components src/services src/utils src/context 2>/dev/null || true
# If any directory is not empty, investigate what's left
```

**Step 3: Run full test suite**

```bash
yarn test
```

Expected: 13 suites, 112 tests, all PASS.

**Step 4: Run lint**

```bash
yarn lint
```

Expected: PASS, zero errors.

**Step 5: Commit**

```bash
git add -A
git commit -m "refactor: update webpack exposed modules, remove empty directories"
```

---

## Task 6: Update ARCHITECTURE.md

**Files:**
- Modify: `docs/ARCHITECTURE.md`

**Step 1: Update the layered architecture table and directory mappings**

Update the layer table to reflect the new structure:

| Layer | Maps to | Depends on |
|-------|---------|------------|
| **Types** | `common/services/types.ts` | nothing |
| **Services** | `common/services/*/Service.ts` + implementations | Types, Utils |
| **Hooks** | `common/services/*/use*.ts` | Services, Types, Utils |
| **Components** | `common/components/` (shared), `pages/<name>/components/` (page-specific) | Hooks, Types, Utils |
| **Pages** | `pages/<name>/` | Components, Hooks, Utils |
| **Utils** | `common/utils/` | nothing (cross-cutting) |

Add a section documenting the co-location convention:

### Co-location Convention

- `src/pages/<name>/` contains the page component, its test, and a `components/` subdir
- `src/pages/<name>/components/` contains components used only by that page
- `src/common/` contains everything shared across pages (components, services, utils, context)
- **Ownership rule:** if a component is imported by only one page (test imports don't count), it lives in `pages/<name>/components/`. If imported by multiple pages, it lives in `common/components/`.

**Step 2: Update the "Page / Component / Hook Rules" section**

Rename "Views" references to "Pages" where applicable.

**Step 3: Commit**

```bash
git add docs/ARCHITECTURE.md
git commit -m "docs: update ARCHITECTURE.md with co-location convention"
```

---

## Task 7: Update TESTING.md

**Files:**
- Modify: `docs/TESTING.md`

**Step 1: Add component vs. page test rule**

Add a new section "## Component vs. Page Tests" after "## What Gets Tested":

```markdown
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
```

**Step 2: Update "What Gets Tested" table**

Rename "Views" to "Pages" in the table:

| Artifact | Test type | Example |
|----------|-----------|----------|
| Pages | Component + E2e | `FunctionsListPage` shows empty state, table |

**Step 3: Update "File Conventions" table**

Update paths to reflect new structure:

| Type | Location |
|------|----------|
| Component tests | `src/pages/<name>/components/*.test.ts\|tsx`, `src/common/components/*.test.ts\|tsx` |
| Page tests | `src/pages/<name>/*.test.ts\|tsx` |
| Service / Hook / Util tests | `src/common/**/*.test.ts\|tsx` |
| E2e specs | `e2e/<feature-name>/*.test.ts` |
| MSW handlers | `testing/msw/handlers.ts` |

**Step 4: Update mock strategy and tooling references**

The codebase has migrated from Jest to Vitest and is migrating towards MSW for all mocking. Update TESTING.md to reflect this:

- In "## Test Layers" table: replace "Jest + React Testing Library" with "Vitest + React Testing Library"
- In "## Mock Strategy": replace "Jest mocks" / "jest.mock" references with "vi.mock" (Vitest). Clarify that MSW is the primary mocking strategy for anything that hits the network (GitHub API, K8s API, Go backend). `vi.mock` is only for framework and library internals that have no external service, e.g. `react-i18next`, `@openshift-console/dynamic-plugin-sdk` (console shell runtime), `@patternfly/react-icons`, `react-router-dom-v5-compat`, `libsodium-wrappers` (WASM). K8s API mocking will use MSW WebSocket capability.
- Replace the "## Mocking Patterns" section. MSW is the primary approach. `vi.mock` is rare. Update all code examples from `jest.mock` / `jest.fn` / `jest.restoreAllMocks` to `vi.mock` / `vi.fn` / `vi.restoreAllMocks`. Keep the forbidden patterns (no `require()`, no JSX in mocks).

**Step 5: Commit**

```bash
git add docs/TESTING.md
git commit -m "docs: update TESTING.md with component vs. page test rules, new paths, and Vitest/MSW migration"
```

---

## Task 8: Final verification

**Step 1: Run full test suite**

```bash
yarn test
```

Expected: 13+ suites, 112+ tests, all PASS.

**Step 2: Run lint**

```bash
yarn lint
```

Expected: zero errors.

**Step 3: Verify directory structure**

```bash
find src -type f \( -name "*.ts" -o -name "*.tsx" \) | sort
```

Expected: all files under `src/pages/` or `src/common/`, no files under old `src/views/`, `src/components/`, `src/services/`, `src/utils/`, or `src/context/`.

**Step 4: Verify dev server builds**

```bash
yarn build
```

Expected: webpack builds without errors, exposed modules resolve correctly.
