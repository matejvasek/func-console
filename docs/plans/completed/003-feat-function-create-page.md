# Function Create Page — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render a Function Create Page at `/functions/create` with a PatternFly 6 form that generates a Knative function skeleton (including CI workflow) via a Go backend and pushes the resulting files to a GitHub repository as an initial commit.

**Architecture:** A `FunctionCreatePage` view orchestrates two services: `FunctionService` (calls the Go backend via `consoleFetchJSON` through the OCP console proxy) and `GitHubService` (pushes files via Octokit Git Data API). The form collects GitHub settings (owner, repo, branch, PAT) and function settings (name, language, registry, namespace). On submit, the page generates files, pushes them, and navigates to `/functions`.

**Tech Stack:** React 17, TypeScript, PatternFly 6, OCP Dynamic Plugin SDK, Go 1.24, `knative.dev/func`, Jest + React Testing Library

---

## File Structure

| Action | Path | Responsibility |
|--------|------|----------------|
| Modify | `backend/go.mod` | Add `knative.dev/func` and `github.com/ory/viper` dependencies |
| Modify | `backend/main.go` | `POST /api/function/create` endpoint with input validation |
| Create | `src/services/types.ts` | Shared types: `FileEntry`, `FunctionConfig`, `RepoInfo` |
| Create | `src/services/function/FunctionService.ts` | FunctionService interface |
| Create | `src/services/function/BackendFunctionService.ts` | Implementation using `consoleFetchJSON` |
| Create | `src/services/function/useFunctionService.ts` | Hook returning singleton |
| Create | `src/services/function/FunctionService.test.ts` | Unit tests |
| Create | `src/services/github/GitHubService.ts` | GitHubService interface |
| Create | `src/services/github/OctokitGitHubService.ts` | Implementation using Octokit Git Data API |
| Create | `src/services/github/useGitHubService.ts` | Hook returning singleton |
| Create | `src/services/github/GitHubService.test.ts` | Unit tests |
| Create | `src/components/CreateFunctionForm.tsx` | PF6 form component |
| Create | `src/components/CreateFunctionForm.test.tsx` | Component tests |
| Create | `src/views/FunctionCreatePage.tsx` | Page view orchestrating services |
| Create | `src/views/FunctionCreatePage.test.tsx` | View tests |
| Modify | `console-extensions.json` | Add `/functions/create` route |
| Modify | `package.json` | Add `FunctionCreatePage` to `exposedModules`, add `@octokit/rest` |
| Modify | `locales/en/plugin__console-functions-plugin.json` | i18n keys for form labels |
| Modify | `init.sh` | Add `start_backend`/`stop_backend` functions |
| Modify | `start-console.sh` | Add `BRIDGE_PLUGIN_PROXY` for console proxy |
| Modify | `charts/openshift-console-plugin/templates/consoleplugin.yaml` | Add proxy section for in-cluster |

---

### Task 1: Go Backend Endpoint

**Files:** `backend/go.mod`, `backend/main.go`

- [x] **Step 1:** Add `knative.dev/func v0.48.3` and `github.com/ory/viper v1.7.5` to `go.mod`
- [x] **Step 2:** Add `funcCreateRequest` struct with `Name`, `Runtime`, `Registry`, `Namespace`, `Branch` fields
- [x] **Step 3:** Add `handleFuncCreate` handler — validates all inputs (name, runtime whitelist, branch, namespace), limits request body to 1 MB, creates temp dir, calls `functions.New().Init()`, walks generated files, returns JSON array of `fileEntry`
- [x] **Step 4:** Add `generateCIWorkflow` function using `knative.dev/func/cmd/ci` with viper globals (serialized via `sync.Mutex`)
- [x] **Step 5:** Register `POST /api/function/create` route
- [x] **Step 6:** Run `go mod tidy` and verify build

---

### Task 2: Dev Environment Setup

**Files:** `init.sh`, `start-console.sh`, `charts/.../consoleplugin.yaml`

- [x] **Step 1:** Add `start_backend`/`stop_backend` functions to `init.sh` with `-buildvcs=false` and `--http-port`
- [x] **Step 2:** Add `BRIDGE_PLUGIN_PROXY` to `start-console.sh` routing through `/api/proxy/plugin/console-functions-plugin/backend/`
- [x] **Step 3:** Add proxy section to `consoleplugin.yaml` for in-cluster deployment

---

### Task 3: Shared Types

**Files:** `src/services/types.ts`

- [x] **Step 1:** Create `FileEntry` interface with `path`, `mode` (`'100644' | '100755' | '120000'`), `content`, `type` (`'blob'`) — mirrors Octokit git tree entry structure
- [x] **Step 2:** Create `FunctionConfig` interface with `name`, `runtime`, `registry`, `namespace`, `branch`
- [x] **Step 3:** Create `FunctionRuntime` type (`'node' | 'python' | 'go' | 'quarkus'`)
- [x] **Step 4:** Create `RepoInfo` interface with `owner`, `repo`, `branch`

---

### Task 4: FunctionService (TDD)

**Files:** `src/services/function/FunctionService.ts`, `BackendFunctionService.ts`, `useFunctionService.ts`, `FunctionService.test.ts`

- [x] **Step 1:** Create `FunctionService` interface with `generateFunction(config): Promise<FileEntry[]>`
- [x] **Step 2:** Write tests mocking `@openshift-console/dynamic-plugin-sdk` with `consoleFetchJSON: { post: jest.fn() }`
- [x] **Step 3:** Implement `BackendFunctionService` using `consoleFetchJSON.post()` through `/api/proxy/plugin/console-functions-plugin/backend/api/function/create`
- [x] **Step 4:** Create `useFunctionService` hook returning module-level singleton
- [x] **Step 5:** Verify tests pass

---

### Task 5: GitHubService (TDD)

**Files:** `src/services/github/GitHubService.ts`, `OctokitGitHubService.ts`, `useGitHubService.ts`, `GitHubService.test.ts`

- [x] **Step 1:** Create `GitHubService` interface with `pushFiles(repo, pat, files, message): Promise<void>`
- [x] **Step 2:** Write tests mocking Octokit — verify initial commit flow (createBlob -> createTree -> createCommit with empty parents -> createRef)
- [x] **Step 3:** Implement `OctokitGitHubService` — creates blobs in parallel, builds tree without `base_tree`, creates commit with `parents: []`, creates branch ref
- [x] **Step 4:** Create `useGitHubService` hook returning module-level singleton
- [x] **Step 5:** Verify tests pass

---

### Task 6: CreateFunctionForm Component (TDD)

**Files:** `src/components/CreateFunctionForm.tsx`, `CreateFunctionForm.test.tsx`

- [x] **Step 1:** Write tests — renders all fields (owner, repo, branch, PAT, name, language, registry, namespace), calls `onSubmit` with form data, calls `onCancel`
- [x] **Step 2:** Implement PF6 form with two `FormSection`s (GitHub Settings, Function Settings), PAT as `type="password"`, language as `FormSelect`, `onSubmit` handler with `preventDefault`, Create button as `type="submit"`
- [x] **Step 3:** Use regex matchers in tests for PF6 `isRequired` asterisk handling
- [x] **Step 4:** Verify tests pass

---

### Task 7: FunctionCreatePage View (TDD)

**Files:** `src/views/FunctionCreatePage.tsx`, `FunctionCreatePage.test.tsx`

- [x] **Step 1:** Write tests — renders form, success flow (generate -> push -> navigate), error flow (shows Alert)
- [x] **Step 2:** Implement page orchestrating `useFunctionService` + `useGitHubService`, using `useNavigate` from `react-router-dom-v5-compat`
- [x] **Step 3:** Verify tests pass

---

### Task 8: Configuration Wiring

**Files:** `console-extensions.json`, `package.json`, `locales/en/plugin__console-functions-plugin.json`

- [x] **Step 1:** Add `/functions/create` route to `console-extensions.json` pointing to `FunctionCreatePage` with `exact: true`
- [x] **Step 2:** Add `"FunctionCreatePage": "./views/FunctionCreatePage"` to `exposedModules` in `package.json`
- [x] **Step 3:** Add `@octokit/rest` to `devDependencies`
- [x] **Step 4:** Add i18n keys for all form labels and UI text

---

### Task 9: Security Hardening

- [x] **Step 1:** Add path traversal protection — validate `cfg.Name` with `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$` regex in Go backend before `filepath.Join`
- [x] **Step 2:** Add input validation for `runtime` (whitelist), `branch` (git-ref regex), `namespace` (k8s name regex)
- [x] **Step 3:** Add request body size limit (`http.MaxBytesReader`, 1 MB)
- [x] **Step 4:** Handle `json.Encode` error on response write

---

### Task 10: Review and Manual Test

- [x] **Step 1:** Run security review
- [x] **Step 2:** Run code review — fix all critical/important issues
- [x] **Step 3:** Manual browser test — fill form, submit, verify function skeleton pushed to GitHub
- [ ] **Step 4:** Flip `passes` to `true` in `docs/features.json`, update `docs/claude-progress.txt`, commit
