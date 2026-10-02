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
        setError(undefined); // clear the recoverable error, if any
      } catch {
        setError('Invalid build status data');
      }
    });

    es.addEventListener('app-error', (e) => {
      try {
        const error = JSON.parse(e.data) as { message: string; isAuthError: boolean };
        setError(error.message);
        if (error.isAuthError) es.close(); // non-recoverable error close EventSource
      } catch {
        setError('Unknown error');
      }
    });

    es.addEventListener('error', () => {
      setError('Unknown error');
    });

    return () => {
      es.close();
    };
  }, [eventSource, connectionId]);

  return { statuses, error };
}
