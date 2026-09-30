import { useEffect, useState } from 'react';
import {
  BuildStatusEventSource,
  WorkflowRunMap,
  createBuildStatusEventSource,
} from './functionsClient';

// useBuildStatus streams GitHub Actions build status over SSE, keyed by
// "owner/repo". Pass the auth connectionId so the stream tears down and
// reconnects with the current PAT on in-place login and account switch.
// If connectionId is undefined, no stream is created (unauthenticated).
export function useBuildStatus(
  connectionId?: number,
  eventSource?: BuildStatusEventSource,
): { statuses: Readonly<WorkflowRunMap>; error?: string } {
  const [statuses, setStatuses] = useState<WorkflowRunMap>({});
  const [error, setError] = useState<string>();

  useEffect(() => {
    if (connectionId === undefined) return;

    const es: BuildStatusEventSource = eventSource ?? createBuildStatusEventSource();

    es.addEventListener('build-status', (e) => {
      try {
        const snap = JSON.parse(e.data) as WorkflowRunMap;
        setStatuses(snap);
      } catch {
        setError('Invalid build status data');
      }
    });

    es.addEventListener('error', (e) => {
      try {
        const error = JSON.parse(e.data) as { message: string };
        setError(error.message);
      } catch {
        setError('Unknown error');
      }
    });

    es.addEventListener('open', () => {
      // clear the error on the re-connect
      setError(undefined);
    });

    return () => {
      es.close();
    };
  }, [eventSource, connectionId]);

  return { statuses, error };
}
