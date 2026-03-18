# OCP Console Dynamic Plugins — Core Concepts Summary

Source: [OCP 4.21 Dynamic Plugins Documentation](https://docs.redhat.com/en/documentation/openshift_container_platform/4.21/html/web_console/dynamic-plugins)

---

## What is a Dynamic Plugin?

A dynamic plugin is a **separately deployed UI bundle** that the OCP Console loads at runtime from a remote HTTP server. It is **decoupled from the Console release cycle** — you ship updates independently.

The plugin runs inside the Console's React app but is loaded dynamically via webpack module federation. The Console discovers plugins through a `ConsolePlugin` custom resource on the cluster.

---

## How It Works

```txt
1. Operator installs → creates Deployment + Service (HTTP server hosting plugin JS/CSS)
2. Operator creates ConsolePlugin CR → tells Console "load plugin from this service"
3. Console fetches plugin-manifest.json from the service
4. Console loads plugin's JS modules at runtime
5. Plugin's extensions (nav items, pages, etc.) appear in the Console UI
```

---

## Key Concepts

### ConsolePlugin CR

The K8s custom resource that registers your plugin with Console:

```yaml
apiVersion: console.openshift.io/v1
kind: ConsolePlugin
metadata:
  name: my-plugin
spec:
  backend:
    service:
      name: my-plugin         # K8s Service name
      namespace: my-plugin     # namespace
      port: 9443               # port
      basePath: /
    type: Service
  displayName: My Plugin
  i18n:
    loadType: Preload          # or Lazy
```

### console-extensions.json

Declares what your plugin contributes to Console. Each entry is an extension with a `type` and `properties`:

```json
[
  {
    "type": "console.navigation/section",
    "properties": {
      "id": "my-section",
      "name": "My Section"
    }
  },
  {
    "type": "console.page/route",
    "properties": {
      "path": "/my-plugin/functions",
      "component": { "$codeRef": "FunctionListPage" }
    }
  }
]
```

### $codeRef

References to React components that are lazy-loaded. Maps to `exposedModules` in `package.json`:

```json
"consolePlugin": {
  "name": "my-plugin",
  "exposedModules": {
    "FunctionListPage": "./views/FunctionListPage"
  }
}
```

---

## Extension Types We Need (for FaaS PoC)

| Extension Type | Purpose | Required? |
|---------------|---------|-----------|
| `console.navigation/section` | Add "Functions" section to left nav | ✅ Yes |
| `console.navigation/href` | Add nav items (Functions list) | ✅ Yes |
| `console.page/route` | Register pages (list, create, editor) | ✅ Yes |
| `console.flag/model` | Feature flag based on CRD presence | ⚠️ Maybe |

---

## SDK APIs We Need

### Data & K8s

| API | Purpose | How we use it |
|-----|---------|---------------|
| `useK8sWatchResource` | Watch K8s resources reactively | List deployed functions from cluster |
| `k8sListResource` | List K8s resources (imperative) | Alternative to watch hook |
| `useActiveNamespace` | Get/set active namespace | Scope function listing |
| `consoleFetch` / `consoleFetchJSON` | Make HTTP requests with Console headers | Proxy requests if needed |

### UI Components

| API | Purpose | How we use it |
|-----|---------|---------------|
| `ListPageHeader` | Page header with title | Functions list page header |
| `CodeEditor` | Monaco-based code editor (lazy loaded) | Function editor view |
| `ErrorStatus` | Error status popover | Show function error state |
| `ProgressStatus` | In-progress status popover | Show deploying state |
| `InfoStatus` | Info status popover | Show function info |
| `SuccessStatus` | Success status popover | Show running/healthy state |
| `ErrorBoundaryFallbackPage` | Full-page error display | Catch unexpected errors in our views |
| `useDeleteModal` | Delete confirmation modal | Delete function action |

> **Note:** `VirtualizedTable` and `ListPageFilter` are deprecated in SDK 4.21.
> Use PatternFly's [Data view](https://www.patternfly.org/extensions/data-view/overview/) instead for table rendering.

---

## Plugin Service Proxy

If your plugin needs to talk to an in-cluster backend service, declare it in `ConsolePlugin.spec.proxy`:

```yaml
spec:
  proxy:
    - alias: my-backend
      authorization: UserToken    # passes user's OCP token
      endpoint:
        service:
          name: my-backend-service
          namespace: my-namespace
          port: 8080
        type: Service
```

Then call from JS: `/api/proxy/plugin/my-plugin/my-backend/endpoint`

This is relevant if we go with Approach C (thin Go backend).

---

## Development Workflow

```txt
Terminal 1: yarn install && yarn run start     # builds + serves plugin on localhost:9001
Terminal 2: oc login && yarn run start-console  # runs OCP Console in container, loads your plugin
```

Visit `http://localhost:9000` to see Console with your plugin loaded.

---

## Deployment

1. Build Docker image: `docker build -t quay.io/my-repo/my-plugin:latest .`
2. Push image: `docker push quay.io/my-repo/my-plugin:latest`
3. Deploy via Helm: `helm upgrade -i my-plugin charts/openshift-console-plugin -n my-plugin-ns --create-namespace --set plugin.image=quay.io/my-repo/my-plugin:latest`

---

## Guidelines

- Use **PatternFly 6** (for OCP 4.19+)
- Use **Webpack 5**
- Use **Yarn** for package management
- Prefix CSS classes with plugin name (e.g., `func-console__heading`)
- Use `useTranslation` hook with `plugin__<plugin-name>` namespace for i18n
- Use `consoleFetch` for HTTP requests (not raw `fetch`)
- Do not import PatternFly CSS directly — Console loads base styles
