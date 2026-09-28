# Function Edit Page Implementation Plan

**Status:** COMPLETED

**Goal:** Build the Function Edit Page with a TreeView file browser and CodeEditor for editing and deploying function code to GitHub.

**Architecture:** The page follows the project's layered architecture. A `useFunctionEditPage` hook (co-located in the view file, unexported) handles data loading, state management, and save logic. A `FileTreeView` component with a co-located `useFileTreeView` hook renders the file tree. An `EditToolbar` component with `useEditToolbar` hook manages save flow and navigation. The `SourceControlService` interface is extended with `fetch()` and `updateRepo()`.

**Tech Stack:** React, PatternFly 6 (TreeView, Toolbar, Modal, Button, Alert, EmptyState), OCP Dynamic Plugin SDK (CodeEditor, DocumentTitle, ListPageHeader), Vitest + React Testing Library + MSW

**Testing approach:** MSW for all GitHub API interactions. `vi.mock` only for `react-i18next` and OCP SDK components.

---

## Completed Tasks

### Task 1: Add fetch() to SourceControlService and GithubService

- Added `fetch(repo)` using `git.getTree({ recursive: '1' })` + per-file `git.getBlob`
- Migrated all GithubService tests from `vi.mock('@octokit/rest')` to MSW handlers
- Renamed test file to `GithubService.test.ts`
- Merged `RepoInfo` + `SourceRepo` into single `RepoMetadata` type
- Added test for `fetchFileContent` error path

### Task 2: Decompose push() into push() and updateRepo()

- `push()` for initial commits (createRef, no parents)
- `updateRepo()` for subsequent commits with local commit SHA cache
- Cache handles GitHub's eventually consistent ref storage (stale getRef on rapid saves)
- Cache clears only on "not a fast forward" errors (external push), not on network errors
- 5 updateRepo tests: first push, second from cache, third from cache, stale cache recovery, network error preserves cache

### Task 3: Add getLanguageFromPath utility

- Maps file extensions and special filenames (Dockerfile, Makefile) to Monaco Language enum
- Extracted to shared `src/utils/utils.ts` along with `parseNamespaceAndRuntime` and `handlerMap`

### Tasks 4+5: FileTreeView component

- Builds `TreeViewDataItem[]` directly from flat `FileEntry[]` paths (no intermediate type)
- Separate handling for root files and nested files
- Directories render before files, sorted alphabetically
- Loading state with spinner and "Loading source..." text
- Empty state with "No files" placeholder (not clickable)
- Dirty indicator after filename (`func.yaml ●`)
- `React.memo` with `useMemo` (justified: parent re-renders on every CodeEditor keystroke)
- Fixed width (16rem) with horizontal scroll, vertical scroll for long file lists
- 11 test cases using realistic Node function file structure from knative/func templates

### Task 6: FunctionEditPage view and hooks

- Page component: toolbar + flex layout (FileTreeView left, SDK CodeEditor right)
- `Flex` with `direction: row`, `flexWrap: nowrap`, `alignItems: stretch` (fixes baseline alignment stacking issue)
- SDK CodeEditor with `height="70vh"`, empty state (code icon + "Start editing"), language label visible
- Handler file auto-selected based on runtime from func.yaml (function.go for Go, index.js for Node, function/func.py for Python)
- Each page loads its own data from URL params (no navigation state passing between pages)
- `resolveRepoContent` extracts API calls from state management
- Repo metadata stored in state after load for save without re-fetching

### EditToolbar component

- Back link (left), success/danger Alert (center), Save & Deploy button (right)
- `useEditToolbar` hook: save flow with isSaving/error/success state
- Success alert "Pushed to GitHub. Deployment running..." with 2-second auto-dismiss (tested with `vi.useFakeTimers`)
- Unsaved changes confirmation modal (LeaveModal) on back navigation when dirty
- Save & Deploy disabled when no changes

### Infrastructure changes

- Migrated from Jest to Vitest with MSW 2.x
- Custom jsdom environment removed (Vitest handles globals natively)
- CSP connect-src for GitHub API added to start-console.sh for dev mode
- Added CSS, co-location, React performance rules to style guide and architecture docs
- Added communication rules and draft PR step to workflow docs

---

## Deviations from original plan

- Used SDK CodeEditor instead of PatternFly CodeEditor (CSP blob URL issues with Monaco workers in OCP Console)
- Removed navigation state passing between list and edit page (each page is self-contained)
- Split `push()` into `push()` + `updateRepo()` (clearer separation of initial vs subsequent commits)
- Added local commit SHA cache (GitHub eventual consistency workaround)
- Added success alert after save (not in original plan)
- Editor empty state with code icon (not in original plan)
- `autoSelectHandler` renamed to `determineHandler` (pure function, returns path instead of calling setState)
