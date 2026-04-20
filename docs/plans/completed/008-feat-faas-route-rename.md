# FaaS Route Rename Implementation Plan

> **REQUIRED SUB-SKILL:** Use the executing-plans skill to implement this plan task-by-task.

**Goal:** Rename all routes from `/functions/*` to `/faas/*`, flatten the dev perspective nav to a single top-level "FaaS" item, and add an admin perspective nav entry under Workloads.

**Architecture:** Pure configuration and string replacement. No new components, hooks, or services. Changes touch console-extensions.json (nav + routes), i18n, source files with hardcoded paths, and their tests.

**Tech Stack:** OCP Dynamic Plugin SDK (console.navigation/*, console.page/route), react-i18next

---

### Task 1: Update tests to expect `/faas` routes

**Files:**

- Modify: `src/components/EmptyState.test.tsx:24-32`
- Modify: `src/views/FunctionCreatePage.test.tsx:94`

**Step 1: Update EmptyState test**

In `src/components/EmptyState.test.tsx`, change the test name and assertion:

```typescript
// line 24
it('renders a "Create function" link pointing to /faas/create', () => {
```

```typescript
// line 32
expect(link).toHaveAttribute('href', '/faas/create');
```

**Step 2: Update FunctionCreatePage test**

In `src/views/FunctionCreatePage.test.tsx`, change the navigation assertion:

```typescript
// line 94
expect(mockNavigate).toHaveBeenCalledWith('/faas');
```

**Step 3: Run tests to verify they fail**

Run: `yarn test`
Expected: 2 failures (EmptyState link href, FunctionCreatePage navigate call)

---

### Task 2: Update source files to use `/faas` routes

**Files:**

- Modify: `src/components/EmptyState.tsx:22` — change `/functions/create` to `/faas/create`
- Modify: `src/views/FunctionCreatePage.tsx:61,70` — change `/functions` to `/faas`
- Modify: `src/views/FunctionsListPage.tsx:38,104` — change `/functions/create` to `/faas/create` and `/functions/edit/` to `/faas/edit/`

**Step 1: Update EmptyState.tsx**

```typescript
// line 22
component={(props) => <Link {...props} to="/faas/create" />}
```

**Step 2: Update FunctionCreatePage.tsx**

```typescript
// line 61
navigate('/faas');
```

```typescript
// line 70
navigate('/faas');
```

**Step 3: Update FunctionsListPage.tsx**

```typescript
// line 38
component={(props) => <Link {...props} to="/faas/create" />}
```

```typescript
// line 104
const onEdit = (name: string) => navigate(`/faas/edit/${name}`);
```

**Step 4: Run tests to verify they pass**

Run: `yarn test`
Expected: All 34 tests pass

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: rename routes from /functions to /faas

Avoid conflict with the legacy Serverless 1.x Functions
console plugin. All internal links and navigation calls
updated to use /faas paths.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 3: Restructure console-extensions.json

**Files:**

- Modify: `console-extensions.json`

**Step 1: Update i18n strings**

In `locales/en/plugin__console-functions-plugin.json`, add the "FaaS" key:

```json
"FaaS": "FaaS",
```

**Step 2: Rewrite console-extensions.json**

Replace the entire file with:

```json
[
  {
    "type": "console.navigation/href",
    "properties": {
      "id": "faas-list",
      "name": "%plugin__console-functions-plugin~FaaS%",
      "href": "/faas",
      "perspective": "dev"
    }
  },
  {
    "type": "console.navigation/separator",
    "properties": {
      "id": "faas-separator",
      "perspective": "admin",
      "section": "workloads",
      "insertAfter": ["statefulsets", "replicationcontrollers", "horizontalpodautoscalers", "poddisruptionbudgets", "deploymentconfigs"]
    }
  },
  {
    "type": "console.navigation/href",
    "properties": {
      "id": "faas-list-admin",
      "name": "%plugin__console-functions-plugin~FaaS%",
      "href": "/faas",
      "perspective": "admin",
      "section": "workloads",
      "insertAfter": "faas-separator"
    }
  },
  {
    "type": "console.page/route",
    "properties": {
      "path": "/faas",
      "component": { "$codeRef": "FunctionsListPage" }
    }
  },
  {
    "type": "console.page/route",
    "properties": {
      "path": "/faas/create",
      "component": { "$codeRef": "FunctionCreatePage" },
      "exact": true
    }
  },
  {
    "type": "console.page/route",
    "properties": {
      "path": "/faas/edit/:name",
      "component": { "$codeRef": "FunctionEditPage" }
    }
  }
]
```

Key changes:

- Dev perspective: removed `console.navigation/section`, single top-level `console.navigation/href` with `"name": "FaaS"`
- Admin perspective: `console.navigation/separator` + `console.navigation/href` both under `"section": "workloads"`, separator uses `insertAfter` array to land after the last Workloads item
- All route paths changed from `/functions/*` to `/faas/*`

**Step 3: Run tests to verify nothing broke**

Run: `yarn test`
Expected: All 34 tests pass

**Step 4: Commit**

```bash
git add -A
git commit -m "feat: restructure nav for dev and admin views

Dev perspective: flatten section+href into single
top-level FaaS nav item. Admin perspective: add FaaS
nav item under Workloads with separator. Add FaaS
i18n key.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 4: Manual verification

**Step 1: Restart dev environment**

```bash
./init.sh
```

**Step 2: Verify in browser**

Open `http://localhost:9000` and check:

- Developer perspective: "FaaS" appears as a top-level nav item (no section wrapper)
- Clicking "FaaS" navigates to `/faas` and renders the list page
- "Create function" button links to `/faas/create`
- Admin perspective: "FaaS" appears under Workloads at the bottom with a separator line above it
- Clicking admin "FaaS" navigates to `/faas` and renders the list page
- Old `/functions` route no longer works
