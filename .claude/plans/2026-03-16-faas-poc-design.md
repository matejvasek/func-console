# FaaS PoC — Design Plan

**Updated:** 2026-03-18
**Status:** Design in progress — service interfaces defined, spec review done. Delete flow, error handling, and editor routing pending.

---

## Executive Summary

A **Functions-as-a-Service PoC UI** for the OpenShift Web Console, built as a dynamic plugin. Developers create, edit, and deploy serverless functions without CLI knowledge. **GitHub is the control plane** — the UI generates all function artifacts and pushes them to a GitHub repo, where a GitHub Actions workflow handles build and deployment to the cluster.

Designed for **dual use**: OCP Console dynamic plugin + donatable upstream to Knative as a standalone reference UI.

---

## Architecture

```mermaid
flowchart TB
    subgraph UI [Browser - React + PatternFly 6]
        VIEWS[Views: List / Create / Editor]
        FS[useFunctionService hook]
        SCS[useSourceControl hook]
        CS[useClusterService hook]
    end

    subgraph FS_IMPL [FunctionService Implementations - pick one]
        WASM[A: WASM - func CLI compiled to WebAssembly]
        TSIMPL[B: TypeScript re-implementation]
        BACKEND[C: Thin Go backend wrapping func CLI]
    end

    subgraph External [External Services]
        GH[GitHub API]
        K8S[K8s / OCP API]
    end

    VIEWS --> FS
    VIEWS --> SCS
    VIEWS --> CS

    FS -.-> WASM
    FS -.-> TSIMPL
    FS -.-> BACKEND

    SCS -->|PoC: PAT auth| GH
    CS -->|PoC: OCP SDK hooks| K8S

    GH -->|push triggers| GHA[GitHub Actions]
    GHA -->|func deploy| K8S
    K8S --> FUNC[Running Function - KEDA scaled]
```

---

## Decisions

| # | Decision | Status |
|---|----------|--------|
| 1 | **PoC scope** = Function List + Create + Editor | ✅ |
| 2 | **Tech stack** = React + PatternFly 6 + OCP Dynamic Plugin SDK | ✅ |
| 3 | **Reference** = CronTab plugin for architecture patterns | ✅ |
| 4 | **Source of truth** = GitHub repo with func.yaml + code | ✅ |
| 5 | **Deployment** = GitHub Workflow runs `func deploy` | ✅ |
| 6 | **Deployment type** = KEDA for PoC | ✅ |
| 7 | **Template** = HTTP only for PoC | ✅ |
| 8 | **3 service hooks** as stable API consumed by all views | ✅ |
| 9 | **FunctionService strategy** = spike WASM first, fallback to backend or TS | ✅ |
| 10 | **ClusterService** = OCP SDK hooks, upstream TBD | ✅ |
| 11 | **Create flow** = form → editor → deploy | ✅ |
| 12 | **Extension types** = `console.page/route` + `console.navigation/section` + `console.navigation/href` | ✅ |
| 13 | **Table component** = PatternFly Data view (not deprecated VirtualizedTable) | ✅ |
| 14 | **Project structure** = views/, components/, services/ | ✅ |
| 15 | **Service wiring** = hooks returning singletons or wrapping SDK hooks | ✅ |

---

## User Flow

```mermaid
flowchart LR
    A[Functions List] -->|Create function| B[Create Form]
    B -->|name, runtime, repo, registry| C[Editor View]
    C -->|review/edit generated code| C
    C -->|Deploy| D[Push to GitHub]
    D --> E[GH Actions: build + deploy]
    E --> F[Function running on cluster]
    F -->|status updates| A
    A -->|click function name| C
```

1. User fills in **Create Form** — name, runtime, GitHub repo, registry, namespace
2. `useFunctionService` generates func.yaml, boilerplate code, GitHub Workflow YAML
3. User lands in **Editor** — can review/edit the generated code
4. User clicks **Deploy** — files pushed to GitHub via `useSourceControl`
5. GitHub Actions picks up the push, runs `func deploy`, deploys to cluster
6. User returns to **List** — function appears with live status from `useClusterService`

---

## Project Structure

```
func-console/
├── console-extensions.json       # declares nav items + page routes
├── package.json                  # plugin metadata, exposedModules, deps
├── webpack.config.ts             # webpack module federation config
├── Dockerfile                    # container image for deployment
│
├── src/
│   ├── services/                 # interfaces + implementations
│   │   ├── types.ts              # FunctionConfig, GeneratedFiles, etc.
│   │   ├── function/
│   │   │   ├── FunctionService.ts          # TypeScript interface
│   │   │   ├── useFunctionService.ts       # hook returning singleton
│   │   │   └── FunctionService.github.ts   # PoC implementation
│   │   ├── source-control/
│   │   │   ├── SourceControlService.ts     # TypeScript interface
│   │   │   ├── useSourceControl.ts         # hook returning singleton
│   │   │   └── SourceControlService.github.ts
│   │   └── cluster/
│   │       ├── ClusterService.ts           # TypeScript interface
│   │       └── useClusterService.ts        # hook wrapping OCP SDK hooks
│   │
│   ├── views/                    # exposed modules - $codeRef targets
│   │   ├── FunctionListPage.tsx
│   │   ├── FunctionCreatePage.tsx
│   │   └── FunctionEditorPage.tsx
│   │
│   ├── components/               # reusable UI pieces
│   │   ├── FunctionTable.tsx     # PatternFly Data view
│   │   ├── CreateForm.tsx        # form fields
│   │   └── EmptyState.tsx        # no functions state
│   │
│   └── utils/
│       └── constants.ts          # route paths, labels, defaults
│
├── locales/                      # i18n translations
└── charts/                       # Helm chart for OCP deployment
```

---

## Service Interface Details

### Types

```typescript
type GeneratedFiles = Map<string, string>;  // filepath → content
```

### FunctionService

Generates function artifacts from form inputs. Pure data transformation, no I/O.

```typescript
interface FunctionConfig {
  name: string;                          // "my-function"
  runtime: 'node' | 'python' | 'go';
  registry: string;                      // "quay.io/sjakusch"
  namespace: string;                     // K8s namespace to deploy into
  repoName: string;                      // GitHub repo name
}

interface WorkflowConfig {
  branch: string;                        // default: "main"
}

interface FunctionService {
  generateFunction(config: FunctionConfig): GeneratedFiles;
  generateWorkflow(config: FunctionConfig, workflow: WorkflowConfig): GeneratedFiles;
}
```

- `generateFunction` → func.yaml, handler code, package files, .gitignore, tests
- `generateWorkflow` → .github/workflows/func-deploy.yaml
- Output consumed by TreeView+CodeEditor (display) and Octokit (push)

### SourceControlService

Manages GitHub repos, file pushes, secrets.

**Implementation:** `@octokit/rest` + `@octokit/auth-token`
**Auth:** PoC uses GitHub PAT (user-provided token input). PAT stored in sessionStorage. OAuth to be explored during implementation.
**Discovery:** Repos tagged with `serverless-function` topic.

```typescript
interface RepoInfo {
  owner: string;
  name: string;
  url: string;
  defaultBranch: string;
}

interface SourceControlService {
  authenticate(): Promise<void>;
  isAuthenticated(): boolean;
  createRepo(name: string): Promise<RepoInfo>;
  listFunctionRepos(): Promise<RepoInfo[]>;
  push(repo: RepoInfo, files: GeneratedFiles, message: string): Promise<void>;
  fetch(repo: RepoInfo): Promise<GeneratedFiles>;
  createSecret(repo: RepoInfo, name: string, value: string): Promise<void>;
  createVariable(repo: RepoInfo, name: string, value: string): Promise<void>;
}
```

**Octokit mapping:**

- `createRepo` → `repos.createForAuthenticatedUser` + `repos.replaceAllTopics`
- `listFunctionRepos` → `search.repos({ q: 'topic:serverless-function user:...' })`
- `push` → `git.createBlob` → `git.createTree` → `git.createCommit` → `git.updateRef`
- `fetch` → `git.getTree({ recursive: '1' })` + blob content fetches
- `createSecret` → `actions.createOrUpdateRepoSecret` (requires libsodium encryption)
- `createVariable` → `actions.createRepoVariable`

**Implementation notes:**

- `createSecret` requires encrypting the value with the repo's public key using `tweetnacl` or `libsodium.js`
- `fetch` retrieves the full repo tree and all file contents for display in the editor

### ClusterService

Queries deployed function status from K8s. Consumed as a React hook.

**Implementation:** Hook wrapping OCP Console SDK `useK8sWatchResource`.
**Watches:** `Deployment` resources with label `function.knative.dev/name` (presence selector).
**Runtime:** from label `function.knative.dev/runtime`.
**Replicas:** from `deployment.status.readyReplicas`.
**Namespace:** from OCP SDK `useActiveNamespace()`.

```typescript
interface DeployedFunction {
  name: string;
  namespace: string;
  runtime: string;
  status: 'Running' | 'ScaledToZero' | 'Deploying' | 'Error' | 'Unknown';
  lastDeployed?: string;
  replicas: number;
}

interface ClusterService {
  functions: DeployedFunction[];
  loaded: boolean;
  error: unknown;
  deleteFunction(name: string, namespace: string): Promise<void>;
}
```

**Status derivation from Deployment:**

- `readyReplicas === desiredReplicas` → Running
- `readyReplicas === 0 && desiredReplicas === 0` → ScaledToZero
- `readyReplicas < desiredReplicas` → Deploying
- Conditions contain failure → Error
- Otherwise → Unknown

**Labels set by func CLI** (verified from `pkg/deployer/common.go`):

- `boson.dev/function: "true"` — legacy marker
- `function.knative.dev/name: <name>` — function name
- `function.knative.dev/runtime: <runtime>` — runtime (node, python, go)

**SDK hooks used:**

| Hook | Purpose |
|------|---------|
| `useK8sWatchResource` | Watch deployed functions reactively |
| `useActiveNamespace` | Get/set active namespace |
| `consoleFetchJSON` | HTTP requests with Console headers (used in `deleteFunction`) |

## Service Wiring

Three hooks, same consumer pattern, different internals:

```tsx
// FunctionService + SourceControlService: hook returns a singleton
const instance = new GitHubFunctionService();
export const useFunctionService = (): FunctionService => instance;

// ClusterService: hook wraps OCP SDK reactive hooks
export function useClusterService(): ClusterService {
  const [data, loaded, error] = useK8sWatchResource({...});
  return { functions: data, loaded, error };
}
```

Components always consume via hooks — never import implementations directly:

```tsx
const { functions, loaded } = useClusterService();
const svc = useFunctionService();
```

Swapping implementations = change what the hook returns. Zero component changes.

---

## Views

Implementation details and decisions about the three views we will implement.

### Functions List Page

The function list merges data from **both** GitHub and the cluster:

- **SourceControlService.listFunctionRepos()** — provides all function repos (source of truth)
- **ClusterService** — provides deployment status for functions that are deployed

A function may exist in GitHub but not yet be deployed (status: "Not deployed"). The list correlates by function name (from func.yaml ↔ Deployment label).

#### Delete Flow

Mirrors func CLI behavior (undeploy only, source code unaffected).

**What happens:** Deletes the Deployment by name in namespace. Owner references cascade: K8s Service and KEDA ScaledObject are removed automatically by the API server.

**UI flow:**

1. User clicks kebab → "Delete" on a function row
2. Confirmation modal: "Undeploy function \<name\>? This removes the function from the cluster. The source code in GitHub is not affected."
3. On confirm: `clusterService.deleteFunction(name, namespace)`
4. List refreshes automatically via `useK8sWatchResource` reactivity

**SDK components used:**

| Component | Purpose |
|-----------|---------|
| `ListPageHeader` | Page header with title |
| `ErrorStatus` / `ProgressStatus` / `SuccessStatus` | Status indicators in table rows |
| `ErrorBoundaryFallbackPage` | Catch unexpected errors |
| `useDeleteModal` | Delete confirmation modal |
| PatternFly Data view | Function list table |

### Functions Create Page

What the UI Generates:

| Artifact | Source | Content |
|----------|--------|---------|
| **func.yaml** | FunctionService | name, runtime, registry, namespace, builder, deploy type |
| **Handler code** | FunctionService | Runtime-specific boilerplate (~30 lines) |
| **package files** | FunctionService | package.json / go.mod / requirements.txt |
| **GH Workflow** | FunctionService | 6-step YAML: checkout → test → k8s-context → registry-login → install-func → deploy |
| **GH Secrets** | SourceControlService | KUBECONFIG, REGISTRY_PASSWORD |
| **GH Variables** | SourceControlService | REGISTRY_URL, REGISTRY_USERNAME, REGISTRY_LOGIN_URL |

**SDK components used:**

| Component | Purpose |
|-----------|---------|
| `ErrorBoundaryFallbackPage` | Catch unexpected errors during create flow |

### Functions Edit Page

PatternFly TreeView sidebar + SDK CodeEditor. Tree built from `GeneratedFiles` map keys split on `/`. Shows full repo contents.

**SDK components used:**

| Component | Purpose |
|-----------|---------|
| `CodeEditor` | Monaco-based code editor (lazy loaded) |
| PatternFly TreeView | File tree sidebar built from `GeneratedFiles` map keys |
| `ErrorBoundaryFallbackPage` | Catch unexpected errors |

---

## Console Extensions

```json
[
  {
    "type": "console.navigation/section",
    "properties": { "id": "functions-section", "name": "Functions", "perspective": "dev" }
  },
  {
    "type": "console.navigation/href",
    "properties": { "id": "functions-list", "name": "Functions", "href": "/functions", "section": "functions-section", "perspective": "dev" }
  },
  {
    "type": "console.page/route",
    "properties": { "path": "/functions", "component": { "$codeRef": "FunctionListPage" }, "exact": true }
  },
  {
    "type": "console.page/route",
    "properties": { "path": "/functions/create", "component": { "$codeRef": "FunctionCreatePage" }, "exact": true }
  },
  {
    "type": "console.page/route",
    "properties": { "path": "/functions/edit/:name", "component": { "$codeRef": "FunctionEditorPage" } }
  }
]
```

---

## WASM Feasibility Summary

| Component | WASM-friendly? | Notes |
|-----------|---------------|-------|
| Workflow YAML generation | ✅ Yes | stdlib + yaml.v3 only, ~210 lines |
| Code templates | ✅ Trivial | Static files, embeddable |
| func.yaml generation | ⚠️ Moderate | Needs refactor to remove OS/viper deps |
| Full client.Init | ⚠️ Moderate | Filesystem writes need abstraction |

**Plan:** Refactor func CLI (`cmd/ci/*` → `pkg/ci/github/*`, remove viper), then spike a WASM compilation test.

---

## Next Steps

- [ ] Design delete flow (what gets deleted: repo, cluster resources, or both?)
- [ ] Design error handling (GitHub API failures, invalid PAT, push failures)
- [ ] Clarify editor routing (how to identify function: by repo name? function name?)
- [ ] Spike: WASM compilation of func CLI Go packages
- [ ] Begin implementation

---

## Key References

| Resource | Link |
|----------|------|
| OCPSTRAT-2460 | <https://redhat.atlassian.net/browse/OCPSTRAT-2460> |
| Plugin Template | <https://github.com/openshift/console-plugin-template> |
| CronTab Plugin | <https://github.com/openshift/console-crontab-plugin> |
| Dynamic Plugin SDK | <https://github.com/openshift/console/blob/main/frontend/packages/console-dynamic-plugin-sdk/README.md> |
| Dynamic Plugins Summary | [`resources/ocp-dynamic-plugins-summary.md`](resources/ocp-dynamic-plugins-summary.md) |
| Dynamic Plugin Guide | [`resources/ocp-console-dynamic-plugin-guide.md`](resources/ocp-console-dynamic-plugin-guide.md) |
| PatternFly | <https://patternfly.org> |
| func CLI CI code | `knative-func/cmd/ci/` |
| func templates | `knative-func/templates/{go,node,python}/` |
