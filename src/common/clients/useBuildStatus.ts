import { consoleFetch } from '@openshift-console/dynamic-plugin-sdk';
import { useEffect, useState } from 'react';
import { BuildStatus, PAT_KEY, PROXY_BASE } from '../types';

const RECONNECT_DELAY_MS = 3000;

// How long a tab may stay hidden before its stream is torn down. A short glance
// at another tab should not churn the connection, because every reconnect
// re-runs repo discovery through GitHub's search API, which has a far lower rate
// limit than the rest of the REST API.
const HIDDEN_GRACE_MS = 30_000;

interface BuildStatusItem {
  buildStatus: BuildStatus['buildStatus'];
  conclusion?: string;
  runURL?: string;
}

interface BuildSnapshot {
  // Keyed by "owner/repo", the identifier a function is correlated on.
  functions: Record<string, BuildStatusItem>;
}

// useBuildStatus streams GitHub Actions build status over SSE, keyed by
// "owner/repo". Pass the auth connectionId so the stream tears down and
// reconnects with the current PAT on in-place login and account switch. The
// stream is paused while the tab sits in the background.
export function useBuildStatus(connectionId = 0): ReadonlyMap<string, BuildStatus> {
  const [statuses, setStatuses] = useState<ReadonlyMap<string, BuildStatus>>(() => new Map());
  const tabActive = useTabActive();

  useEffect(() => {
    // Nothing to stream to. Aborting the request does more than stop reading:
    // it cancels the request context, which ends the server-side watch and its
    // per-repo GitHub polling, so a forgotten tab costs no API budget.
    //
    // The last snapshot stays rendered rather than being cleared. It is still
    // the best available answer, and resuming refreshes it within one poll
    // round-trip, because the backend emits a snapshot on connect before its
    // first tick.
    if (!tabActive) return;

    let cancelled = false;
    const controller = new AbortController();

    async function run() {
      while (!cancelled) {
        const pat = sessionStorage.getItem(PAT_KEY);
        if (!pat) return;
        try {
          const res = await consoleFetch(
            `${PROXY_BASE}/api/v1/func/build/watch`,
            {
              headers: { 'X-SCM-Token': pat },
              signal: controller.signal,
            },
            0, // no timeout; the default ~60s would abort this long-lived stream
          );
          // A 2xx with no body is unexpected; reconnect rather than stop.
          if (res.body) {
            await readStream(res.body, (snap) => {
              if (!cancelled) setStatuses(toMap(snap));
            });
          }
        } catch (err) {
          if (cancelled) return;
          if (isAuthError(err)) {
            // A bad or expired PAT will not recover on retry, so stop rather
            // than reconnect in a tight loop.
            console.error(
              'useBuildStatus: build status stream unauthorized, not reconnecting',
              err,
            );
            return;
          }
          console.error('useBuildStatus: build status stream error, reconnecting', err);
        }
        // Stream ended or errored transiently; back off, then reconnect.
        await delay(RECONNECT_DELAY_MS, controller.signal);
      }
    }

    run();
    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [connectionId, tabActive]);

  return statuses;
}

// useTabActive reports whether the tab is worth streaming to: true while it is
// visible, false once it has been hidden for HIDDEN_GRACE_MS. Visibility only
// covers backgrounded tabs and minimized windows; navigating away within the
// console unmounts the consumer, which tears the stream down already.
function useTabActive(): boolean {
  const [active, setActive] = useState(() => document.visibilityState !== 'hidden');

  useEffect(() => {
    let graceTimer: ReturnType<typeof setTimeout> | undefined;

    const onVisibilityChange = () => {
      // Coming back before the grace period elapses cancels the pending
      // teardown, so the stream is never interrupted.
      clearTimeout(graceTimer);
      if (document.visibilityState === 'hidden') {
        graceTimer = setTimeout(() => setActive(false), HIDDEN_GRACE_MS);
      } else {
        setActive(true);
      }
    };

    document.addEventListener('visibilitychange', onVisibilityChange);
    return () => {
      clearTimeout(graceTimer);
      document.removeEventListener('visibilitychange', onVisibilityChange);
    };
  }, []);

  return active;
}

async function readStream(
  body: ReadableStream<Uint8Array>,
  onSnapshot: (snap: BuildSnapshot) => void,
): Promise<void> {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  for (;;) {
    const { done, value } = await reader.read();
    if (done) return;
    buffer += decoder.decode(value, { stream: true });
    let idx: number;
    while ((idx = buffer.indexOf('\n\n')) !== -1) {
      const frame = buffer.slice(0, idx);
      buffer = buffer.slice(idx + 2);
      const snap = parseFrame(frame);
      if (snap) onSnapshot(snap);
    }
  }
}

function parseFrame(frame: string): BuildSnapshot | null {
  let event = '';
  const dataLines: string[] = [];
  for (const line of frame.split('\n')) {
    if (line.startsWith(':')) continue; // heartbeat / comment
    if (line.startsWith('event:')) event = line.slice('event:'.length).trim();
    else if (line.startsWith('data:')) dataLines.push(line.slice('data:'.length).trim());
  }
  if (event !== 'build-status') return null;
  if (dataLines.length === 0) return null;
  try {
    return JSON.parse(dataLines.join('\n')) as BuildSnapshot;
  } catch {
    return null;
  }
}

function toMap(snap: BuildSnapshot): ReadonlyMap<string, BuildStatus> {
  return new Map(
    Object.entries(snap.functions ?? {}).map(([key, f]) => [
      key,
      {
        buildStatus: f.buildStatus,
        conclusion: f.conclusion,
        runURL: f.runURL,
      },
    ]),
  );
}

// consoleFetch throws an HttpError carrying the status on `code`; `response.status`
// is a defensive fallback that avoids depending on the SDK error class at runtime.
function isAuthError(err: unknown): boolean {
  if (typeof err !== 'object' || err === null) return false;
  const e = err as { code?: number; response?: { status?: number } };
  const status = e.code ?? e.response?.status;
  return status === 401 || status === 403;
}

function delay(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    if (signal.aborted) return resolve();
    const onAbort = () => {
      clearTimeout(id);
      resolve();
    };
    const id = setTimeout(() => {
      signal.removeEventListener('abort', onAbort);
      resolve();
    }, ms);
    signal.addEventListener('abort', onAbort, { once: true });
  });
}
