# Better Testing Server Setup (init.sh) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make init.sh use PID files for process management, support randomized ports for concurrent dev environments, auto-recompile the Go backend on file changes, and update agent docs to reference the new port file.

**Architecture:** init.sh becomes the orchestrator: it allocates ports (default or random), writes `.dev-env.json`, manages processes via PID files in `.dev-pids/`, and spawns an inotifywait watcher for Go auto-recompile. Downstream scripts (start-console.sh, webpack.config.ts) accept ports via CLI args or env vars with backwards-compatible defaults.

**Tech Stack:** Bash, inotifywait (inotify-tools), Go build toolchain, webpack dev server

**Spec:** `docs/superpowers/specs/2026-05-14-better-init-sh-design.md`

---

## File Structure

| File | Responsibility |
|------|---------------|
| `init.sh` | Main orchestrator: port allocation, PID management, backend watcher, `.dev-env.json` output |
| `start-console.sh` | OCP console container launcher, now accepts port CLI args |
| `webpack.config.ts` | Webpack dev server config, now reads `PLUGIN_PORT` env var |
| `.gitignore` | Ignore `.dev-pids/` and `.dev-env.json` |
| `.dockerignore` | Ignore `.dev-pids/` and `.dev-env.json` |
| `CLAUDE.md` | Knowledge base table update |
| `docs/WORKFLOW.md` | Startup sequence update |
| `.claude/commands/init-session.md` | Agent session startup update |

---

### Task 1: Update .gitignore and .dockerignore

**Files:**
- Modify: `.gitignore`
- Modify: `.dockerignore`

- [ ] **Step 1: Add `.dev-pids/` and `.dev-env.json` to `.gitignore`**

Add under the "Local development configuration" section, after the existing `.dev-logs` entry:

```gitignore
.dev-pids
.dev-env.json
```

The section should look like:

```gitignore
# Local development configuration
.dev
.dev-logs
.dev-pids
.dev-env.json
```

- [ ] **Step 2: Add `.dev-pids/` and `.dev-env.json` to `.dockerignore`**

The current `.dockerignore` only has `/bin`. Add the new entries:

```dockerignore
/bin
.dev-logs
.dev-pids
.dev-env.json
```

- [ ] **Step 3: Verify syntax**

Run: `git diff`
Expected: Only the two ignore files changed, no unintended modifications.

- [ ] **Step 4: Commit**

```bash
git add .gitignore .dockerignore
git commit -m "chore: add .dev-pids/ and .dev-env.json to ignore files"
```

---

### Task 2: Rewrite init.sh stop functions to use PID files

Replace the pgrep/pkill-based stop functions with PID-file-based ones. This task only changes the stop side. The start functions still use the old approach until Task 3.

**Files:**
- Modify: `init.sh`

- [ ] **Step 1: Add PID_DIR variable and stop_pid helper**

After the `TIMEOUT=60` line (line 10), add:

```bash
PID_DIR=".dev-pids"
```

After the `wait_for_port` function (after line 26), add a helper function that stops a process by PID file:

```bash
stop_pid() {
  local pidfile="$PID_DIR/$1"
  local label=$2

  if [ ! -f "$pidfile" ]; then
    return
  fi

  local pid
  pid=$(cat "$pidfile")
  if kill -0 "$pid" 2>/dev/null; then
    kill "$pid" 2>/dev/null
    wait "$pid" 2>/dev/null || true
    echo "Stopped $label (PID $pid)."
  fi
  rm -f "$pidfile"
}
```

- [ ] **Step 2: Rewrite stop_backend**

Replace the current `stop_backend` function:

```bash
stop_backend() {
  stop_pid "backend-watcher.pid" "backend watcher"
  stop_pid "backend.pid" "Go backend"
}
```

- [ ] **Step 3: Rewrite stop_plugin**

Replace the current `stop_plugin` function:

```bash
stop_plugin() {
  stop_pid "webpack.pid" "plugin dev server"
}
```

- [ ] **Step 4: Rewrite stop_console**

Replace the current `stop_console` function to use a CID file:

```bash
stop_console() {
  local cidfile="$PID_DIR/console.cid"

  if [ ! -f "$cidfile" ]; then
    return
  fi

  local cid
  cid=$(cat "$cidfile")
  if podman stop "$cid" >/dev/null 2>&1; then
    echo "Stopped OpenShift console (container $cid)."
  fi
  rm -f "$cidfile"
}
```

- [ ] **Step 5: Add cleanup of .dev-env.json to stop_dev**

Update `stop_dev` to also remove the port file:

```bash
stop_dev() {
  stop_backend
  stop_plugin
  stop_console
  rm -f .dev-env.json
}
```

- [ ] **Step 6: Verify syntax**

Run: `bash -n init.sh`
Expected: No output (no syntax errors).

- [ ] **Step 7: Commit**

```bash
git add init.sh
git commit -m "chore: rewrite init.sh stop functions to use PID files"
```

---

### Task 3: Rewrite init.sh start functions to write PID files

Update the start functions to record PIDs and the console container ID.

**Files:**
- Modify: `init.sh`

- [ ] **Step 1: Create .dev-pids/ directory in main**

In the `main` function, change the `mkdir` line from:

```bash
mkdir -p "$LOG_DIR" bin
```

to:

```bash
mkdir -p "$LOG_DIR" "$PID_DIR" bin
```

- [ ] **Step 2: Update start_backend to write PID file**

Replace the current `start_backend` function:

```bash
start_backend() {
  echo "Building Go backend..."
  (cd backend && go build -buildvcs=false -o ../bin/backend .)
  echo "Starting Go backend..."
  ./bin/backend --http-port "$BACKEND_PORT" >>"$LOG_DIR/backend.log" 2>&1 &
  echo $! > "$PID_DIR/backend.pid"
}
```

- [ ] **Step 3: Update start_plugin to write PID file and pass PLUGIN_PORT**

Replace the current `start_plugin` function:

```bash
start_plugin() {
  echo "Starting plugin dev server..."
  PLUGIN_PORT="$PLUGIN_PORT" yarn start >"$LOG_DIR/webpack.log" 2>&1 &
  echo $! > "$PID_DIR/webpack.pid"
}
```

- [ ] **Step 4: Update start_console to use --cidfile and pass ports**

Replace the current `start_console` function:

```bash
start_console() {
  echo "Starting OpenShift console..."
  ./start-console.sh \
    --backend-port "$BACKEND_PORT" \
    --plugin-port "$PLUGIN_PORT" \
    --console-port "$CONSOLE_PORT" \
    >"$LOG_DIR/console.log" 2>&1 &
  echo $! > "$PID_DIR/console.cid"
}
```

Note: This writes the shell PID to console.cid for now. Task 7 will update start-console.sh to accept these arguments. The `--cidfile` approach for the container ID will be wired in Task 7 when start-console.sh is updated. For now, the shell PID allows `stop_console` to kill the process, but we will revise `start_console` and `stop_console` together in Task 7 to use the actual container ID via `--cidfile`.

- [ ] **Step 5: Verify syntax**

Run: `bash -n init.sh`
Expected: No output (no syntax errors).

- [ ] **Step 6: Commit**

```bash
git add init.sh
git commit -m "chore: update init.sh start functions to write PID files"
```

---

### Task 4: Add port randomization and .dev-env.json output

Add the `--randomize-ports` flag, `random_free_port()` helper, and `.dev-env.json` output.

**Files:**
- Modify: `init.sh`

- [ ] **Step 1: Add random_free_port helper**

After the `stop_pid` function, add:

```bash
random_free_port() {
  local port
  while true; do
    port=$((RANDOM % 50001 + 10000))
    if ! bash -c "echo >/dev/tcp/localhost/$port" 2>/dev/null; then
      echo "$port"
      return
    fi
  done
}
```

- [ ] **Step 2: Add write_dev_env function**

After `random_free_port`, add:

```bash
write_dev_env() {
  cat > .dev-env.json <<EOF
{
  "backendPort": $BACKEND_PORT,
  "pluginPort": $PLUGIN_PORT,
  "consolePort": $CONSOLE_PORT
}
EOF
}
```

- [ ] **Step 3: Update argument parsing for --randomize-ports**

Replace the bottom of init.sh (the current `if` block at lines 114-118):

```bash
case "${1:-}" in
  --stop)
    stop_dev
    ;;
  --randomize-ports)
    BACKEND_PORT=$(random_free_port)
    PLUGIN_PORT=$(random_free_port)
    CONSOLE_PORT=$(random_free_port)
    main
    ;;
  "")
    main
    ;;
  *)
    echo "Usage: $0 [--stop | --randomize-ports]"
    exit 1
    ;;
esac
```

- [ ] **Step 4: Call write_dev_env in main**

In the `main` function, add `write_dev_env` after the `mkdir` line and before `check_prerequisites`:

```bash
main() {
  mkdir -p "$LOG_DIR" "$PID_DIR" bin
  write_dev_env
  check_prerequisites
  install_dependencies
  stop_dev
  start_backend
  wait_for_port "$BACKEND_PORT" "Go backend"
  start_plugin
  wait_for_port "$PLUGIN_PORT" "Plugin dev server"
  start_console
  wait_for_port "$CONSOLE_PORT" "OpenShift console"
  print_status
}
```

- [ ] **Step 5: Verify syntax**

Run: `bash -n init.sh`
Expected: No output (no syntax errors).

- [ ] **Step 6: Commit**

```bash
git add init.sh
git commit -m "feat: add --randomize-ports flag and .dev-env.json output to init.sh"
```

---

### Task 5: Update webpack.config.ts to read PLUGIN_PORT env var

**Files:**
- Modify: `webpack.config.ts`

- [ ] **Step 1: Change hardcoded port to env var with fallback**

In `webpack.config.ts`, change line 64:

```typescript
    port: 9001,
```

to:

```typescript
    port: Number(process.env.PLUGIN_PORT) || 9001,
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `yarn webpack --dry-run 2>&1 || true`
Expected: No TypeScript type errors related to the port change.

- [ ] **Step 3: Verify tests still pass**

Run: `yarn test`
Expected: All 112 tests pass (webpack config is not tested directly, but this confirms nothing broke).

- [ ] **Step 4: Commit**

```bash
git add webpack.config.ts
git commit -m "chore: read PLUGIN_PORT env var in webpack dev server config"
```

---

### Task 6: Update start-console.sh to accept port CLI arguments

**Files:**
- Modify: `start-console.sh`

- [ ] **Step 1: Add argument parsing at the top of start-console.sh**

After the `set -euo pipefail` line (line 3) and before the `CONSOLE_IMAGE` line (line 5), add:

```bash
# Parse CLI arguments (all optional, with defaults)
BACKEND_PORT=8080
PLUGIN_PORT=9001

while [[ $# -gt 0 ]]; do
  case $1 in
    --backend-port) BACKEND_PORT="$2"; shift 2 ;;
    --plugin-port) PLUGIN_PORT="$2"; shift 2 ;;
    --console-port) CONSOLE_PORT="$2"; shift 2 ;;
    *) echo "Unknown argument: $1"; exit 1 ;;
  esac
done
```

- [ ] **Step 2: Update CONSOLE_PORT default**

Change line 6 from:

```bash
CONSOLE_PORT=${CONSOLE_PORT:=9000}
```

to:

```bash
CONSOLE_PORT=${CONSOLE_PORT:-9000}
```

This ensures the CLI argument (if set earlier by the arg parser) takes precedence, and the env var is only used as a fallback when no CLI arg was provided. Move this line after the argument parsing block.

- [ ] **Step 3: Replace hardcoded 9001 and 8080 in BRIDGE variables**

Change line 59:

```bash
BRIDGE_PLUGINS="${PLUGIN_NAME}=http://${PLUGIN_HOST}:9001"
```

to:

```bash
BRIDGE_PLUGINS="${PLUGIN_NAME}=http://${PLUGIN_HOST}:${PLUGIN_PORT}"
```

Change line 60. The `BRIDGE_PLUGIN_PROXY` line has embedded `8080`:

```bash
BRIDGE_PLUGIN_PROXY='{"services":[{"consoleAPIPath":"/api/proxy/plugin/'"${PLUGIN_NAME}"'/backend/","endpoint":"http://'"${PLUGIN_HOST}"':8080","authorize":false}]}'
```

to:

```bash
BRIDGE_PLUGIN_PROXY='{"services":[{"consoleAPIPath":"/api/proxy/plugin/'"${PLUGIN_NAME}"'/backend/","endpoint":"http://'"${PLUGIN_HOST}"':'"${BACKEND_PORT}"'","authorize":false}]}'
```

- [ ] **Step 4: Update container port mapping**

In the container network options section, change the `-p` flag lines. Line 51:

```bash
        CONTAINER_NETWORK_OPTS="-p ${CONSOLE_PORT}:9000"
```

stays as-is (already uses `$CONSOLE_PORT`). Same for line 56. No change needed here.

- [ ] **Step 5: Verify syntax**

Run: `bash -n start-console.sh`
Expected: No output (no syntax errors).

- [ ] **Step 6: Commit**

```bash
git add start-console.sh
git commit -m "chore: accept --backend-port, --plugin-port, --console-port in start-console.sh"
```

---

### Task 7: Wire start_console in init.sh to use --cidfile

Now that start-console.sh accepts port arguments (Task 6), update init.sh's `start_console` and `stop_console` to use the container's actual CID via `--cidfile`.

**Files:**
- Modify: `init.sh`
- Modify: `start-console.sh`

- [ ] **Step 1: Add --cidfile to podman run in start-console.sh**

In `start-console.sh`, the `podman run` line (line 69) needs to accept a `--cidfile` argument. Add `--cidfile` to the CLI argument parser (from Task 6):

Add this case to the `while` loop:

```bash
    --cidfile) CIDFILE="$2"; shift 2 ;;
```

And initialize `CIDFILE=""` before the while loop (alongside `BACKEND_PORT` and `PLUGIN_PORT`).

Then update the `podman run` line at the bottom. Change:

```bash
$CONTAINER_CMD run --pull always --platform $CONSOLE_IMAGE_PLATFORM --rm $CONTAINER_NETWORK_OPTS --env-file <(env | grep ^BRIDGE) $CONSOLE_IMAGE
```

to:

```bash
CIDFILE_OPTS=""
if [ -n "$CIDFILE" ]; then
  CIDFILE_OPTS="--cidfile $CIDFILE"
fi

$CONTAINER_CMD run --pull always --platform $CONSOLE_IMAGE_PLATFORM --rm $CIDFILE_OPTS $CONTAINER_NETWORK_OPTS --env-file <(env | grep ^BRIDGE) $CONSOLE_IMAGE
```

- [ ] **Step 2: Update start_console in init.sh to pass --cidfile**

Replace the `start_console` function in init.sh:

```bash
start_console() {
  echo "Starting OpenShift console..."
  ./start-console.sh \
    --backend-port "$BACKEND_PORT" \
    --plugin-port "$PLUGIN_PORT" \
    --console-port "$CONSOLE_PORT" \
    --cidfile "$PID_DIR/console.cid" \
    >"$LOG_DIR/console.log" 2>&1 &
}
```

Note: No PID file for the shell process. The CID file is written by podman directly via `--cidfile`.

- [ ] **Step 3: Update stop_console in init.sh**

The `stop_console` function from Task 2 already reads `.dev-pids/console.cid` and calls `podman stop`. It should work as-is with the actual container ID now being written by podman. No changes needed.

- [ ] **Step 4: Verify syntax of both files**

Run: `bash -n init.sh && bash -n start-console.sh`
Expected: No output (no syntax errors).

- [ ] **Step 5: Commit**

```bash
git add init.sh start-console.sh
git commit -m "chore: wire --cidfile for console container ID tracking"
```

---

### Task 8: Add Go backend auto-recompile watcher

**Files:**
- Modify: `init.sh`

- [ ] **Step 1: Add start_backend_watcher function**

After the `start_backend` function, add:

```bash
start_backend_watcher() {
  if ! command -v inotifywait &>/dev/null; then
    echo "Warning: inotifywait not found. Install inotify-tools for auto-recompile."
    return
  fi

  echo "Starting backend file watcher..."
  (
    while true; do
      inotifywait -r -e modify,create,delete,move --include '\.(go|mod|sum)$' backend/ >/dev/null 2>&1
      sleep 1  # debounce

      echo "[watcher] Detected change, rebuilding backend..."
      if (cd backend && go build -buildvcs=false -o ../bin/backend-tmp .); then
        local old_pid
        old_pid=$(cat "$PID_DIR/backend.pid" 2>/dev/null || true)
        if [ -n "$old_pid" ]; then
          kill "$old_pid" 2>/dev/null || true
          wait "$old_pid" 2>/dev/null || true
        fi
        mv bin/backend-tmp bin/backend
        ./bin/backend --http-port "$BACKEND_PORT" >>"$LOG_DIR/backend.log" 2>&1 &
        echo $! > "$PID_DIR/backend.pid"
        echo "[watcher] Backend restarted (PID $!)."
      else
        echo "[watcher] Build failed. Keeping current backend running."
        rm -f bin/backend-tmp
      fi
    done
  ) >>"$LOG_DIR/backend.log" 2>&1 &
  echo $! > "$PID_DIR/backend-watcher.pid"
}
```

- [ ] **Step 2: Call start_backend_watcher in main**

Add the call after `wait_for_port "$BACKEND_PORT"` and before `start_plugin`:

```bash
main() {
  mkdir -p "$LOG_DIR" "$PID_DIR" bin
  write_dev_env
  check_prerequisites
  install_dependencies
  stop_dev
  start_backend
  wait_for_port "$BACKEND_PORT" "Go backend"
  start_backend_watcher
  start_plugin
  wait_for_port "$PLUGIN_PORT" "Plugin dev server"
  start_console
  wait_for_port "$CONSOLE_PORT" "OpenShift console"
  print_status
}
```

- [ ] **Step 3: Verify syntax**

Run: `bash -n init.sh`
Expected: No output (no syntax errors).

- [ ] **Step 4: Commit**

```bash
git add init.sh
git commit -m "feat: add Go backend auto-recompile watcher using inotifywait"
```

---

### Task 9: Update agent documentation

**Files:**
- Modify: `CLAUDE.md`
- Modify: `docs/WORKFLOW.md`
- Modify: `.claude/commands/init-session.md`

- [ ] **Step 1: Add .dev-env.json and .dev-logs/ to CLAUDE.md knowledge base**

In `CLAUDE.md`, add two rows to the knowledge base table (after the `docs/agent-struggles.json` row):

```markdown
| `.dev-env.json` | Dev server ports (backendPort, pluginPort, consolePort), written by init.sh |
| `.dev-logs/` | Dev server log files (backend.log, webpack.log, console.log) |
```

- [ ] **Step 2: Update WORKFLOW.md startup sequence**

In `docs/WORKFLOW.md`, after step 5 (`Run ./init.sh`), add a new step and renumber subsequent steps:

```markdown
6. Read `.dev-env.json` — note the dev server ports (backend, plugin, console)
7. Run tests — verify app is healthy
8. If broken → fix first. If clean → start [Feature Development Sequence](#feature-development-sequence).
```

(The old step 6 becomes 7, old step 7 becomes 8.)

- [ ] **Step 3: Update init-session.md**

In `.claude/commands/init-session.md`, after step 5 ("Start dev env"), add a new step and renumber:

```markdown
6. **Read ports** — read `.dev-env.json` and note the backend, plugin, and console ports.
7. **Run tests** — run `yarn test` and verify app is healthy.
8. **Wait** — tell the user you're oriented, report the picked feature and which step of the Feature Development Sequence you'd start at. When the user says to proceed, follow the Feature Development Sequence in `docs/WORKFLOW.md` step by step. Do NOT start any work autonomously.
```

Also add `Bash(cat .dev-env.json)` to the `allowed-tools` line at the top:

```
allowed-tools: Bash(git log:*), Bash(pwd), Bash(./init.sh), Bash(yarn test*), Bash(cat .dev-env.json), Read
```

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md docs/WORKFLOW.md .claude/commands/init-session.md
git commit -m "docs: add .dev-env.json and .dev-logs references to agent docs"
```

---

### Task 10: Final verification and cleanup

- [ ] **Step 1: Verify all file syntax**

Run:
```bash
bash -n init.sh && bash -n start-console.sh && echo "Shell syntax OK"
```
Expected: `Shell syntax OK`

- [ ] **Step 2: Run tests**

Run: `yarn test`
Expected: All 112 tests pass.

- [ ] **Step 3: Review full init.sh**

Read `init.sh` end to end. Verify:
- PID_DIR and LOG_DIR variables are at top
- All start_* functions write PID/CID files
- All stop_* functions read PID/CID files
- random_free_port returns ports in 10000-60000 range
- write_dev_env writes valid JSON
- Argument parsing handles `--stop`, `--randomize-ports`, empty, and unknown
- start_backend_watcher builds to backend-tmp, swaps only on success
- print_status shows correct port variables

- [ ] **Step 4: Verify .gitignore and .dockerignore**

Run: `git status`
Expected: No untracked `.dev-pids/` or `.dev-env.json` files showing.

- [ ] **Step 5: Final commit (if any fixups needed)**

If any issues were found in steps 1-4, fix and commit:
```bash
git add -A
git commit -m "chore: fixups from final init.sh verification"
```
