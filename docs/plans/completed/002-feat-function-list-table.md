# Function List Table Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render a PatternFly table on the Functions List Page showing function data from GitHub repos merged with K8s deployment status.

**Architecture:** Bottom-up — types first, then SourceControlService (Octokit), ClusterService (OCP SDK hook), FunctionTable component (props-driven), and finally FunctionsListPage wiring with an unexported merging hook. Delete uses SDK's `useDeleteModal`.

**Tech Stack:** React 17, TypeScript, PatternFly 6 (`@patternfly/react-table`), OCP Dynamic Plugin SDK (`useK8sWatchResource`, `useActiveNamespace`, `useDeleteModal`), Octokit (`@octokit/rest`), Jest + React Testing Library.

**Docs to read before starting:**

- `docs/ARCHITECTURE.md` — layered architecture, dependency rules, page/component/hook rules
- `docs/STYLEGUIDE.md` — code style, naming, no `any`, no `console.log`
- `docs/TESTING.md` — TDD flow, mock patterns, forbidden patterns
- `docs/design/2026-03-16-faas-poc-design.md` — service interfaces, status derivation, delete flow

---

## File Structure

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `src/services/types.ts` | Add `RepoInfo` type |
| Create | `src/services/source-control/SourceControlService.ts` | Interface: `listFunctionRepos()`, `fetchFileContent()` |
| Create | `src/services/source-control/GithubService.ts` | Octokit implementation |
| Create | `src/services/source-control/useSourceControl.ts` | Hook returning singleton |
| Create | `src/services/source-control/SourceControlService.test.ts` | Unit tests for Octokit implementation |
| Create | `src/services/cluster/useClusterService.ts` | Hook wrapping `useK8sWatchResource`, returns raw `K8sResourceKind[]` deployments |
| Create | `src/services/cluster/useClusterService.test.ts` | Unit tests for cluster hook |
| Create | `src/components/FunctionTable.tsx` | Props-driven PatternFly table with clickable rows |
| Create | `src/components/FunctionTable.test.tsx` | Component tests |
| Modify | `src/views/FunctionsListPage.tsx` | Add `useFunctionListPage` hook, `deriveStatus`, conditional rendering |
| Modify | `src/views/FunctionsListPage.test.tsx` | Add table/loading/delete tests |
| Modify | `locales/en/plugin__console-functions-plugin.json` | Add new i18n strings |

---

### Task 1: Add `RepoInfo` type to `services/types.ts`

**Files:**

- Create: `src/services/types.ts`

This task has no tests — it's a type-only change. `src/services/types.ts` does not exist on this branch yet (it's only in PR #5). Create it with `RepoInfo` which is consumed by `SourceControlService` in Task 2.

- [ ] **Step 1: Add `RepoInfo` interface**

Create `src/services/types.ts`:

```typescript
export interface RepoInfo {
  owner: string;
  name: string;
  url: string;
  defaultBranch: string;
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `npx tsc --noEmit`
Expected: No errors related to `types.ts`

- [ ] **Step 3: Commit**

```bash
git add src/services/types.ts
git commit -m "feat: add RepoInfo type"
```

---

### Task 2: SourceControlService interface and Octokit implementation

**Files:**

- Create: `src/services/source-control/SourceControlService.ts`
- Create: `src/services/source-control/GithubService.ts`
- Create: `src/services/source-control/useSourceControl.ts`
- Create: `src/services/source-control/SourceControlService.test.ts`

The interface needs `listFunctionRepos()` which returns `RepoInfo[]` and `fetchFileContent()` which fetches a single file from a repo. `RepoInfo` carries source control data only: `owner`, `name`, `url`, `defaultBranch`. Runtime is not on `RepoInfo` — it comes from `func.yaml` content, fetched in the page hook.

#### Step group: Interface

- [ ] **Step 1: Create `SourceControlService` interface**

Create `src/services/source-control/SourceControlService.ts`:

```typescript
import { RepoInfo } from '../types';

export interface SourceControlService {
  listFunctionRepos(): Promise<RepoInfo[]>;
  fetchFileContent(repo: RepoInfo, path: string): Promise<string>;
}
```

This is the minimal interface for the list table feature. `pushFiles()` and other methods will be added when the create page is wired up.

#### Step group: Octokit implementation — TDD

- [ ] **Step 2: Write failing test — `listFunctionRepos` returns repos**

Create `src/services/source-control/SourceControlService.test.ts`:

```typescript
import { GithubService } from './GithubService';
import { RepoInfo } from '../types';

const mockSearch = jest.fn();
const mockGetContent = jest.fn();

jest.mock('@octokit/rest', () => ({
  Octokit: jest.fn().mockImplementation(() => ({
    users: { getAuthenticated: jest.fn().mockResolvedValue({ data: { login: 'twoGiants' } }) },
    search: { repos: mockSearch },
    repos: { getContent: mockGetContent },
  })),
}));

afterEach(() => {
  jest.restoreAllMocks();
});

describe('GithubService', () => {
  it('lists function repos tagged with serverless-function topic', async () => {
    mockSearch.mockResolvedValue({
      data: {
        items: [
          {
            owner: { login: 'twoGiants' },
            name: 'my-func',
            html_url: 'https://github.com/twoGiants/my-func',
            default_branch: 'main',
          },
        ],
      },
    });

    const svc = new GithubService('fake-token');
    const repos: RepoInfo[] = await svc.listFunctionRepos();

    expect(repos).toEqual([
      {
        owner: 'twoGiants',
        name: 'my-func',
        url: 'https://github.com/twoGiants/my-func',
        defaultBranch: 'main',
      },
    ]);
    expect(mockSearch).toHaveBeenCalledWith({ q: 'topic:serverless-function user:twoGiants' });
  });
});
```

- [ ] **Step 3: Run test to verify it fails**

Run: `yarn test -- --testPathPattern=SourceControlService`
Expected: FAIL — `Cannot find module './GithubService'`

- [ ] **Step 4: Install `@octokit/rest`**

Run: `yarn add @octokit/rest`

- [ ] **Step 5: Write minimal implementation**

Create `src/services/source-control/GithubService.ts`:

```typescript
import { Octokit } from '@octokit/rest';
import { RepoInfo } from '../types';
import { SourceControlService } from './SourceControlService';

export class GithubService implements SourceControlService {
  private octokit: Octokit;
  private username: string | undefined;

  constructor(pat: string) {
    this.octokit = new Octokit({ auth: pat });
  }

  async listFunctionRepos(): Promise<RepoInfo[]> {
    if (!this.username) {
      const { data: user } = await this.octokit.users.getAuthenticated();
      this.username = user.login;
    }

    const { data } = await this.octokit.search.repos({
      q: `topic:serverless-function user:${this.username}`,
    });

    return data.items.map((item) => ({
      owner: item.owner.login,
      name: item.name,
      url: item.html_url,
      defaultBranch: item.default_branch,
    }));
  }

  async fetchFileContent(repo: RepoInfo, path: string): Promise<string> {
    const { data } = await this.octokit.repos.getContent({
      owner: repo.owner,
      repo: repo.name,
      path,
    });

    if (!('content' in data)) {
      throw new Error(`${path} is not a file`);
    }
    return atob(data.content);
  }
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `yarn test -- --testPathPattern=SourceControlService`
Expected: PASS

- [ ] **Step 7: Write failing test — `fetchFileContent` returns file content**

Add to `SourceControlService.test.ts`:

```typescript
  it('fetches file content from a repo', async () => {
    mockGetContent.mockResolvedValue({
      data: { content: btoa('name: my-func\nruntime: go\n'), encoding: 'base64' },
    });

    const svc = new GithubService('fake-token');
    const content = await svc.fetchFileContent(
      { owner: 'twoGiants', name: 'my-func', url: 'https://github.com/twoGiants/my-func', defaultBranch: 'main' },
      'func.yaml',
    );

    expect(content).toBe('name: my-func\nruntime: go\n');
    expect(mockGetContent).toHaveBeenCalledWith({
      owner: 'twoGiants',
      repo: 'my-func',
      path: 'func.yaml',
    });
  });
```

- [ ] **Step 8: Run test to verify it passes (should already pass)**

Run: `yarn test -- --testPathPattern=SourceControlService`
Expected: PASS

- [ ] **Step 9: Create the hook**

Create `src/services/source-control/useSourceControl.ts`:

```typescript
import { GithubService } from './GithubService';
import { SourceControlService } from './SourceControlService';

// PAT injected via webpack DefinePlugin from GITHUB_PAT env variable.
// For dev/testing: export GITHUB_PAT=ghp_... before running yarn start.
// DO NOT hardcode a real PAT here — this file is committed.
const pat = (typeof __GITHUB_PAT__ !== 'undefined') ? __GITHUB_PAT__ : '';
const instance = new GithubService(pat);

export function useSourceControl(): SourceControlService {
  return instance;
}
```

Add the webpack `DefinePlugin` entry to `webpack.config.ts`. Find the `plugins` array inside the webpack config and add:

```typescript
new DefinePlugin({
  __GITHUB_PAT__: JSON.stringify(process.env.GITHUB_PAT || ''),
}),
```

Import `DefinePlugin` from `webpack` if not already imported.

And add a type declaration in a new `src/globals.d.ts`:

```typescript
declare const __GITHUB_PAT__: string;
```

- [ ] **Step 10: Commit**

```bash
git add src/services/source-control/ src/globals.d.ts
git commit -m "feat: add SourceControlService with listFunctionRepos and fetchFileContent"
```

---

### Task 3: ClusterService hook

**Files:**

- Create: `src/services/cluster/useClusterService.ts`
- Create: `src/services/cluster/useClusterService.test.ts`

The hook wraps `useK8sWatchResource` to watch Deployments labeled `function.knative.dev/name` and returns raw `K8sResourceKind[]`. Status derivation and merging with GitHub data happens in the page hook.

#### Step group: TDD the hook

- [ ] **Step 1: Write failing test — returns raw deployments from cluster**

Create `src/services/cluster/useClusterService.test.ts`:

```typescript
import { renderHook } from '@testing-library/react';
import { useClusterService } from './useClusterService';

const mockUseK8sWatchResource = jest.fn();
const mockUseActiveNamespace = jest.fn();

jest.mock('@openshift-console/dynamic-plugin-sdk', () => ({
  useK8sWatchResource: (...args: unknown[]) => mockUseK8sWatchResource(...args),
  useActiveNamespace: () => mockUseActiveNamespace(),
}));

afterEach(() => {
  jest.restoreAllMocks();
});

const mockDeployment = {
  apiVersion: 'apps/v1',
  kind: 'Deployment',
  metadata: {
    name: 'func-demo-26',
    namespace: 'demo',
    labels: {
      'function.knative.dev/name': 'func-demo-26',
      'function.knative.dev/runtime': 'go',
    },
  },
  spec: { replicas: 1 },
  status: { readyReplicas: 1 },
};

describe('useClusterService', () => {
  it('returns raw deployments when loaded', () => {
    mockUseActiveNamespace.mockReturnValue(['demo']);
    mockUseK8sWatchResource.mockReturnValue([[mockDeployment], true, null]);

    const { result } = renderHook(() => useClusterService());

    expect(result.current.deployments).toEqual([mockDeployment]);
    expect(result.current.loaded).toBe(true);
    expect(result.current.error).toBeNull();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `yarn test -- --testPathPattern=useClusterService`
Expected: FAIL — `Cannot find module './useClusterService'`

- [ ] **Step 3: Write minimal implementation**

Create `src/services/cluster/useClusterService.ts`:

```typescript
import {
  K8sResourceKind,
  useK8sWatchResource,
  useActiveNamespace,
} from '@openshift-console/dynamic-plugin-sdk';

export interface ClusterService {
  deployments: K8sResourceKind[];
  loaded: boolean;
  error: unknown;
}

export function useClusterService(): ClusterService {
  const [activeNamespace] = useActiveNamespace();

  const [data, loaded, error] = useK8sWatchResource<K8sResourceKind[]>({
    groupVersionKind: { group: 'apps', version: 'v1', kind: 'Deployment' },
    namespace: activeNamespace,
    isList: true,
    selector: {
      matchExpressions: [
        { key: 'function.knative.dev/name', operator: 'Exists' },
      ],
    },
  });

  const deployments = loaded ? (data ?? []) : [];

  return { deployments, loaded, error };
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `yarn test -- --testPathPattern=useClusterService`
Expected: PASS

- [ ] **Step 5: Write failing test — returns empty deployments when not loaded**

Add to `useClusterService.test.ts`:

```typescript
  it('returns empty deployments when not loaded', () => {
    mockUseActiveNamespace.mockReturnValue(['demo']);
    mockUseK8sWatchResource.mockReturnValue([[], false, null]);

    const { result } = renderHook(() => useClusterService());

    expect(result.current.deployments).toEqual([]);
    expect(result.current.loaded).toBe(false);
    expect(result.current.error).toBeNull();
  });
```

- [ ] **Step 6: Run test to verify it passes (should already pass)**

Run: `yarn test -- --testPathPattern=useClusterService`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add src/services/cluster/
git commit -m "feat: add useClusterService hook"
```

---

### Task 4: FunctionTable component

**Files:**

- Create: `src/components/FunctionTable.tsx`
- Create: `src/components/FunctionTable.test.tsx`

A simple, props-driven component. Receives data and callbacks, renders a PatternFly table with clickable rows. FunctionTable defines its own `FunctionTableItem` prop type inline — the page passes data that satisfies it. No import from views (architecture: components don't depend on views).

- [ ] **Step 1: Write failing test — renders table with function data**

Create `src/components/FunctionTable.test.tsx`:

```typescript
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { FunctionTable, FunctionTableItem } from './FunctionTable';

jest.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

const mockUseDeleteModal = jest.fn().mockReturnValue(jest.fn());

jest.mock('@openshift-console/dynamic-plugin-sdk', () => ({
  SuccessStatus: ({ title }: { title: string }) => `Success: ${title}`,
  ProgressStatus: ({ title }: { title: string }) => `Progress: ${title}`,
  ErrorStatus: ({ title }: { title: string }) => `Error: ${title}`,
  InfoStatus: ({ title }: { title: string }) => `Info: ${title}`,
  StatusIconAndText: ({ title }: { title: string }) => `Warning: ${title}`,
  useDeleteModal: (...args: unknown[]) => mockUseDeleteModal(...args),
}));

jest.mock('@patternfly/react-icons', () => ({
  YellowExclamationTriangleIcon: () => 'WarningIcon',
  PencilAltIcon: () => 'EditIcon',
  TrashIcon: () => 'DeleteIcon',
}));

afterEach(() => {
  jest.restoreAllMocks();
});

const mockDeployment = {
  apiVersion: 'apps/v1',
  kind: 'Deployment',
  metadata: { name: 'my-func', namespace: 'demo', labels: { 'function.knative.dev/name': 'my-func' } },
};

const mockFunctions: FunctionTableItem[] = [
  {
    name: 'my-func',
    runtime: 'go',
    status: 'Running',
    url: 'http://my-func.demo.svc',
    replicas: 1,
    namespace: 'demo',
    deployment: mockDeployment,
  },
  {
    name: 'idle-func',
    runtime: 'node',
    status: 'NotDeployed',
    replicas: 0,
    namespace: 'demo',
  },
];

describe('FunctionTable', () => {
  it('renders a row for each function', () => {
    render(
      <MemoryRouter>
        <FunctionTable
          functions={mockFunctions}
          onEdit={jest.fn()}
        />
      </MemoryRouter>,
    );

    expect(screen.getByText('my-func')).toBeInTheDocument();
    expect(screen.getByText('idle-func')).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `yarn test -- --testPathPattern=FunctionTable`
Expected: FAIL — `Cannot find module './FunctionTable'`

- [ ] **Step 3: Write minimal implementation**

Create `src/components/FunctionTable.tsx`:

```typescript
import {
  K8sResourceKind,
  SuccessStatus,
  ProgressStatus,
  ErrorStatus,
  InfoStatus,
  StatusIconAndText,
  useDeleteModal,
} from '@openshift-console/dynamic-plugin-sdk';
import {
  Table,
  Thead,
  Tr,
  Th,
  Tbody,
  Td,
} from '@patternfly/react-table';
import {
  YellowExclamationTriangleIcon,
  PencilAltIcon,
  TrashIcon,
} from '@patternfly/react-icons';
import { Button } from '@patternfly/react-core';
import { useTranslation } from 'react-i18next';

type FunctionStatus =
  | 'CreatingRepo'
  | 'Pushing'
  | 'PushedToGitHub'
  | 'Deploying'
  | 'Running'
  | 'ScaledToZero'
  | 'Error'
  | 'Unknown'
  | 'NotDeployed';

export interface FunctionTableItem {
  name: string;
  runtime: string;
  status: FunctionStatus;
  url?: string;
  replicas: number;
  namespace: string;
  deployment?: K8sResourceKind;
}

interface FunctionTableProps {
  functions: FunctionTableItem[];
  onEdit: (name: string) => void;
}

function StatusCell({ status }: { status: FunctionStatus }) {
  switch (status) {
    case 'Running':
      return <SuccessStatus title={status} />;
    case 'Deploying':
    case 'CreatingRepo':
    case 'Pushing':
    case 'PushedToGitHub':
      return <ProgressStatus title={status} />;
    case 'Error':
      return <ErrorStatus title={status} />;
    case 'ScaledToZero':
    case 'NotDeployed':
      return <InfoStatus title={status} />;
    case 'Unknown':
      return <StatusIconAndText title={status} icon={<YellowExclamationTriangleIcon />} />;
  }
}

function UrlCell({ url }: { url?: string }) {
  if (!url) return <>—</>;

  const hostname = new URL(url).hostname.split('.')[0];
  return (
    <a href={url} target="_blank" rel="noopener noreferrer">
      {hostname}
    </a>
  );
}

function DeleteActionButton({ deployment }: { deployment?: K8sResourceKind }) {
  const { t } = useTranslation('plugin__console-functions-plugin');
  const launchDelete = useDeleteModal(
    deployment as K8sResourceKind,
    undefined,
    undefined,
    t('Undeploy'),
  );

  return (
    <Button
      variant="plain"
      aria-label={t('Delete')}
      icon={<TrashIcon />}
      isDisabled={!deployment}
      onClick={(e) => {
        e.stopPropagation();
        launchDelete();
      }}
    />
  );
}

export function FunctionTable({ functions, onEdit }: FunctionTableProps) {
  const { t } = useTranslation('plugin__console-functions-plugin');

  const columns = [
    t('Name'),
    t('Runtime'),
    t('Status'),
    t('URL'),
    t('Replicas'),
    t('Actions'),
  ];

  return (
    <Table aria-label={t('Functions')}>
      <Thead>
        <Tr>
          {columns.map((col) => (
            <Th key={col}>{col}</Th>
          ))}
        </Tr>
      </Thead>
      <Tbody>
        {functions.map((fn) => (
          <Tr
            key={fn.name}
            isClickable
            onRowClick={() => onEdit(fn.name)}
          >
            <Td dataLabel={t('Name')}>{fn.name}</Td>
            <Td dataLabel={t('Runtime')}>{fn.runtime}</Td>
            <Td dataLabel={t('Status')}><StatusCell status={fn.status} /></Td>
            <Td dataLabel={t('URL')}><UrlCell url={fn.url} /></Td>
            <Td dataLabel={t('Replicas')}>{fn.replicas}</Td>
            <Td dataLabel={t('Actions')} isActionCell>
              <Button
                variant="plain"
                aria-label={t('Edit')}
                icon={<PencilAltIcon />}
                onClick={(e) => {
                  e.stopPropagation();
                  onEdit(fn.name);
                }}
              />
              <DeleteActionButton deployment={fn.deployment} />
            </Td>
          </Tr>
        ))}
      </Tbody>
    </Table>
  );
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `yarn test -- --testPathPattern=FunctionTable`
Expected: PASS

- [ ] **Step 5: Write failing test — renders status components correctly**

Add to `FunctionTable.test.tsx`:

```typescript
  it('renders SuccessStatus for Running functions', () => {
    render(
      <MemoryRouter>
        <FunctionTable
          functions={[mockFunctions[0]]}
          onEdit={jest.fn()}
        />
      </MemoryRouter>,
    );

    expect(screen.getByText('Success: Running')).toBeInTheDocument();
  });

  it('renders InfoStatus for NotDeployed functions', () => {
    render(
      <MemoryRouter>
        <FunctionTable
          functions={[mockFunctions[1]]}
          onEdit={jest.fn()}
        />
      </MemoryRouter>,
    );

    expect(screen.getByText('Info: NotDeployed')).toBeInTheDocument();
  });
```

- [ ] **Step 6: Run tests to verify they pass (should already pass)**

Run: `yarn test -- --testPathPattern=FunctionTable`
Expected: PASS

- [ ] **Step 7: Write failing test — URL displays hostname only**

Add to `FunctionTable.test.tsx`:

```typescript
  it('displays hostname-only link for URL', () => {
    render(
      <MemoryRouter>
        <FunctionTable
          functions={[mockFunctions[0]]}
          onEdit={jest.fn()}
        />
      </MemoryRouter>,
    );

    const link = screen.getByRole('link', { name: 'my-func' });
    expect(link).toHaveAttribute('href', 'http://my-func.demo.svc');
    expect(link).toHaveAttribute('target', '_blank');
  });

  it('displays dash when URL is undefined', () => {
    render(
      <MemoryRouter>
        <FunctionTable
          functions={[mockFunctions[1]]}
          onEdit={jest.fn()}
        />
      </MemoryRouter>,
    );

    expect(screen.getByText('—')).toBeInTheDocument();
  });
```

- [ ] **Step 8: Run tests to verify they pass (should already pass)**

Run: `yarn test -- --testPathPattern=FunctionTable`
Expected: PASS

- [ ] **Step 9: Write failing test — clicking row calls onEdit**

Add to `FunctionTable.test.tsx`:

```typescript
  it('calls onEdit when row is clicked', async () => {
    const onEdit = jest.fn();
    const { user } = renderWithUser(
      <MemoryRouter>
        <FunctionTable
          functions={[mockFunctions[0]]}
          onEdit={onEdit}
        />
      </MemoryRouter>,
    );

    await user.click(screen.getByText('my-func'));

    expect(onEdit).toHaveBeenCalledWith('my-func');
  });
```

Add the `renderWithUser` helper at the top of the test file:

```typescript
import userEvent from '@testing-library/user-event';

function renderWithUser(ui: React.ReactElement) {
  return {
    user: userEvent.setup(),
    ...render(ui),
  };
}
```

- [ ] **Step 10: Run test to verify it passes (should already pass)**

Run: `yarn test -- --testPathPattern=FunctionTable`
Expected: PASS

- [ ] **Step 11: Write failing test — delete button launches useDeleteModal with deployment**

Add to `FunctionTable.test.tsx`:

```typescript
  it('launches delete modal when delete button is clicked on deployed function', async () => {
    const mockLauncher = jest.fn();
    mockUseDeleteModal.mockReturnValue(mockLauncher);
    const { user } = renderWithUser(
      <MemoryRouter>
        <FunctionTable
          functions={[mockFunctions[0]]}
          onEdit={jest.fn()}
        />
      </MemoryRouter>,
    );

    await user.click(screen.getByRole('button', { name: 'Delete' }));

    expect(mockLauncher).toHaveBeenCalled();
    expect(mockUseDeleteModal).toHaveBeenCalledWith(
      mockDeployment,
      undefined,
      undefined,
      'Undeploy',
    );
  });

  it('disables delete button for NotDeployed functions', () => {
    render(
      <MemoryRouter>
        <FunctionTable
          functions={[mockFunctions[1]]}
          onEdit={jest.fn()}
        />
      </MemoryRouter>,
    );

    expect(screen.getByRole('button', { name: 'Delete' })).toBeDisabled();
  });
```

- [ ] **Step 12: Run tests to verify they pass (should already pass)**

Run: `yarn test -- --testPathPattern=FunctionTable`
Expected: PASS

- [ ] **Step 13: Commit**

```bash
git add src/components/FunctionTable.tsx src/components/FunctionTable.test.tsx
git commit -m "feat: add FunctionTable component"
```

---

### Task 5: Wire FunctionsListPage with merging hook and conditional rendering

**Files:**

- Modify: `src/views/FunctionsListPage.tsx`
- Modify: `src/views/FunctionsListPage.test.tsx`
- Modify: `locales/en/plugin__console-functions-plugin.json`

The page gets an unexported `useFunctionListPage` hook that merges data from both services. Delete is handled by `DeleteActionButton` inside FunctionTable (uses SDK's `useDeleteModal` per row). The component conditionally renders: spinner while loading, empty state when no functions, table when functions exist.

#### Step group: TDD the page

- [ ] **Step 1: Write failing test — renders spinner while loading**

Update `src/views/FunctionsListPage.test.tsx`. Replace the existing content:

```typescript
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

const mockUseSourceControl = jest.fn();
jest.mock('../services/source-control/useSourceControl', () => ({
  useSourceControl: () => mockUseSourceControl(),
}));

const mockUseClusterService = jest.fn();
jest.mock('../services/cluster/useClusterService', () => ({
  useClusterService: () => mockUseClusterService(),
}));

jest.mock('../components/FunctionTable', () => ({
  FunctionTable: ({ functions }: { functions: { name: string }[] }) =>
    functions.map((f) => f.name).join(','),
}));

afterEach(() => {
  jest.restoreAllMocks();
});

describe('FunctionsListPage', () => {
  it('renders a spinner while loading', () => {
    mockUseSourceControl.mockReturnValue({
      listFunctionRepos: jest.fn().mockResolvedValue([]),
      fetchFileContent: jest.fn(),
    });
    mockUseClusterService.mockReturnValue({ deployments: [], loaded: false, error: null });

    render(
      <MemoryRouter>
        <FunctionsListPage />
      </MemoryRouter>,
    );

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `yarn test -- --testPathPattern=FunctionsListPage`
Expected: FAIL — no element with role "progressbar" found (currently always renders empty state)

- [ ] **Step 3: Write minimal implementation for loading state**

Update `src/views/FunctionsListPage.tsx`:

```typescript
import { DocumentTitle, ListPageHeader, K8sResourceKind } from '@openshift-console/dynamic-plugin-sdk';
import { PageSection, Spinner } from '@patternfly/react-core';
import { useTranslation } from 'react-i18next';
import { useHistory } from 'react-router-dom';
import { useEffect, useState, useCallback, useMemo } from 'react';
import { FunctionsEmptyState } from '../components/EmptyState';
import { FunctionTable, FunctionTableItem } from '../components/FunctionTable';
import { useSourceControl } from '../services/source-control/useSourceControl';
import { useClusterService } from '../services/cluster/useClusterService';

function deriveStatus(deployment: K8sResourceKind): FunctionTableItem['status'] {
  const desired = deployment.spec?.replicas ?? 0;
  const ready = deployment.status?.readyReplicas ?? 0;
  const conditions = deployment.status?.conditions ?? [];

  const hasFailed = conditions.some(
    (c: { type: string; status: string }) => c.type === 'Available' && c.status === 'False',
  );
  if (hasFailed) return 'Error';

  if (ready === desired && desired > 0) return 'Running';
  if (ready === 0 && desired === 0) return 'ScaledToZero';
  if (ready < desired) return 'Deploying';

  return 'Unknown';
}

function useFunctionListPage() {
  const sourceControl = useSourceControl();
  const { deployments, loaded: clusterLoaded } = useClusterService();
  const history = useHistory();

  const [functionItems, setFunctionItems] = useState<FunctionTableItem[]>([]);
  const [reposLoaded, setReposLoaded] = useState(false);

  // Fetch repos + func.yaml, build partial FunctionTableItem (no cluster data yet)
  useEffect(() => {
    let ignore = false;

    async function loadRepos() {
      const repos = await sourceControl.listFunctionRepos();
      const items = await Promise.all(
        repos.map(async (repo) => {
          const content = await sourceControl.fetchFileContent(repo, 'func.yaml');
          const runtimeMatch = content.match(/^runtime:\s*(.+)$/m);
          const namespaceMatch = content.match(/^namespace:\s*(.+)$/m);
          if (!runtimeMatch) throw new Error(`func.yaml in ${repo.name} missing runtime field`);
          if (!namespaceMatch) throw new Error(`func.yaml in ${repo.name} missing namespace field`);

          return {
            name: repo.name,
            namespace: namespaceMatch[1].trim(),
            runtime: runtimeMatch[1].trim(),
            status: 'NotDeployed' as const,
            replicas: 0,
          };
        }),
      );
      if (!ignore) {
        setFunctionItems(items);
        setReposLoaded(true);
      }
    }

    loadRepos();
    return () => { ignore = true; };
  }, [sourceControl]);

  // Merge repo items with cluster deployments
  const functions = useMemo(
    () =>
      functionItems.map((item) => {
        const deployment = deployments.find(
          (d) => d.metadata?.labels?.['function.knative.dev/name'] === item.name,
        );
        if (!deployment) return item;
        return {
          ...item,
          status: deriveStatus(deployment),
          url: `http://${item.name}.${deployment.metadata?.namespace}.svc`,
          replicas: deployment.status?.readyReplicas ?? 0,
          deployment,
        };
      }),
    [functionItems, deployments],
  );

  const loaded = reposLoaded && clusterLoaded;

  const onEdit = useCallback((name: string) => {
    history.push(`/functions/edit/${name}`);
  }, [history]);

  return { functions, loaded, onEdit };
}

export default function FunctionsListPage() {
  const { t } = useTranslation('plugin__console-functions-plugin');
  const { functions, loaded, onEdit } = useFunctionListPage();

  return (
    <>
      <DocumentTitle>{t('Functions')}</DocumentTitle>
      <ListPageHeader title={t('Functions')} />
      <PageSection>
        {!loaded && <Spinner aria-label={t('Loading')} />}
        {loaded && functions.length === 0 && <FunctionsEmptyState />}
        {loaded && functions.length > 0 && (
          <FunctionTable functions={functions} onEdit={onEdit} />
        )}
      </PageSection>
    </>
  );
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `yarn test -- --testPathPattern=FunctionsListPage`
Expected: PASS

- [ ] **Step 5: Write failing test — renders empty state when loaded with no functions**

Add to `FunctionsListPage.test.tsx`:

```typescript
  it('renders the empty state when loaded with no functions', async () => {
    mockUseSourceControl.mockReturnValue({
      listFunctionRepos: jest.fn().mockResolvedValue([]),
      fetchFileContent: jest.fn(),
    });
    mockUseClusterService.mockReturnValue({ deployments: [], loaded: true, error: null });

    render(
      <MemoryRouter>
        <FunctionsListPage />
      </MemoryRouter>,
    );

    expect(
      await screen.findByRole('heading', { name: 'No functions found' }),
    ).toBeInTheDocument();
  });
```

- [ ] **Step 6: Run test to verify it passes (should already pass)**

Run: `yarn test -- --testPathPattern=FunctionsListPage`
Expected: PASS

- [ ] **Step 7: Write failing test — renders table when functions exist**

Add to `FunctionsListPage.test.tsx`:

```typescript
  it('renders table when functions are loaded', async () => {
    mockUseSourceControl.mockReturnValue({
      listFunctionRepos: jest.fn().mockResolvedValue([
        { owner: 'twoGiants', name: 'my-func', url: 'https://github.com/twoGiants/my-func', defaultBranch: 'main' },
      ]),
      fetchFileContent: jest.fn().mockResolvedValue('name: my-func\nruntime: go\nnamespace: demo\n'),
    });
    mockUseClusterService.mockReturnValue({
      deployments: [
        {
          apiVersion: 'apps/v1',
          kind: 'Deployment',
          metadata: { name: 'my-func', namespace: 'demo', labels: { 'function.knative.dev/name': 'my-func', 'function.knative.dev/runtime': 'go' } },
          spec: { replicas: 1 },
          status: { readyReplicas: 1 },
        },
      ],
      loaded: true,
      error: null,
    });

    render(
      <MemoryRouter>
        <FunctionsListPage />
      </MemoryRouter>,
    );

    expect(await screen.findByText('my-func')).toBeInTheDocument();
  });
```

- [ ] **Step 8: Run test to verify it passes**

Run: `yarn test -- --testPathPattern=FunctionsListPage`
Expected: PASS

- [ ] **Step 9: Write failing test — merges repo with NotDeployed status when no cluster match**

Add to `FunctionsListPage.test.tsx`:

```typescript
  it('shows NotDeployed status for repos without cluster deployment', async () => {
    mockUseSourceControl.mockReturnValue({
      listFunctionRepos: jest.fn().mockResolvedValue([
        { owner: 'twoGiants', name: 'orphan-func', url: 'https://github.com/twoGiants/orphan-func', defaultBranch: 'main' },
      ]),
      fetchFileContent: jest.fn().mockResolvedValue('name: orphan-func\nruntime: node\nnamespace: demo\n'),
    });
    mockUseClusterService.mockReturnValue({ deployments: [], loaded: true, error: null });

    render(
      <MemoryRouter>
        <FunctionsListPage />
      </MemoryRouter>,
    );

    expect(await screen.findByText('orphan-func')).toBeInTheDocument();
  });
```

- [ ] **Step 10: Run test to verify it passes (should already pass)**

Run: `yarn test -- --testPathPattern=FunctionsListPage`
Expected: PASS

- [ ] **Step 11: Add i18n strings**

Update `locales/en/plugin__console-functions-plugin.json`:

```json
{
  "Functions": "Functions",
  "No functions found": "No functions found",
  "Create a serverless function to get started.": "Create a serverless function to get started.",
  "Create function": "Create function",
  "Loading": "Loading",
  "Name": "Name",
  "Runtime": "Runtime",
  "Status": "Status",
  "URL": "URL",
  "Replicas": "Replicas",
  "Actions": "Actions",
  "Edit": "Edit",
  "Delete": "Delete"
}
```

- [ ] **Step 12: Run all tests**

Run: `yarn test`
Expected: All tests pass

- [ ] **Step 13: Commit**

```bash
git add src/views/FunctionsListPage.tsx src/views/FunctionsListPage.test.tsx locales/
git commit -m "feat: wire FunctionsListPage with merging hook and table"
```

---

### Task 6: Final verification

- [ ] **Step 1: Run full test suite**

Run: `yarn test`
Expected: All tests pass

- [ ] **Step 2: TypeScript check**

Run: `npx tsc --noEmit`
Expected: No errors

- [ ] **Step 3: Verify app loads in browser**

Run: open `http://localhost:9000` and navigate to `/functions`
Expected: Page renders (spinner → empty state if no GitHub PAT configured, or table if repos exist)

- [ ] **Step 4: Update design doc**

Review the implementation against the design doc `docs/design/2026-03-16-faas-poc-design.md`, present to the user and after approval update any sections that diverged.
