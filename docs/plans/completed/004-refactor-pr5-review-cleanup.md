# PR #5 Review Cleanup Implementation Plan

> **REQUIRED SUB-SKILL:** Use the executing-plans skill to implement this plan task-by-task.

**Goal:** Consolidate duplicate GitHub services, fix naming conventions, and extract hooks per architecture guidelines — all from PR #5 review feedback.

**Architecture:** Merge `src/services/github/` into `src/services/source-control/`, combining the `pushFiles` method from `OctokitGitHubService` into the existing `GithubService` class under the unified `SourceControlService` interface. Rename files and hooks to match conventions. Extract page/form logic into co-named hooks per architecture rules.

**Tech Stack:** TypeScript, React, Jest, PatternFly 6, Octokit

---

### Task 1: Remove duplicate `@octokit/rest` from `devDependencies`

**Files:**

- Modify: `package.json`

**Step 1: Remove the duplicate entry**

In `package.json`, `@octokit/rest` appears in both `devDependencies` and `dependencies`. Remove it from `devDependencies`, keeping only the `dependencies` entry.

Before:

```json
"devDependencies": {
    ...
    "@octokit/rest": "^22.0.1",
    ...
}
...
"dependencies": {
    "@octokit/rest": "^22.0.1"
}
```

After:

```json
"devDependencies": {
    ...
    // @octokit/rest removed from here
    ...
}
...
"dependencies": {
    "@octokit/rest": "^22.0.1"
}
```

**Step 2: Run `yarn install` to update lockfile**

Run: `yarn install`

**Step 3: Run tests to verify nothing broke**

Run: `yarn test`
Expected: All 34 tests pass (9 suites).

**Step 4: Commit**

```bash
git add package.json yarn.lock
git commit -m "chore: remove duplicate @octokit/rest from devDependencies"
```

---

### Task 2: Merge `src/services/github/` push logic into `src/services/source-control/`

This is the core consolidation. The `push` method from `OctokitGitHubService` moves into `GithubService`, the `SourceControlService` interface gains a `push` method, and `src/services/github/` is deleted entirely.

**Files:**

- Modify: `src/services/source-control/SourceControlService.ts`
- Modify: `src/services/source-control/GithubService.ts`
- Modify: `src/services/source-control/SourceControlService.test.ts`
- Modify: `src/services/source-control/useSourceControl.ts`
- Delete: `src/services/github/GitHubService.ts`
- Delete: `src/services/github/OctokitGitHubService.ts`
- Delete: `src/services/github/useGitHubService.ts`
- Delete: `src/services/github/GitHubService.test.ts`

**Step 1: Write the failing test for push**

Add a `push` test to `src/services/source-control/SourceControlService.test.ts`. The test should verify that `GithubService.push()` creates blobs, tree, commit, and ref via Octokit — same behavior as the existing `OctokitGitHubService.pushFiles` test.

Add these mocks to the existing `jest.mock('@octokit/rest')` block in `SourceControlService.test.ts`:

```typescript
const mockCreateBlob = jest.fn();
const mockCreateTree = jest.fn();
const mockCreateCommit = jest.fn();
const mockCreateRef = jest.fn();
```

Update the mock Octokit constructor to include the `git` property alongside the existing `users`, `search`, `repos`:

```typescript
git: {
  createBlob: mockCreateBlob,
  createTree: mockCreateTree,
  createCommit: mockCreateCommit,
  createRef: mockCreateRef,
},
```

Add a new `describe('push')` block:

```typescript
describe('push', () => {
  const repoInfo: RepoInfo = { owner: 'twoGiants', repo: 'my-func', branch: 'main' };
  const files: FileEntry[] = [
    { path: 'func.yaml', mode: '100644', content: 'name: my-func', type: 'blob' },
  ];

  beforeEach(() => {
    mockCreateBlob.mockResolvedValue({ data: { sha: 'blob-sha-123' } });
    mockCreateTree.mockResolvedValue({ data: { sha: 'tree-sha-123' } });
    mockCreateCommit.mockResolvedValue({ data: { sha: 'commit-sha-123' } });
    mockCreateRef.mockResolvedValue({});
  });

  it('creates an initial commit with the provided files', async () => {
    const svc = new GithubService('fake-token');
    await svc.push(repoInfo, files, 'Initialize function');

    expect(mockCreateBlob).toHaveBeenCalledWith({
      owner: 'twoGiants',
      repo: 'my-func',
      content: 'name: my-func',
      encoding: 'utf-8',
    });
    expect(mockCreateTree).toHaveBeenCalledWith({
      owner: 'twoGiants',
      repo: 'my-func',
      tree: [{ path: 'func.yaml', mode: '100644', type: 'blob', sha: 'blob-sha-123' }],
    });
    expect(mockCreateCommit).toHaveBeenCalledWith({
      owner: 'twoGiants',
      repo: 'my-func',
      message: 'Initialize function',
      tree: 'tree-sha-123',
      parents: [],
    });
    expect(mockCreateRef).toHaveBeenCalledWith({
      owner: 'twoGiants',
      repo: 'my-func',
      ref: 'refs/heads/main',
      sha: 'commit-sha-123',
    });
  });

  it('propagates errors from intermediate API calls', async () => {
    mockCreateTree.mockRejectedValue(new Error('Validation Failed'));
    const svc = new GithubService('fake-token');

    await expect(svc.push(repoInfo, files, 'Initialize function')).rejects.toThrow(
      'Validation Failed',
    );
  });
});
```

Import `FileEntry` and `RepoInfo` from `../types` at the top of the test file (add to existing import).

**Step 2: Run test to verify it fails**

Run: `yarn test -- --testPathPattern=source-control`
Expected: FAIL — `svc.push is not a function`

**Step 3: Add `push` to the `SourceControlService` interface**

In `src/services/source-control/SourceControlService.ts`:

```typescript
import { FileEntry, RepoInfo, SourceRepo } from '../types';

export interface SourceControlService {
  listFunctionRepos(): Promise<SourceRepo[]>;
  fetchFileContent(repo: SourceRepo, path: string): Promise<string>;
  push(repo: RepoInfo, files: FileEntry[], message: string): Promise<void>;
}
```

**Step 4: Implement `push` in `GithubService`**

In `src/services/source-control/GithubService.ts`, add the import for `FileEntry` and `RepoInfo`, then add the `push` method:

```typescript
import { FileEntry, RepoInfo, SourceRepo } from '../types';
```

Add this method to the `GithubService` class:

```typescript
async push(repo: RepoInfo, files: FileEntry[], message: string): Promise<void> {
  const { owner, repo: repoName, branch } = repo;

  const treeEntries = await Promise.all(
    files.map(async (file) => {
      const { data: blob } = await this.octokit.git.createBlob({
        owner,
        repo: repoName,
        content: file.content,
        encoding: 'utf-8',
      });
      return {
        path: file.path,
        mode: file.mode,
        type: file.type as 'blob',
        sha: blob.sha,
      };
    }),
  );

  const { data: tree } = await this.octokit.git.createTree({
    owner,
    repo: repoName,
    tree: treeEntries,
  });

  const { data: commit } = await this.octokit.git.createCommit({
    owner,
    repo: repoName,
    message,
    tree: tree.sha,
    parents: [],
  });

  await this.octokit.git.createRef({
    owner,
    repo: repoName,
    ref: `refs/heads/${branch}`,
    sha: commit.sha,
  });
}
```

Note: This uses `this.octokit` (already initialized in the constructor) instead of creating a new Octokit instance per call like `OctokitGitHubService` did.

**Step 5: Run test to verify it passes**

Run: `yarn test -- --testPathPattern=source-control`
Expected: PASS — all tests in `SourceControlService.test.ts` pass.

**Step 6: Commit**

```bash
git add src/services/source-control/
git commit -m "feat: add push method to SourceControlService interface and GithubService"
```

---

### Task 3: Update `FunctionCreatePage` to use `useSourceControl` instead of `useGitHubService`

**Files:**

- Modify: `src/views/FunctionCreatePage.tsx`
- Modify: `src/views/FunctionCreatePage.test.tsx`

**Step 1: Update the test mock**

In `src/views/FunctionCreatePage.test.tsx`:

Replace:

```typescript
jest.mock('../services/github/useGitHubService', () => ({
  useGitHubService: () => ({ pushFiles: mockPushFiles }),
}));
```

With:

```typescript
jest.mock('../services/source-control/useSourceControl', () => ({
  useSourceControl: () => ({
    push: mockPushFiles,
    listFunctionRepos: jest.fn(),
    fetchFileContent: jest.fn(),
  }),
}));
```

Update the test assertion — change `mockPushFiles` call expectation. Replace the `pushFiles` assertion:

```typescript
expect(mockPushFiles).toHaveBeenCalledWith(
  { owner: 'testuser', repo: 'my-repo', branch: 'main' },
  'ghp_token',
  files,
  'Initialize Knative function project',
);
```

With (PAT is no longer passed — the service is already authenticated):

```typescript
expect(mockPushFiles).toHaveBeenCalledWith(
  { owner: 'testuser', repo: 'my-repo', branch: 'main' },
  files,
  'Initialize Knative function project',
);
```

**Step 2: Run test to verify it fails**

Run: `yarn test -- --testPathPattern=FunctionCreatePage`
Expected: FAIL — still importing `useGitHubService`.

**Step 3: Update the component**

In `src/views/FunctionCreatePage.tsx`:

Replace:

```typescript
import { useGitHubService } from '../services/github/useGitHubService';
```

With:

```typescript
import { useSourceControl } from '../services/source-control/useSourceControl';
```

Replace:

```typescript
const gitHubService = useGitHubService();
```

With:

```typescript
const sourceControl = useSourceControl();
```

Replace the `pushFiles` call:

```typescript
await gitHubService.pushFiles(
  { owner: data.owner, repo: data.repo, branch: data.branch },
  data.pat,
  files,
  'Initialize Knative function project',
);
```

With:

```typescript
await sourceControl.push(
  { owner: data.owner, repo: data.repo, branch: data.branch },
  files,
  'Initialize Knative function project',
);
```

**Step 4: Run test to verify it passes**

Run: `yarn test -- --testPathPattern=FunctionCreatePage`
Expected: PASS

**Step 5: Run all tests**

Run: `yarn test`
Expected: All 34 tests pass.

**Step 6: Commit**

```bash
git add src/views/FunctionCreatePage.tsx src/views/FunctionCreatePage.test.tsx
git commit -m "refactor: use useSourceControl instead of useGitHubService in FunctionCreatePage"
```

---

### Task 4: Delete `src/services/github/` directory

Now that nothing references it anymore, delete the entire directory.

**Files:**

- Delete: `src/services/github/GitHubService.ts`
- Delete: `src/services/github/OctokitGitHubService.ts`
- Delete: `src/services/github/useGitHubService.ts`
- Delete: `src/services/github/GitHubService.test.ts`

**Step 1: Verify no remaining references**

Run: `grep -rn "services/github" src/`
Expected: No output (no files reference the github directory).

**Step 2: Delete the directory**

Run: `rm -rf src/services/github/`

**Step 3: Run all tests**

Run: `yarn test`
Expected: All tests pass (now 8 suites, 32 tests — two tests from `GitHubService.test.ts` removed).

**Step 4: Commit**

```bash
git add -A src/services/github/
git commit -m "refactor: remove duplicate src/services/github/ directory"
```

---

### Task 5: Rename `BackendFunctionService.ts` to `FunctionBackendService.ts`

**Files:**

- Rename: `src/services/function/BackendFunctionService.ts` → `src/services/function/FunctionBackendService.ts`
- Modify: `src/services/function/useFunctionService.ts`
- Modify: `src/services/function/FunctionService.test.ts`

**Step 1: Rename the file**

Run: `mv src/services/function/BackendFunctionService.ts src/services/function/FunctionBackendService.ts`

**Step 2: Update the class name inside the file**

In `src/services/function/FunctionBackendService.ts`, rename the class:

Replace: `export class BackendFunctionService`
With: `export class FunctionBackendService`

**Step 3: Update the hook import**

In `src/services/function/useFunctionService.ts`:

Replace:

```typescript
import { BackendFunctionService } from './BackendFunctionService';
```

With:

```typescript
import { FunctionBackendService } from './FunctionBackendService';
```

Replace:

```typescript
const instance = new BackendFunctionService();
```

With:

```typescript
const instance = new FunctionBackendService();
```

**Step 4: Update the test import**

In `src/services/function/FunctionService.test.ts`:

Replace:

```typescript
import { BackendFunctionService } from './BackendFunctionService';
```

With:

```typescript
import { FunctionBackendService } from './FunctionBackendService';
```

Replace all occurrences of `new BackendFunctionService()` with `new FunctionBackendService()` in the test file (2 occurrences — in each `it` block).

**Step 5: Run tests**

Run: `yarn test`
Expected: All tests pass.

**Step 6: Commit**

```bash
git add src/services/function/
git commit -m "refactor: rename BackendFunctionService to FunctionBackendService"
```

---

### Task 6: Rename `useSourceControl` hook to `useSourceControlService`

Per convention, hooks wrapping services should be named `use<ServiceName>`: `SourceControlService` → `useSourceControlService`.

**Files:**

- Rename: `src/services/source-control/useSourceControl.ts` → `src/services/source-control/useSourceControlService.ts`
- Modify: `src/views/FunctionsListPage.tsx`
- Modify: `src/views/FunctionCreatePage.tsx`
- Modify: `src/views/FunctionCreatePage.test.tsx`
- Modify: `src/views/FunctionsListPage.test.tsx`

**Step 1: Rename the file**

Run: `mv src/services/source-control/useSourceControl.ts src/services/source-control/useSourceControlService.ts`

**Step 2: Rename the exported function**

In `src/services/source-control/useSourceControlService.ts`:

Replace: `export function useSourceControl()`
With: `export function useSourceControlService()`

**Step 3: Update imports in FunctionsListPage**

In `src/views/FunctionsListPage.tsx`:

Replace: `import { useSourceControl } from '../services/source-control/useSourceControl';`
With: `import { useSourceControlService } from '../services/source-control/useSourceControlService';`

Replace: `const sourceControl = useSourceControl();`
With: `const sourceControl = useSourceControlService();`

**Step 4: Update imports in FunctionCreatePage**

In `src/views/FunctionCreatePage.tsx`:

Replace: `import { useSourceControl } from '../services/source-control/useSourceControl';`
With: `import { useSourceControlService } from '../services/source-control/useSourceControlService';`

Replace: `const sourceControl = useSourceControl();`
With: `const sourceControl = useSourceControlService();`

**Step 5: Update test mocks**

In `src/views/FunctionCreatePage.test.tsx`:

Replace: `jest.mock('../services/source-control/useSourceControl', () => ({`
With: `jest.mock('../services/source-control/useSourceControlService', () => ({`

Replace: `useSourceControl: () => ({`
With: `useSourceControlService: () => ({`

In `src/views/FunctionsListPage.test.tsx`:

Replace: `'../services/source-control/useSourceControl'`
With: `'../services/source-control/useSourceControlService'`

Replace: `useSourceControl:`
With: `useSourceControlService:`

**Step 6: Run tests**

Run: `yarn test`
Expected: All tests pass.

**Step 7: Commit**

```bash
git add src/services/source-control/ src/views/
git commit -m "refactor: rename useSourceControl to useSourceControlService"
```

---

### Task 7: Extract `useCreateFunctionForm` hook from `CreateFunctionForm`

Consolidate individual `useState` calls into a single `fields` state object with a `setField(key, value)` helper. Derive `isValid` from `fields`.

**Files:**

- Create: `src/components/useCreateFunctionForm.ts`
- Modify: `src/components/CreateFunctionForm.tsx`

**Step 1: Write the failing test for the hook**

Create `src/components/useCreateFunctionForm.test.ts`:

```typescript
import { renderHook, act } from '@testing-library/react';
import { useCreateFunctionForm } from './useCreateFunctionForm';

describe('useCreateFunctionForm', () => {
  it('initializes with empty fields and invalid state', () => {
    const { result } = renderHook(() => useCreateFunctionForm());

    expect(result.current.fields.name).toBe('');
    expect(result.current.fields.owner).toBe('');
    expect(result.current.fields.runtime).toBe('node');
    expect(result.current.isValid).toBe(false);
  });

  it('updates a single field via setField', () => {
    const { result } = renderHook(() => useCreateFunctionForm());

    act(() => {
      result.current.setField('name', 'my-func');
    });

    expect(result.current.fields.name).toBe('my-func');
  });

  it('is valid when all required fields are filled', () => {
    const { result } = renderHook(() => useCreateFunctionForm());

    act(() => {
      result.current.setField('owner', 'testuser');
      result.current.setField('repo', 'my-repo');
      result.current.setField('branch', 'main');
      result.current.setField('pat', 'ghp_token');
      result.current.setField('name', 'my-func');
      result.current.setField('registry', 'quay.io/test');
      result.current.setField('namespace', 'default');
    });

    expect(result.current.isValid).toBe(true);
  });
});
```

**Step 2: Run test to verify it fails**

Run: `yarn test -- --testPathPattern=useCreateFunctionForm`
Expected: FAIL — module not found.

**Step 3: Implement the hook**

Create `src/components/useCreateFunctionForm.ts`:

```typescript
import { useCallback, useMemo, useState } from 'react';
import { FunctionRuntime } from '../services/types';

export interface CreateFunctionFormFields {
  owner: string;
  repo: string;
  branch: string;
  pat: string;
  name: string;
  runtime: FunctionRuntime;
  registry: string;
  namespace: string;
}

const initialFields: CreateFunctionFormFields = {
  owner: '',
  repo: '',
  branch: '',
  pat: '',
  name: '',
  runtime: 'node',
  registry: '',
  namespace: '',
};

export function useCreateFunctionForm() {
  const [fields, setFields] = useState<CreateFunctionFormFields>(initialFields);

  const setField = useCallback(
    <K extends keyof CreateFunctionFormFields>(key: K, value: CreateFunctionFormFields[K]) => {
      setFields((prev) => ({ ...prev, [key]: value }));
    },
    [],
  );

  const isValid = useMemo(
    () =>
      Boolean(
        fields.owner &&
          fields.repo &&
          fields.branch &&
          fields.pat &&
          fields.name &&
          fields.registry &&
          fields.namespace,
      ),
    [fields],
  );

  return { fields, setField, isValid };
}
```

**Step 4: Run test to verify it passes**

Run: `yarn test -- --testPathPattern=useCreateFunctionForm`
Expected: PASS

**Step 5: Refactor `CreateFunctionForm` to use the hook**

In `src/components/CreateFunctionForm.tsx`:

Replace the entire file with:

```typescript
import {
  ActionGroup,
  Button,
  Form,
  FormGroup,
  FormSection,
  FormSelect,
  FormSelectOption,
  TextInput,
} from '@patternfly/react-core';
import { useTranslation } from 'react-i18next';
import { FunctionRuntime } from '../services/types';
import { CreateFunctionFormFields, useCreateFunctionForm } from './useCreateFunctionForm';

export type CreateFunctionFormData = CreateFunctionFormFields;

interface CreateFunctionFormProps {
  onSubmit: (data: CreateFunctionFormData) => void;
  onCancel: () => void;
  isSubmitting: boolean;
}

const runtimeOptions = [
  { value: 'node', label: 'Node.js' },
  { value: 'python', label: 'Python' },
  { value: 'go', label: 'Go' },
  { value: 'quarkus', label: 'Quarkus' },
];

export function CreateFunctionForm({ onSubmit, onCancel, isSubmitting }: CreateFunctionFormProps) {
  const { t } = useTranslation('plugin__console-functions-plugin');
  const { fields, setField, isValid } = useCreateFunctionForm();

  return (
    <Form onSubmit={(e) => { e.preventDefault(); onSubmit(fields); }}>
      <FormSection title={t('GitHub Settings')}>
        <FormGroup label={t('Owner')} isRequired fieldId="owner">
          <TextInput
            id="owner"
            isRequired
            value={fields.owner}
            onChange={(_e, val) => setField('owner', val)}
          />
        </FormGroup>
        <FormGroup label={t('Repository')} isRequired fieldId="repo">
          <TextInput
            id="repo"
            isRequired
            value={fields.repo}
            onChange={(_e, val) => setField('repo', val)}
          />
        </FormGroup>
        <FormGroup label={t('Branch')} isRequired fieldId="branch">
          <TextInput
            id="branch"
            isRequired
            value={fields.branch}
            onChange={(_e, val) => setField('branch', val)}
          />
        </FormGroup>
        <FormGroup label={t('Personal Access Token')} isRequired fieldId="pat">
          <TextInput
            id="pat"
            type="password"
            isRequired
            value={fields.pat}
            onChange={(_e, val) => setField('pat', val)}
          />
        </FormGroup>
      </FormSection>
      <FormSection title={t('Function Settings')}>
        <FormGroup label={t('Name')} isRequired fieldId="name">
          <TextInput
            id="name"
            isRequired
            value={fields.name}
            onChange={(_e, val) => setField('name', val)}
          />
        </FormGroup>
        <FormGroup label={t('Language')} isRequired fieldId="runtime">
          <FormSelect
            id="runtime"
            value={fields.runtime}
            onChange={(_e, val) => setField('runtime', val as FunctionRuntime)}
            aria-label={t('Language')}
          >
            {runtimeOptions.map(({ value, label }) => (
              <FormSelectOption key={value} value={value} label={label} />
            ))}
          </FormSelect>
        </FormGroup>
        <FormGroup label={t('Registry')} isRequired fieldId="registry">
          <TextInput
            id="registry"
            isRequired
            value={fields.registry}
            onChange={(_e, val) => setField('registry', val)}
          />
        </FormGroup>
        <FormGroup label={t('Namespace')} isRequired fieldId="namespace">
          <TextInput
            id="namespace"
            isRequired
            value={fields.namespace}
            onChange={(_e, val) => setField('namespace', val)}
          />
        </FormGroup>
      </FormSection>
      <ActionGroup>
        <Button
          type="submit"
          variant="primary"
          isDisabled={!isValid || isSubmitting}
          isLoading={isSubmitting}
        >
          {t('Create')}
        </Button>
        <Button variant="link" onClick={onCancel}>
          {t('Cancel')}
        </Button>
      </ActionGroup>
    </Form>
  );
}
```

**Step 6: Run all tests**

Run: `yarn test`
Expected: All tests pass (existing `CreateFunctionForm.test.tsx` tests still pass — same user-facing behavior).

**Step 7: Commit**

```bash
git add src/components/useCreateFunctionForm.ts src/components/useCreateFunctionForm.test.ts src/components/CreateFunctionForm.tsx
git commit -m "refactor: extract useCreateFunctionForm hook from CreateFunctionForm"
```

---

### Task 8: Extract `useFunctionCreatePage` hook from `FunctionCreatePage`

Move service calls and state management into a co-named hook. The component becomes rendering-only.

**Files:**

- Create: `src/views/useFunctionCreatePage.ts`
- Modify: `src/views/FunctionCreatePage.tsx`
- Modify: `src/views/FunctionCreatePage.test.tsx`

**Step 1: Write the failing test for the hook**

Create `src/views/useFunctionCreatePage.test.ts`:

```typescript
import { renderHook, act } from '@testing-library/react';
import { useFunctionCreatePage } from './useFunctionCreatePage';
import { CreateFunctionFormFields } from '../components/useCreateFunctionForm';

const mockGenerateFunction = jest.fn();
const mockPush = jest.fn();
const mockNavigate = jest.fn();

jest.mock('../services/function/useFunctionService', () => ({
  useFunctionService: () => ({ generateFunction: mockGenerateFunction }),
}));

jest.mock('../services/source-control/useSourceControlService', () => ({
  useSourceControlService: () => ({
    push: mockPush,
    listFunctionRepos: jest.fn(),
    fetchFileContent: jest.fn(),
  }),
}));

jest.mock('react-router-dom-v5-compat', () => ({
  useNavigate: () => mockNavigate,
}));

afterEach(() => {
  jest.clearAllMocks();
});

const formData: CreateFunctionFormFields = {
  owner: 'testuser',
  repo: 'my-repo',
  branch: 'main',
  pat: 'ghp_token',
  name: 'my-func',
  runtime: 'node',
  registry: 'quay.io/test',
  namespace: 'default',
};

describe('useFunctionCreatePage', () => {
  it('initializes with no error and not submitting', () => {
    const { result } = renderHook(() => useFunctionCreatePage());

    expect(result.current.isSubmitting).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('calls generateFunction then push on handleSubmit, and navigates on success', async () => {
    const files = [{ path: 'func.yaml', mode: '100644', content: 'name: f', type: 'blob' }];
    mockGenerateFunction.mockResolvedValue(files);
    mockPush.mockResolvedValue(undefined);

    const { result } = renderHook(() => useFunctionCreatePage());

    await act(async () => {
      await result.current.handleSubmit(formData);
    });

    expect(mockGenerateFunction).toHaveBeenCalledWith({
      name: 'my-func',
      runtime: 'node',
      registry: 'quay.io/test',
      namespace: 'default',
      branch: 'main',
    });
    expect(mockPush).toHaveBeenCalledWith(
      { owner: 'testuser', repo: 'my-repo', branch: 'main' },
      files,
      'Initialize Knative function project',
    );
    expect(mockNavigate).toHaveBeenCalledWith('/functions');
  });

  it('sets error on failure', async () => {
    mockGenerateFunction.mockRejectedValue(new Error('Backend error'));

    const { result } = renderHook(() => useFunctionCreatePage());

    await act(async () => {
      await result.current.handleSubmit(formData);
    });

    expect(result.current.error).toBe('Backend error');
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('navigates to /functions on handleCancel', () => {
    const { result } = renderHook(() => useFunctionCreatePage());

    act(() => {
      result.current.handleCancel();
    });

    expect(mockNavigate).toHaveBeenCalledWith('/functions');
  });
});
```

**Step 2: Run test to verify it fails**

Run: `yarn test -- --testPathPattern=useFunctionCreatePage`
Expected: FAIL — module not found.

**Step 3: Implement the hook**

Create `src/views/useFunctionCreatePage.ts`:

```typescript
import { useState } from 'react';
import { useNavigate } from 'react-router-dom-v5-compat';
import { CreateFunctionFormFields } from '../components/useCreateFunctionForm';
import { useFunctionService } from '../services/function/useFunctionService';
import { useSourceControlService } from '../services/source-control/useSourceControlService';

export function useFunctionCreatePage() {
  const navigate = useNavigate();
  const functionService = useFunctionService();
  const sourceControl = useSourceControlService();

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (data: CreateFunctionFormFields) => {
    setIsSubmitting(true);
    setError(null);

    try {
      const files = await functionService.generateFunction({
        name: data.name,
        runtime: data.runtime,
        registry: data.registry,
        namespace: data.namespace,
        branch: data.branch,
      });

      await sourceControl.push(
        { owner: data.owner, repo: data.repo, branch: data.branch },
        files,
        'Initialize Knative function project',
      );

      navigate('/functions');
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleCancel = () => {
    navigate('/functions');
  };

  return { isSubmitting, error, handleSubmit, handleCancel };
}
```

**Step 4: Run test to verify it passes**

Run: `yarn test -- --testPathPattern=useFunctionCreatePage`
Expected: PASS

**Step 5: Simplify `FunctionCreatePage` to use the hook**

Replace `src/views/FunctionCreatePage.tsx` with:

```typescript
import { DocumentTitle, ListPageHeader } from '@openshift-console/dynamic-plugin-sdk';
import { Alert, PageSection } from '@patternfly/react-core';
import { useTranslation } from 'react-i18next';
import { CreateFunctionForm } from '../components/CreateFunctionForm';
import { useFunctionCreatePage } from './useFunctionCreatePage';

export default function FunctionCreatePage() {
  const { t } = useTranslation('plugin__console-functions-plugin');
  const { isSubmitting, error, handleSubmit, handleCancel } = useFunctionCreatePage();

  return (
    <>
      <DocumentTitle>{t('Create function')}</DocumentTitle>
      <ListPageHeader title={t('Create function')} />
      <PageSection>
        {error && (
          <Alert variant="danger" title={t('Error creating function')} isInline>
            {error}
          </Alert>
        )}
        <CreateFunctionForm
          onSubmit={handleSubmit}
          onCancel={handleCancel}
          isSubmitting={isSubmitting}
        />
      </PageSection>
    </>
  );
}
```

**Step 6: Update `FunctionCreatePage.test.tsx`**

The page test now mocks the hook instead of individual services. Replace the entire test file with:

```typescript
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import FunctionCreatePage from './FunctionCreatePage';

const mockHandleSubmit = jest.fn();
const mockHandleCancel = jest.fn();

jest.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

jest.mock('@openshift-console/dynamic-plugin-sdk', () => ({
  DocumentTitle: ({ children }: { children: string }) => children,
  ListPageHeader: ({ title }: { title: string }) => title,
}));

jest.mock('./useFunctionCreatePage', () => ({
  useFunctionCreatePage: () => ({
    isSubmitting: false,
    error: null,
    handleSubmit: mockHandleSubmit,
    handleCancel: mockHandleCancel,
  }),
}));

afterEach(() => {
  jest.clearAllMocks();
});

describe('FunctionCreatePage', () => {
  it('renders CreateFunctionForm', () => {
    render(
      <MemoryRouter>
        <FunctionCreatePage />
      </MemoryRouter>,
    );

    expect(screen.getByRole('textbox', { name: /Owner/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Create/ })).toBeInTheDocument();
  });

  it('calls handleSubmit when form is submitted', async () => {
    const user = userEvent.setup();

    render(
      <MemoryRouter>
        <FunctionCreatePage />
      </MemoryRouter>,
    );

    await user.type(screen.getByRole('textbox', { name: /Owner/ }), 'testuser');
    await user.type(screen.getByRole('textbox', { name: /Repository/ }), 'my-repo');
    await user.type(screen.getByRole('textbox', { name: /Branch/ }), 'main');
    await user.type(screen.getByLabelText(/Personal Access Token/), 'ghp_token');
    await user.type(screen.getByRole('textbox', { name: /^Name$/ }), 'my-func');
    await user.type(screen.getByRole('textbox', { name: /Registry/ }), 'quay.io/test');
    await user.type(screen.getByRole('textbox', { name: /Namespace/ }), 'default');

    await user.click(screen.getByRole('button', { name: /Create/ }));

    await waitFor(() => {
      expect(mockHandleSubmit).toHaveBeenCalledWith({
        owner: 'testuser',
        repo: 'my-repo',
        branch: 'main',
        pat: 'ghp_token',
        name: 'my-func',
        runtime: 'node',
        registry: 'quay.io/test',
        namespace: 'default',
      });
    });
  });

  it('shows an alert when error is set', () => {
    jest.resetModules();
    jest.mock('./useFunctionCreatePage', () => ({
      useFunctionCreatePage: () => ({
        isSubmitting: false,
        error: 'Backend error',
        handleSubmit: mockHandleSubmit,
        handleCancel: mockHandleCancel,
      }),
    }));

    // Re-require after resetting mocks
    const { default: FunctionCreatePageWithError } = require('./FunctionCreatePage');

    render(
      <MemoryRouter>
        <FunctionCreatePageWithError />
      </MemoryRouter>,
    );

    expect(screen.getByText('Backend error')).toBeInTheDocument();
  });
});
```

**Step 7: Run all tests**

Run: `yarn test`
Expected: All tests pass.

**Step 8: Commit**

```bash
git add src/views/useFunctionCreatePage.ts src/views/useFunctionCreatePage.test.ts src/views/FunctionCreatePage.tsx src/views/FunctionCreatePage.test.tsx
git commit -m "refactor: extract useFunctionCreatePage hook from FunctionCreatePage"
```

---

### Task 9: Update design doc to reflect renames and new method signatures

**Files:**

- Modify: `docs/design/2026-03-16-faas-poc-design.md`

**Step 1: Update the design doc**

In the design doc, make these changes:

1. In the file tree section (~line 128), rename `SourceControlService.github.ts` → `GithubService.ts`.

2. In the `SourceControlService` interface definition (~line 222), update to reflect the actual `push` signature:
   - `push(repo: RepoInfo, files: FileEntry[], message: string): Promise<void>;`
   (Note: the design doc uses `GeneratedFiles` — update to `FileEntry[]` to match actual implementation.)

3. Add a note about the consolidated service: `GithubService` in `src/services/source-control/` implements both listing/fetching and push methods.

4. In the hook listing section (~line 314), update:
   - `useSourceControl` → `useSourceControlService`
   - Note that `useGitHubService` was removed (consolidated into `useSourceControlService`)

5. Rename `BackendFunctionService` → `FunctionBackendService` wherever it appears.

**Step 2: Commit**

```bash
git add docs/design/2026-03-16-faas-poc-design.md
git commit -m "docs: update design doc to reflect PR #5 cleanup renames"
```

---

## Summary

| Task | Description | Tests |
|------|-------------|-------|
| 1 | Remove duplicate `@octokit/rest` from devDependencies | Existing pass |
| 2 | Add `push` to `SourceControlService` + `GithubService` | 2 new tests |
| 3 | Update `FunctionCreatePage` to use `useSourceControl` | Existing updated |
| 4 | Delete `src/services/github/` | Existing pass (minus 2) |
| 5 | Rename `BackendFunctionService` → `FunctionBackendService` | Existing updated |
| 6 | Rename `useSourceControl` → `useSourceControlService` | Existing updated |
| 7 | Extract `useCreateFunctionForm` hook | 3 new tests |
| 8 | Extract `useFunctionCreatePage` hook | 4 new tests |
| 9 | Update design doc | None |
