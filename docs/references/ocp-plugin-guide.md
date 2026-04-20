# OCP Dynamic Plugin Guide

Reference for OpenShift Console dynamic plugin mechanics. For styling rules, see `docs/STYLEGUIDE.md`.

## Dynamic Plugin System

This plugin uses webpack module federation to load at runtime into the OpenShift Console. Key files:

- `console-extensions.json`: Declares what the plugin adds to console (routes, nav items, etc.)
- `package.json` `consolePlugin` section: Plugin metadata and exposed modules mapping
- `webpack.config.ts`: Configures module federation and build

**Critical:** Any component referenced in `console-extensions.json` must have a corresponding entry in `package.json` under `consolePlugin.exposedModules`.

## Internationalization (i18n)

**Namespace convention:** `plugin__<plugin-name>` (e.g., `plugin__func-console`)

### In React Components

```tsx
const { t } = useTranslation('plugin__func-console');
return <h1>{t('Hello, World!')}</h1>;
```

### In console-extensions.json

```json
"name": "%plugin__func-console~My Label%"
```

**After adding/changing messages:** Run `yarn i18n` to update locale files in `/locales`.

## Common Development Tasks

### Adding a New Page

1. Create component in `src/components/MyPage.tsx`
2. Add to `package.json` `exposedModules`: `"MyPage": "./components/MyPage"`
3. Add route in `console-extensions.json`:

   ```json
   {
     "type": "console.page/route",
     "properties": {
       "path": "/my-page",
       "component": { "$codeRef": "MyPage" }
     }
   }
   ```

4. Optional: Add nav item in `console-extensions.json`
5. Run `yarn i18n` if you added translatable strings

### Adding a Navigation Item

```json
{
  "type": "console.navigation/href",
  "properties": {
    "id": "my-nav-item",
    "name": "%plugin__func-console~My Page%",
    "href": "/my-page",
    "perspective": "admin",
    "section": "home"
  }
}
```

## Extension Points

See [OpenShift Console Extension Types](https://github.com/openshift/console/blob/main/frontend/packages/console-dynamic-plugin-sdk/docs/console-extensions.md) for all available types:

- `console.page/route` — add new pages
- `console.navigation/href` — add nav items
- `console.navigation/section` — add nav sections
- `console.tab` — add tabs to resource pages
- `console.action/provider` — add actions to resources
- `console.flag` — feature flags

## Constraints & Gotchas

1. **i18n namespace must match ConsolePlugin resource name** with `plugin__` prefix
2. **Module federation requires exact module mapping** — `exposedModules` must match `$codeRef` values
3. **No webpack HMR for extensions** — changes to `console-extensions.json` require restart
4. **React 17, not 18** — matches console's React version
5. **Template, not fork** — the upstream repo is used via "Use this template", not forked
