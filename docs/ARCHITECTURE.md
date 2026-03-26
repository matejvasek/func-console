# Architecture — func-console

## Stack

React + TypeScript + PatternFly 6 + OCP Dynamic Plugin SDK

## Layered Architecture

```mermaid
flowchart TB
    TYPES[Types] ---|cross-cutting| UTILS[Utils]
    SERVICES[Services] ---|cross-cutting| UTILS
    COMPONENTS[Components] ---|cross-cutting| UTILS
    VIEWS[Views] --> COMPONENTS[Components]
    VIEWS --> HOOKS[Hooks]
    COMPONENTS --> HOOKS
    COMPONENTS --> TYPES
    HOOKS --> SERVICES[Services]
    HOOKS --> TYPES
    SERVICES --> TYPES[Types]
```

Arrows mean "imports / depends on."

| Layer | Maps to | Depends on |
|-------|---------|------------|
| **Types** | `services/types.ts` | nothing |
| **Services** | `services/*/Service.ts` + implementations | Types, Utils |
| **Hooks** | `services/*/use*.ts` — wiring layer | Services, Types, Utils |
| **Components** | `components/` — FunctionTable, CreateForm, etc. | Hooks, Types, Utils |
| **Views** | `views/` — page-level components | Components, Hooks, Utils |
| **Utils** | `utils/` — constants, helpers | nothing (cross-cutting) |

### Dependency Rules

- Unidirectional: Types <- Services <- Hooks <- Components <- Views
- Utils can be imported by any layer
- Views never import Services directly (always through Hooks)
- Services never import Components or Views
- No circular dependencies

## Architectural Guidance

- PatternFly components preferred over custom HTML
- Error handling through ErrorProvider/addError pattern
- Shared utilities in `utils/`, not hand-rolled per component
- Services consumed through hooks, never imported directly
