# SRVOCF-1038: Show deployment status and pipeline failures in the UI

Status: Implemented
Date: 2026-08-26
Updated: 2026-10-19 (synced with the implementation)
Jira: SRVOCF-1038 (Story, parent SRVOCF-953)

## Problem

After "Save & Deploy", a function is pushed to GitHub, a GitHub Actions workflow
builds the image and deploys a Knative Service, and only then does the cluster
show status. Before this work, function status came entirely from a K8s watch of
the Knative Service plus its Deployment. That meant:

- The build window (queued, building, deploying) was invisible in the UI.
- A pipeline failure (compile error, image push error) was completely invisible:
  the ksvc simply never appeared and the row showed "Not deployed" forever.

This work surfaces GitHub Actions build status in the functions list and keeps
the list current as a deployment progresses.

## Scope

- Surface only in the **functions list** (no separate post-save detail view).
- Simple statuses only: a build is **Building**, **Succeeded** or **Failed**,
  plus **None** for "no run to report". More granular states can come later.
- Live updates via **SSE**, one stream for the whole list.
- Deterministic testing via a scripted fake GitHub Actions API.

## Where the code lives

End to end, GitHub to table cell:

```
GitHub Actions ──poll 3s──> scm/github/watch.go ──chan──> handler/build.go
                                                               │ SSE
                                                               v
FunctionTable.tsx <── useFunctions <── functionsClient.ts <── sseEventSource.ts
  (merge + render)     (FunctionsListPage.tsx)                 (browser)
                            ^
                            └── useK8sWatchResource (ksvc + Deployment)
```

| File | Role |
|------|------|
| `backend/scm/client.go` | SCM interface definition: `Client.WatchWorkflowRuns` and the `WorkflowWatch` / `WorkflowRun` / `BuildStatus` types it is declared in terms of |
| `backend/scm/github/watch.go` | The poll loop: discovery, per-repo fetch, status derivation, change-only emission |
| `backend/scm/github/client.go` | ETag-caching HTTP client with forced revalidation |
| `backend/functions/ci.go` | `WorkflowFilename`, the single workflow the watch is scoped to |
| `backend/handler/build.go` | The SSE endpoint, `WorkflowRunDTO` / `ErrorDTO`, error mapping |
| `backend/ticker/ticker.go` | Ticker abstraction behind an injectable factory, so tests step time instead of sleeping |
| `backend/fakegithub/server.go` | Fake Actions API plus the `/_admin` scripting control |
| `src/common/transport/sseEventSource.ts` | Custom SSE client implementing a subset of the native `EventSource` API, generic over event names |
| `src/common/clients/functionsClient.ts` | `receiveWorkflowRunEvents`: wire events to callbacks |
| `src/common/types.ts`, `src/common/errors.ts` | `WorkflowRunRecord`, `WorkflowRun`, `AppError`, `ClusterError` |
| `src/pages/function-list/FunctionsListPage.tsx` | `useFunctions`: merges list, K8s watches and the SSE stream |
| `src/pages/function-list/components/FunctionTable.tsx` | `StatusCell`, the merge table rendered |
| `e2e/helpers/fakegithub.ts` | `setWorkflowRun` and reset helpers for e2e |

Cadences, all hardcoded (no config or env override; changing one means editing
the constant):

| Cadence | Value | Defined in |
|---------|-------|------------|
| Workflow run poll | 3s | `defaultWatchPollInterval`, `scm/github/watch.go` |
| Repo rediscovery | 30s | `defaultWatchRediscoverInterval`, `scm/github/watch.go` |
| SSE heartbeat | 15s | `defaultHeartbeat`, `handler/build.go` |
| Client reconnect delay | 3s | `reconnectDelayMs` default, `sseEventSource.ts` |

None of these is configurable at runtime: there is no flag, env var or API to
change them, so retuning one means editing the constant. They are overridable
in-process, though, which is how tests reach them: `github.WithWatchTickerFactories`
and `handler.WithHeartbeatTickerFactory` are exported options that replace the
ticker driving a cadence (not its duration). On the frontend, `reconnectDelayMs`
is a transport option, but `functionsClient.ts` does not pass it, so the 3s
default is what runs in the app.

## Lifecycle and status merge

```
                git push
                    │
                    ▼
  ┌────────────────────────────────────┐
  │ GitHub Actions run                 │  GitHub Actions is
  │ queued ─► in_progress ─► completed │  authoritative here
  └─────────────────┬──────────────────┘
            ┌───────┴────────┐
         success          failure
            │                 │
            ▼                 ▼
  ┌──────────────────┐   never reaches the cluster:
  │ ksvc created     │   no ksvc, no Deployment. Before
  │         │        │   this work the row read
  │         ▼        │   "Not deployed" forever.
  │ Deployment ready │
  └─────────┬────────┘
            │  the cluster is authoritative
            ▼  from here on
         Running
```

The two sources are authoritative at different times, and neither covers the
whole lifecycle on its own, so the frontend merges them per function.

The merged status is a composite, not a single flattened string:

```ts
interface FunctionStatus {
  cluster: ClusterStatus;  // from the ksvc + Deployment watch
  workflow: WorkflowRun;   // from the SSE build stream
}

interface ClusterStatus {
  status: 'NotDeployed' | 'Deploying' | 'Running' | 'ScaledToZero' | 'Undeploying' | 'Error';
  error?: ClusterError;
}

interface WorkflowRun {
  status: 'None' | 'Building' | 'Succeeded' | 'Failed';
  url?: string;
  error?: AppError;
}
```

A function is treated as **known to the cluster** when the watch matched a ksvc
for it, whatever its status. It has deployed at least once, so a subsequent build
is a rebuild. Expressed in the type above: `cluster.status !== 'NotDeployed'`.

| Workflow run         | Cluster      | Shown status                                       |
|----------------------|--------------|----------------------------------------------------|
| Building             | known        | cluster status + secondary "in progress" indicator |
| Building             | not known    | "Building" + "in progress" indicator               |
| Failed               | known        | cluster status + secondary "failed" indicator      |
| Failed               | not known    | "Build failed", linked to the run                  |
| Succeeded            | known        | cluster status                                     |
| Succeeded            | not known    | "Deploying"                                        |
| None (no run)        | known        | cluster status                                     |
| None (no run)        | not known    | "Not deployed"                                     |

Notes:

- The build status is **non-destructive** over a function the cluster knows
  about: it keeps its cluster status even while a new revision builds or a
  rebuild fails, so the cluster state is never misrepresented. The build activity
  is surfaced as a small secondary indicator next to the status: a spinning sync
  icon (tooltip "Workflow run in progress") while building, or a red
  (danger-colored) warning icon (tooltip "Latest workflow run failed", link to
  the run) when the latest run failed. The failed tooltip is phrased to make
  clear the function is still available and only the latest rebuild failed, not
  the function itself. This avoids flip-flopping a deployed function between its
  cluster status and "Building"/"Build failed" on every redeploy.
- The tooltip says "workflow run" rather than "build" because the run covers
  both building and deploying. The primary status moves from "Building" to
  "Deploying" partway through the same run, so calling the indicator a build
  would stop being accurate.
- The gate is **cluster presence, not the status value.** Gating on availability
  ("Running"/"Scaled to zero") instead looked equivalent but was not: a live
  function reports "Deploying" for a second or two while the build applies its new
  revision (ksvc `Ready=Unknown`, or no Deployment for the latest ready revision
  yet), and during that window the merge overwrote the status with "Building", so
  every redeploy flickered Running -> Building -> Running. Cluster presence
  also keeps a broken ksvc (`Ready=False` -> "Error") as the primary status
  instead of hiding it behind "Build failed".
- For a function the cluster does **not** know about, the build status is the most
  useful thing to show, so it becomes the primary status: "Building" on a first
  deploy or a redeploy of a deleted function, "Build failed" when the run failed,
  and "Deploying" once the run succeeded but the ksvc has not appeared yet. That
  last case closes the gap between a green workflow and the first cluster
  resource, which would otherwise read as "Not deployed".
- "Build failed" (and the secondary failed indicator) links to the failing run.
  No failure reason is surfaced in the list; the run is one click away.
- A cluster "Error" carries a `ClusterError` built from the ksvc `Ready`
  condition and shows "Deployment error. Reason: `<reason>`." in a tooltip. The
  reason comes from the cluster, so unlike build failures there is no external
  run to link to.
- The backend returns a build-centric status (Building/Succeeded/Failed/None);
  the merge to `FunctionStatus` happens in the frontend, which is the only place
  that also has the cluster status.

## Source decision: polling GitHub, not webhooks

GitHub can push `workflow_run` events to a webhook, which would be lower latency
than a 3s poll, but it is a much bigger system: a publicly reachable route into
the cluster, a webhook plus signing secret registered and kept in sync on every
function repo, signature verification, and server-side state to fan each event
out to the right browser session. Polling needs none of that. It runs inside the
existing user-scoped request, holds no state beyond the life of the connection
(matching the stateless backend below), and unchanged polls are 304s, so the
steady-state cost is close to zero. The push that is actually needed, backend to
browser, is the one SSE provides.

## Transport decision: SSE over consoleFetch stream

- Server-to-client push only, so SSE fits better than WebSocket (full-duplex we
  would never use).
- The GitHub PAT lives only in the browser (sessionStorage) and reaches the
  backend as the `X-SCM-Token` header. Both native `EventSource` and native
  `WebSocket` cannot set custom headers, so they would force the PAT into a URL
  query param or WS subprotocol. Reading an SSE stream with `consoleFetch` +
  `ReadableStream` lets us send the PAT as a header cleanly. This is the same
  pattern the OpenShift Lightspeed console plugin uses for streaming chat.
- SSE is plain `net/http` + `Flusher`: zero new backend dependencies, matching
  the stdlib-only backend. WebSocket would need a third-party library.

### Custom EventSource, and the way back to the native one

Because native `EventSource` is unusable (no custom headers, no custom fetch),
the frontend reimplements it in `src/common/transport/sseEventSource.ts`.

It is **deliberately a subset of the native API, not a polyfill of it.** It
implements only what this feature needs, and for everything it does implement it
matches native semantics, so consuming code is written against `EventSource` as
a concept rather than around a temporary workaround. What it provides:

- `addEventListener(type, cbk)` for named events, plus `'open'` and `'error'`.
- `close()`.
- Automatic reconnection after a delay.
- Comment frames (lines starting with `:`) ignored, so heartbeats are invisible
  to consumers.

Not implemented, because nothing here needs it: the default `message` event type
(a frame with no `event:` field is dropped rather than dispatched as `message`),
`id:` and Last-Event-ID resumption, the server-sent `retry:` field, the
`readyState` / `url` / `withCredentials` properties, the `onopen` / `onmessage` /
`onerror` setters, and `removeEventListener`. It is a factory returning a plain
object, not an `EventTarget` subclass, so it cannot be passed somewhere that
expects a real `EventSource`.

Two deliberate deviations where it does differ in behaviour:

- The `error` listener receives an `Error` (an `HttpError` carrying the status
  code when the failure was an HTTP response) rather than a bare `Event`. Native
  `EventSource` hides the status; our fetch has the `Response`, so we surface it.
- The transport always reconnects, including after HTTP errors such as 401,
  where native `EventSource` would go to `CLOSED` and stop. Deciding that an
  error is terminal is a policy question the transport cannot answer, so it is
  left to the consumer, which calls `close()`.

Migration path: once the backend authenticates with an HTTP-only session cookie
instead of a PAT header, native `EventSource` becomes usable and this file can be
deleted. Because the implemented surface is a subset, consumers port unchanged,
and only two things change: the `HttpError` context is lost, and auth failures
must then be detected from the `app-error` event the backend already sends.

## Backend

Stateless, like the rest of the backend: no shared in-memory store (unlike the
removed SSE spike). Each request creates its own SCM client from the caller's
PAT; each SSE connection runs its own poll loop.

### SCM interface (`backend/scm/client.go`)

One method on `scm.Client`. Polling lives behind it, so the handler stays a pure
SSE writer and the whole poll loop is stubbable in handler tests:

```go
WatchWorkflowRuns(ctx context.Context, workflowFile string) (WorkflowWatch, error)
```

```go
type WorkflowWatch interface {
    ResultChan() <-chan WorkflowRunsOrErr
    Stop()
}

type WorkflowRunsOrErr struct {
    Runs map[string]WorkflowRun
    Err  error
}

type WorkflowRun struct {
    BuildStatus BuildStatus
    HTMLURL     string
    Error       error
}

type BuildStatus int // None | Building | Succeeded | Failed
```

Design notes on these shapes:

- **A `WorkflowWatch` rather than a bare channel.** The caller needs to stop a
  long-lived poll loop that outlives the request context it was started from, and
  a channel alone gives it no way to say so. The resulting result-channel plus
  `Stop()` pair is the same shape as a Kubernetes `watch.Interface`, which is
  what the rest of the list page already consumes.
- **The status is derived inside the SCM layer**, not passed up as raw GitHub
  `status`/`conclusion` strings. GitHub's vocabulary then stops at the SCM
  boundary and the handler only has to serialize. Fields nothing consumes (run
  ID, head SHA) are not carried at all.
- **Errors travel in-band**, in the same `WorkflowRunsOrErr` as the snapshots,
  rather than on a second channel. One ordered stream, and a recoverable error
  does not have to end the watch.
- **Two error levels.** `WorkflowRunsOrErr.Err` is stream-wide (repo discovery
  failed, token revoked). `WorkflowRun.Error` is scoped to one repo, so one
  failing repo degrades one row instead of the whole list.
- **A map, not a slice.** Both the change detection in the poll loop and the
  frontend merge are keyed lookups by `owner/name`; neither has any use for an
  ordering.

The workflow file is the only parameter; the method is otherwise
**user-scoped**. It discovers the caller's function repos itself (`ListRepos`,
`topic:serverless-function user:<login>`) and polls each one's default branch.
Discovery runs synchronously so an auth failure is returned to the caller
instead of being lost in the goroutine. `Repo.FullName()` (`owner/name`) is the
key everything is correlated on, matching what `listFunctions` returns.

### GitHub implementation (`backend/scm/github/watch.go`)

- Per repo, `Actions.ListWorkflowRunsByFileName(ctx, owner, repo, workflowFile,
  {Branch: defaultBranch, PerPage: 1})`. Runs come back `created_at` descending,
  so the single element is the newest. A 404 means the workflow does not exist
  there (a non-func repo, or not pushed yet) and maps to no run.
- Scoped to a single workflow file, `functions.WorkflowFilename` (func's
  `func-deploy.yaml`), so an unrelated workflow in the same repo cannot be
  reported as the function's build.
- No per-job lookup. A composed failure reason ("<job> / <step>") was tried and
  dropped: it cost an extra `ListWorkflowJobs` call per failed repo per poll, it
  is GitHub-Actions-shaped and would not port to another builder, and it told a
  user little they would not get by opening the run.
- Repos are polled concurrently (errgroup, limit 10). A per-repo error carries
  that repo's last-known run forward with `WorkflowRun.Error` set, rather than
  flickering the status to `None`.
- The channel carries **changes only**: a snapshot equal to the previous one is
  not sent. This covers per-repo errors too, because a repeated failure reuses
  the previous error instance and so compares equal. It does **not** cover
  stream-wide errors: the dedup check is skipped whenever `Err != nil`, so a
  persistently failing rediscovery re-emits an identical event every 30s for the
  life of the connection.
- Poll interval ~3s. The repo set is rediscovered on a slower cadence (~30s) so
  newly created or deleted functions appear/disappear without reconnecting.
- **A failed rediscovery does not end the watch.** It emits
  `WorkflowRunsOrErr{Runs: <last snapshot>, Err: err}` and keeps polling the repo
  set it already has. Closing the channel was the original design and was
  reversed so that the client, not the server, decides when the connection ends:
  the server cannot tell a transient GitHub failure from a terminal one, and
  closing on the transient case also tripped the browser's own connection-closed
  error, which needed a workaround to suppress. The user now sees the error while
  the last-known statuses stay on screen, and the frontend closes the stream
  itself for the one case it knows is terminal (see the `401` handling below).
- Errors mapped through the existing `mapErr` (401/403 -> `scm.ErrUnauthorized`).
- The GitHub HTTP client issues conditional requests using the ETags GitHub
  returns, and forces revalidation on every poll rather than honouring GitHub's
  freshness window (which would hide a new build for up to ~60s). An unchanged
  run answers 304, which does not count against the primary rate limit. The cache
  is per-PAT, like the client, so one user's cached responses are never served to
  another.

### Endpoint (`backend/handler/build.go`)

`GET /api/v1/func/build/watch` (SSE) is the only build endpoint. It is
**parameterless and user-scoped**, mirroring `GET /api/v1/func/list`: it requires
only the `X-SCM-Token` header, and repo discovery happens inside
`WatchWorkflowRuns`. No owner/name/branch params, no per-function URLs, one
connection for the whole list.

- Headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`,
  `Connection: keep-alive`, `X-Accel-Buffering: no`, then an immediate flush so
  the client's request completes and it can start reading.
- Each snapshot from the watch is written as one `event: build-status` frame.
  The first frame is the full snapshot; a full snapshot is re-sent whenever any
  function's build status changes.
- Heartbeat comment (`:\n\n`) every 15s to survive proxy idle timeouts.
- Exit on `r.Context().Done()`, on write error (detects client disconnect
  without TCP close), or when the watch channel closes. The first two are
  learnings from the spike (c5a1455, f7059667).

#### Errors are events, not status codes

The original design returned a plain `401` for a bad token by calling
`WatchWorkflowRuns` before writing any SSE header. That was reversed: the handler
now commits `200` and the SSE headers immediately, and **every** failure, before
or during the stream, is reported as an `app-error` event.

The reason is that the status code did not survive the trip. A browser
`EventSource` does not expose the HTTP status to its error handler, so a `401`
sent before the stream opened reached the frontend as an indistinguishable
generic error, which is exactly the case the frontend most needs to tell apart.
Carrying the code in the event body makes it readable. The secondary reason is
symmetry: only the very first failure could ever have been a status code, since a
mid-stream auth failure or rediscovery error has to be an event anyway, so the
status-code path bought one special case and a second error-handling path on the
client for it.

The custom transport this plugin uses *could* read the status (it is a `fetch`
underneath), but the wire format is deliberately kept to what a native
`EventSource` can consume, for the reasons in the transport section above.

The only status code the endpoint still produces itself is `500`, when the
`ResponseWriter` does not implement `http.Flusher`. Without flushing, events
would buffer and heartbeats would never arrive, so failing fast is better than
serving a stream that silently does not stream.

**Known gap:** "the client decides when the connection ends" holds for errors
raised once the watch is running, but not before it. All three pre-stream
failures write their `app-error` and then return, ending the response. The
transport reconnects unconditionally, so a non-401 pre-stream failure loops:
during a GitHub outage the `502` path closes, the client retries 3s later, the
handler redoes `Users.Get` plus `Search.Repositories` and fails again. That is
roughly 20 search calls a minute against a 30/minute limit, with the alert
re-firing each cycle. The `401` case is spared only because the frontend
special-cases it.

#### Wire format

`build-status`, keyed by `owner/name`:

```json
{
  "matejvasek/fn-testing-a": {
    "status": "Failed",
    "url": "https://github.com/matejvasek/fn-testing-a/actions/runs/123"
  },
  "matejvasek/fn-testing-b": {
    "status": "Building",
    "url": "https://github.com/matejvasek/fn-testing-b/actions/runs/124",
    "error": { "message": "Unable to fetch build status. Please try again later." }
  }
}
```

`url` is omitted when there is no run. `error` is omitted when the repo polled
cleanly; when present, the accompanying `status` is the last-known one carried
forward.

The two ends of this contract are declared separately and must be kept in step:

```go
// backend/handler/build.go
type WorkflowRunDTO struct {
    Status string    `json:"status"`
    URL    string    `json:"url,omitempty"`
    Error  *ErrorDTO `json:"error,omitempty"`
}

type ErrorDTO struct {
    Message string `json:"message"`
    Code    *int   `json:"code,omitempty"`
}
```

```ts
// src/common/types.ts
type WorkflowRunRecord = Record<string, WorkflowRun>; // keyed by "owner/repo"

interface WorkflowRun {
  status: 'None' | 'Building' | 'Succeeded' | 'Failed';
  url?: string;
  error?: AppError;
}

// src/common/errors.ts
interface AppError {
  message: string;
  code?: number;
}
```

`WorkflowRun` is the TypeScript image of `WorkflowRunDTO`, and `AppError` of
`ErrorDTO`, field for field. Note that the backend has **two** types called
WorkflowRun: `scm.WorkflowRun` is the internal one (int enum, `error`), and
`handler.WorkflowRunDTO` is the serialized one. Only the DTO is part of this
contract; the TS type mirrors the DTO, not the SCM type. The handler owning its
own DTO is what lets the SCM layer use Go-native types without those leaking
into the wire format.

Nothing enforces the pairing. Each side is tested against hand-written frames of
its own (handler tests assert the Go DTO's JSON, client tests feed the TS types
back through the real parser), so a rename on one side alone would pass both
suites. Only the e2e run, which drives the real backend, would catch it.

`app-error`, for stream-wide failures:

```json
{ "message": "Authentication failed. Please check your access token.", "code": 401 }
```

`code` is an HTTP status code. `401` marks an auth failure and is the only one
the frontend branches on (it stops reconnecting and prompts for a new token);
`502` is sent when initial repo discovery fails. Errors raised mid-stream carry
`401` or no code at all.

`status` is one of `Building | Succeeded | Failed | None`, derived from the run:

- `queued` / `in_progress` / `waiting` / `requested` / `pending` -> `Building`
- `completed` + `success` -> `Succeeded`
- `completed` + `failure|cancelled|timed_out|startup_failure` -> `Failed`
- `completed` + `skipped|neutral|stale|action_required` -> `None`; these are not
  failures, so the frontend falls back to the cluster status rather than showing
  a red badge
- no run -> `None`

#### Error messages are sanitized

No upstream error reaches the browser verbatim; the original is logged
server-side with full detail. For errors raised once the watch is running there
are exactly two user-facing strings, via `errorToMessage`: an auth-specific one,
and a generic "Unable to fetch build status. Please try again later." for
everything else (rate limits, network failures, rediscovery errors).

**Known gap:** the three pre-stream failures do not go through `errorToMessage`.
They carry developer-facing strings ("X-SCM-Token header is required", "invalid
SCM token", "failed to list repositories"), and the frontend only substitutes a
friendly message when `code === 401`, so a `502` discovery failure renders the
literal text "failed to list repositories" in the page's danger alert.

GitHub's error strings embed request URLs and other upstream detail that the
user cannot act on and that the UI has no way to present usefully. The
distinction the user can act on is "your token is bad" versus "something went
wrong, retry", and that is what the two in-stream messages convey.

## Fake GitHub (`backend/fakegithub`)

The Actions API surface plus an admin control to script runs deterministically.

GitHub API:

- `GET /repos/{owner}/{repo}/actions/workflows/{workflow}/runs` ->
  `{ total_count, workflow_runs: [...] }`, filtered by `?branch=` and by the
  workflow file in the path, mirroring real GitHub. Each run: `id, head_branch,
  head_sha, status, conclusion, html_url`, returned most-recent first. The
  repo-wide `/actions/runs` listing is deliberately not implemented:
  nothing calls it once build status is scoped to one workflow.

Admin control:

- `POST /_admin/actions/runs` with `{ owner, repo, branch, headSha, status,
  conclusion, workflow }` appends a run to the repo. Lets tests drive
  queued -> in_progress -> failed transitions with exact control. `workflow`
  defaults to the func build workflow; set it to script a run under a different
  workflow and assert build status stays scoped.
- `POST /_admin/reset` also clears runs.

State: runs are an **append-only log** on the in-memory `repo` struct, not a
mutable latest-run field. GitHub's runs are immutable records, so modelling them
that way keeps the fake honest: a test that scripts `in_progress` then
`completed` produces two runs, as real GitHub would, and the by-file-name
endpoint picking the newest match is then a property of the endpoint rather than
something the fake has quietly arranged.

## Frontend

### SSE transport (`src/common/transport/sseEventSource.ts`)

`createSSEEventSource(url, { fetchFn, headers, reconnectDelayMs })` returns
`{ addEventListener, close }`, the subset of the native `EventSource` API this
feature uses. See the transport decision above for why it exists, what it leaves
out, and how it differs. It is generic over the event names; it knows nothing
about build status.

### Client function (`src/common/clients/functionsClient.ts`)

`receiveWorkflowRunEvents(eventCbk, errorCbk)` opens the stream (PAT in
`X-SCM-Token`, `consoleFetch` with timeout `0` so the SDK's ~60s abort does not
kill a long-lived stream) and returns a close function. It owns the mapping from
the three wire events to its two callbacks:

- `build-status` -> `eventCbk(runs)` as a `WorkflowRunRecord`.
- `app-error` -> `errorCbk(message)`. A `code` of `401` is terminal: it reports a
  reconnect-and-refresh message and closes the stream, because a bad or expired
  PAT will not recover and reconnecting would spin.
- transport `error` -> `errorCbk` with either `Server error (<status>)` for an
  `HttpError` or a "connection lost, reconnecting" message otherwise. The
  transport reconnects on its own, so the message says so.

There are no per-function arguments; the backend scopes the stream to the user.

Wire types (`WorkflowRunRecord`, `WorkflowRun`, `AppError`) stay in
`src/common/` rather than in the page directory, because they are the client's
half of the backend contract rather than page-local domain types. They are
declared alongside their Go counterparts under Wire format above.

### List integration (`src/pages/function-list/FunctionsListPage.tsx`)

A private `useFunctions` hook owns the whole composition, as the page is the only
consumer. It layers three sources onto a `Map` keyed `namespace/name`:

1. `listFunctions(namespace)` for the repo/cluster function metadata, which seeds
   the map.
2. Two `useK8sWatchResource` watches (Knative Service and Deployment, selected by
   the `function.knative.dev/name` label), folded in by `withClusterData`, which
   also derives `ClusterStatus` from the ksvc `Ready` condition and the
   Deployment's ready replicas.
3. `receiveWorkflowRunEvents`, folded in by `withWorkflowRunData`, looking each
   function up by its `owner/repo` key.

The stream is re-established when the auth `connectionId` changes, so it picks up
the current PAT on in-place login and account switch, and on an explicit refresh.

Page-local domain types (`Function`, `FunctionStatus`, `ClusterStatus`) live in
`src/pages/function-list/types.ts`; only the wire types stay in `common`.

### Rendering (`src/pages/function-list/components/FunctionTable.tsx`)

`FunctionTable` takes `Function[]` and is otherwise a plain renderer. `StatusCell`
implements the merge table above against `fn.status.cluster` and
`fn.status.workflow`:

- Cluster not known: the workflow status is primary. "Building" (`InfoStatus`)
  plus the spinning icon, "Deploying" (`InfoStatus`) once the run succeeded,
  "Build failed" (`ErrorStatus`, linked to the run), or "Not deployed"
  (`InfoStatus`).
- Cluster known: the cluster badge is primary. "Running" is `SuccessStatus`,
  "Error" is `ErrorStatus` with the `ClusterError` message in a tooltip, and
  "Deploying" / "Scaled to zero" / "Undeploying" are `InfoStatus`.
- Secondary indicator, rendered beside the cluster badge: a spinning
  `RhUiSyncIcon` while the run is in progress, a danger-colored warning icon
  (linked to the run) when it failed, nothing for `Succeeded` or `None`.

The badge and icon choices (`InfoStatus` rather than the originally planned
`ProgressStatus`, `RhUiSyncIcon` rather than a PatternFly `Spinner`) were made to
match the conventions the rest of the OCP console already uses for these states.

## Testing

Conventions per `docs/TESTING.md`.

Backend (Ginkgo/Gomega):

- `scm/github`: `WatchWorkflowRuns` happy path (in_progress, success), failure
  path, change-only emission, per-repo error carry-forward, rediscovery and
  rediscovery failure, missing workflow file, and conditional-request
  revalidation. The intervals themselves are hardcoded, but the tickers that
  drive them come from an injectable factory, so tests step time explicitly
  rather than sleeping; the same applies to the heartbeat in the handler.
- `handler`: build-status frames carry the mapped status; an initial snapshot
  then a new snapshot on change; heartbeats; `X-SCM-Token` required; `app-error`
  frames for pre-stream and mid-stream failures; the stream continues after a
  recoverable error; channel close ends the stream. Driven through
  `scm.ClientStub.OnWatchWorkflowRuns`. A typed SSE event-stream parser in the
  test package turns the raw frames back into DTOs so assertions compare structs,
  not strings.
- `fakegithub`: the Actions endpoints and `/_admin/actions/runs`.

Frontend (Vitest + RTL): stub the network boundary, never `vi.mock` our own
modules. The SSE endpoint is stubbed with MSW via `receiveWorkflowRunEventsStub`,
which serves a `ReadableStream` of SSE frames from either a single snapshot or an
async iterable of events, so a test can drive a sequence of updates and errors.

- `sseEventSource.test.ts`: the transport in isolation. Frame parsing, event
  dispatch, reconnect, close semantics.
- `functionsClient.test.ts`: the wire-event to callback mapping, including the
  401 self-close.
- `FunctionsListPage.test.tsx`: the real hook against the MSW stub alongside the
  K8s watch stub, asserting the merge and the rendered status column. This is
  what catches SSE-payload versus consumer shape drift.

Two shared test doubles support this: `AsyncQueue` (a disposable async iterable
for feeding events and awaiting them deterministically) and `timerFake` (fake
timers scoped by `using`, with `shouldAdvanceTime` so React's microtask
processing keeps working).

E2e (Playwright): run against the **real backend connected to fakegithub**, no
`page.route` mocking (e2e no longer mocks GitHub; it seeds/resets fakegithub via
`/_admin` in `e2e/helpers/fakegithub.ts`). `setWorkflowRun(owner, name, branch,
run)` POSTs to `/_admin/actions/runs`. A test seeds a repo, scripts an
`in_progress` run, loads the list and asserts the status column, then scripts a
`completed`/`failure` run and asserts it updates to "Build failed" with a link to
the run. Because the list streams over SSE the update arrives without a manual
refresh, so the assertions poll with a timeout rather than reloading the page.

## Out of scope

- Faithful workflow execution via `act` (deferred; `/_admin` scripting instead).
- Live streaming on any surface other than the list.
- Granular per-step progress beyond Building/Succeeded/Failed.
- Persisting build history.
