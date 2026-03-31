# Function List Table Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Render a PatternFly table of serverless functions on the list page, with real API response shapes hardcoded in hook skeletons. Merging hook combines GitHub repos (source of truth) with cluster deployment status.

**Architecture:** Types → Hook skeletons (hardcoded data) → Merging hook (useFunctionsList) → Component (FunctionTable with pagination) → View (FunctionsListPage). No real API calls yet — hooks return hardcoded data captured from real K8s and GitHub responses. Future features replace hardcoded data with real API calls.

**Tech Stack:** React 17, TypeScript, PatternFly 6 (`@patternfly/react-table`, `Pagination`), OCP Dynamic Plugin SDK, Jest + React Testing Library, Cypress.

---

## Prerequisites (User-Handled)

1. **GitHub repos tagged** — `serverless-function` topic added to repos via `gh repo edit --add-topic`
2. **Function deployed** — `func-demo-26` deployed to namespace `demo` with raw deployer
3. **API responses captured** — saved to `.tmp/`:
   - `.tmp/k8s-deployment-response.json` — `oc get deployment -n demo -l function.knative.dev/name -o json`
   - `.tmp/gh-search-response.json` — `gh api 'search/repositories?q=topic:serverless-function+user:twoGiants'`

---

## Task 1: Types

Create `src/services/types.ts` with `DeployedFunction`, `RepoInfo`, `FunctionListItem` interfaces.

No tests — pure type definitions.

- [ ] Create types
- [ ] `yarn test` — existing 4 tests still pass
- [ ] Commit

---

## Task 2: Hook Skeletons

Create `src/services/cluster/useClusterService.ts` and `src/services/source-control/useSourceControl.ts`. Both return hardcoded data mapped from the captured API responses in `.tmp/`.

No tests — hardcoded data, no logic.

**Mapping rules for useClusterService** (from `.tmp/k8s-deployment-response.json`):

- `name` ← `metadata.labels['function.knative.dev/name']`
- `namespace` ← `metadata.namespace`
- `runtime` ← `metadata.labels['function.knative.dev/runtime']`
- `replicas` ← `spec.replicas`
- `status` ← derived from `readyReplicas` vs `spec.replicas`
- `url` ← `http://<name>.<namespace>.svc`

**Mapping rules for useSourceControl** (from `.tmp/gh-search-response.json`):

- `owner` ← `item.owner.login`
- `name` ← `item.name`
- `url` ← `item.html_url`
- `defaultBranch` ← `item.default_branch`

- [ ] Create hook skeletons
- [ ] `yarn test` — existing 4 tests still pass
- [ ] Commit

---

## Task 3–4: Merging Hook (useFunctionsList)

Create `src/hooks/useFunctionsList.ts` with exported `mergeFunctionData` pure function and `useFunctionsList` hook.

**Test cases** (`src/hooks/useFunctionsList.test.ts`) — implement one at a time (red/green/refactor):

- Merges deployed function with repo data (copies deployment fields)
- Marks functions without deployment as NotDeployed (empty namespace, empty runtime, 0 replicas)
- Returns one item per repo
- Returns empty array when no repos exist

- [ ] TDD cycle per test case
- [ ] `yarn test` — all tests pass
- [ ] Commit

---

## Task 5–6: FunctionTable Component

Create `src/components/FunctionTable.tsx` with PatternFly table and pagination.

**Test cases** (`src/components/FunctionTable.test.tsx`) — implement one at a time (red/green/refactor):

- Renders column headers (name, runtime, status, url, replicas)
- Renders a row for each function
- Renders function name as a link to `/functions/edit/:name`
- Renders shortened URL as clickable external link
- Renders a dash when url is not set
- Renders edit and delete action buttons per row
- Edit links to `/functions/edit/:name`
- Paginates at 20 items by default
- Does not paginate when 20 or fewer items

- [ ] TDD cycle per test case
- [ ] `yarn test` — all tests pass
- [ ] Commit

---

## Task 7: i18n Keys — camelCase Migration

Migrate existing sentence-case i18n keys to camelCase in the translation file, EmptyState component, and EmptyState tests. Add new keys for table columns and actions.

- [ ] Update translation file, component, and tests
- [ ] `yarn test` — all tests pass
- [ ] Commit

---

## Task 8–9: FunctionsListPage — Wire Table

Modify `src/views/FunctionsListPage.tsx` to use `useFunctionsList` and conditionally render table vs empty state.

**Test cases** (`src/views/FunctionsListPage.test.tsx` — rewrite with hook mocks) — implement one at a time (red/green/refactor):

- Renders empty state when no functions exist
- Renders "createFunction" link to `/functions/create`
- Renders function table when functions exist
- Does not render empty state when functions exist

- [ ] TDD cycle per test case
- [ ] `yarn test` — all tests pass
- [ ] Commit

---

## Task 10: E2e Tests

Create `e2e/function-list-table/function-list-table.test.ts`.

**Test cases:**

- Table renders with column headers
- Function rows render from hardcoded data
- Clicking function name navigates to edit page

- [ ] Write and run Cypress tests
- [ ] Commit

---

## Task 11: Finalize

- [ ] Update `docs/claude-progress.txt`
- [ ] Set `"passes": true` in `docs/features.json`
- [ ] Commit
