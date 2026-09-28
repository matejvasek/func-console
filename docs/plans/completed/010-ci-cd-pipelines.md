# CI/CD Pipelines Implementation Plan

> **REQUIRED SUB-SKILL:** Use the executing-plans skill to implement this plan task-by-task.

**Goal:** Single GitHub Actions workflow that runs lint + test on PRs and additionally builds + publishes a container image to GHCR on master merges.

**Architecture:** One workflow file following knative/func patterns (single file, dual trigger, conditional publish job). ESLint config fixed (redundant rule spreads removed, Jest globals added). README deployment section rewritten with concrete commands.

**Tech Stack:** GitHub Actions, docker/build-push-action, actions/setup-node@v4 (Node 22, corepack, yarn cache), ESLint flat config

---

### Task 1: Fix ESLint config — remove redundant rule spreads and add Jest globals

All 194 lint errors are ESLint config issues, not code issues:
- 185 `no-undef` errors: Jest globals (`jest`, `describe`, `it`, `expect`, etc.) not declared for test files.
- 9 `no-unused-vars` false positives: the base ESLint `no-unused-vars` rule flags TypeScript interface parameter names (e.g., `onSubmit: (data: FormData) => void`). The `@typescript-eslint/no-unused-vars` rule handles these correctly and is already enabled by `tseslint.configs.recommended` at the top level. But the `src/**` block re-spreads `...eslint.configs.recommended.rules`, which re-enables the base rule. The `...tseslint.configs.recommended.rules` spread is a no-op because `tseslint.configs.recommended` is an array, not an object, so `.rules` is `undefined`.

**Files:**
- Modify: `eslint.config.mjs`

**Step 1: Remove redundant rule spreads from the `src/**` block**

In the `src/**` config block, remove `...eslint.configs.recommended.rules` and `...tseslint.configs.recommended.rules` from the `rules` object. Keep only the react rules:

```javascript
    rules: {
      ...react.configs.recommended.rules,
      ...react.configs['jsx-runtime'].rules,
    },
```

**Step 2: Add Jest globals for test files**

Add a new config block after the `src/**` block and before the `integration-tests/**` block:

```javascript
  {
    files: ['src/**/*.test.{ts,tsx}'],
    languageOptions: {
      globals: {
        jest: 'readonly',
        describe: 'readonly',
        it: 'readonly',
        expect: 'readonly',
        beforeEach: 'readonly',
        afterEach: 'readonly',
        beforeAll: 'readonly',
        afterAll: 'readonly',
      },
    },
  },
```

**Step 3: Run lint to verify zero errors**

Run: `yarn lint 2>&1 | tail -5`
Expected: No errors (exit 0)

**Step 4: Run tests to verify nothing broke**

Run: `yarn test`
Expected: 8 suites, 34 tests, all passing

**Step 5: Commit**

```bash
git add eslint.config.mjs
git commit -m "fix: remove redundant ESLint rule spreads and add Jest globals"
```

---

### Task 2: Downgrade @console/pluginAPI to ^4.19.0

**Files:**
- Modify: `package.json:98` — change `"@console/pluginAPI": "^4.21.0"` to `"@console/pluginAPI": "^4.19.0"`

**Step 1: Update package.json**

Change the moduleFederation shared dependency:

```json
"@console/pluginAPI": "^4.19.0"
```

**Step 2: Run tests to verify nothing broke**

Run: `yarn test`
Expected: 8 suites, 34 tests, all passing

**Step 3: Commit**

```bash
git add package.json
git commit -m "fix: downgrade @console/pluginAPI shared dep to ^4.19.0"
```

---

### Task 3: Create the GitHub Actions workflow file

**Files:**
- Create: `.github/workflows/ci.yml`

**Step 1: Create the workflow file**

Follow the knative/func patterns: top-level permissions, concurrency group, global env for version pins, section comments separating job groups, timeout-minutes on every job.

```yaml
name: CI

permissions:
  contents: read
  packages: write

on:
  push:
    branches:
      - master
  pull_request:
    branches:
      - master

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: ${{ github.event_name == 'pull_request' }}

env:
  NODE_VERSION: "22"

jobs:
  # --------
  # CHECKS
  # --------
  checks:
    name: Lint and Test
    runs-on: ubuntu-latest
    timeout-minutes: 15
    steps:
      - uses: actions/checkout@v4

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: ${{ env.NODE_VERSION }}
          cache: yarn

      - name: Enable Corepack
        run: corepack enable

      - name: Install Dependencies
        run: yarn install --immutable

      - name: Lint
        run: yarn lint

      - name: Test
        run: yarn test

  # -----------------
  # BUILD AND PUBLISH
  # -----------------
  build-and-publish:
    name: Build and Publish
    needs: checks
    if: github.event_name == 'push' && github.ref == 'refs/heads/master'
    runs-on: ubuntu-latest
    timeout-minutes: 15
    steps:
      - uses: actions/checkout@v4

      - name: Login to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and Push Image
        uses: docker/build-push-action@v6
        with:
          context: .
          push: true
          tags: |
            ghcr.io/twogiants/console-functions-plugin:latest
            ghcr.io/twogiants/console-functions-plugin:sha-${{ github.sha }}
```

**Step 2: Validate YAML syntax**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))" && echo "Valid YAML"`
Expected: `Valid YAML`

**Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add GitHub Actions workflow for PR checks and master publish"
```

---

### Task 4: Update README.md deployment section

**Files:**
- Modify: `README.md:96-117`

**Step 1: Replace lines 96-117 with concrete deployment instructions**

Replace the entire "Deployment on cluster" section (from `## Deployment on cluster` through the two NOTE paragraphs) with:

```markdown
## Deployment on cluster

```shell
oc new-project console-functions-plugin
helm upgrade -i console-functions-plugin charts/openshift-console-plugin \
    -n console-functions-plugin --create-namespace \
    --set "plugin.image=ghcr.io/twogiants/console-functions-plugin:latest@sha256:<digest>"
```

Consult the chart [values](charts/openshift-console-plugin/values.yaml) file for additional parameters.

```

**Step 2: Verify the README renders correctly**

Visually inspect the changed section in the file.

**Step 3: Commit**

```bash
git add README.md
git commit -m "docs: update deployment section with concrete GHCR instructions"
```

---

### Task 5: Final verification

**Step 1: Run full lint + test suite**

Run: `yarn lint && yarn test`
Expected: Zero lint errors, 8 suites, 34 tests, all passing

**Step 2: Review all changes**

Run: `git log --oneline master..HEAD`
Verify 4 commits covering: ESLint config fix, pluginAPI downgrade, workflow file, README update.
