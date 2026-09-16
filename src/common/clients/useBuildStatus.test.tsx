import { describe, it, expect, vi } from 'vitest';

vi.mock('@openshift-console/dynamic-plugin-sdk', () => ({
  consoleFetch: vi.fn(),
}));

import { renderHook, waitFor } from '@testing-library/react';
import { useBuildStatus } from './useBuildStatus';

interface BuildSnapshotEvent {
  readonly data: string;
}

interface ErrorEvent {
  readonly message: string;
  readonly isAuthError: boolean;
}

interface BuildStatusEventSource {
  addEventListener(
    event: 'build-status' | 'error' | 'open',
    cbk: ((e: BuildSnapshotEvent) => void) | ((e: ErrorEvent) => void) | (() => void),
  ): void;
  close(): void;
}

describe('useBuildStatus', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  function createEventSourceMethods(listeners: Array<(e: BuildSnapshotEvent) => void>) {
    const errorListeners: Array<(e: ErrorEvent) => void> = [];
    const openListeners: Array<() => void> = [];
    return {
      addEventListener(
        event: 'build-status' | 'error' | 'open',
        cbk: ((e: BuildSnapshotEvent) => void) | ((e: ErrorEvent) => void) | (() => void),
      ) {
        if (event === 'build-status') {
          listeners.push(cbk as (e: BuildSnapshotEvent) => void);
        } else if (event === 'error') {
          errorListeners.push(cbk as (e: ErrorEvent) => void);
        } else if (event === 'open') {
          openListeners.push(cbk as () => void);
        }
      },
      close() {
        listeners.length = 0;
      },
      errorListeners,
      openListeners,
    };
  }

  function createStubEventSource(
    snapshots: Array<{ functions: Record<string, unknown> }> = [],
  ): BuildStatusEventSource {
    const listeners: Array<(e: BuildSnapshotEvent) => void> = [];

    // Emit all snapshots immediately
    setTimeout(() => {
      snapshots.forEach((snap) => {
        listeners.forEach((cbk) => cbk({ data: JSON.stringify(snap) }));
      });
    }, 0);

    return createEventSourceMethods(listeners);
  }

  function createSequentialStubEventSource(
    snapshots: Array<{ functions: Record<string, unknown> }> = [],
  ): BuildStatusEventSource {
    const listeners: Array<(e: BuildSnapshotEvent) => void> = [];

    // Emit snapshots sequentially over time
    snapshots.forEach((snap, idx) => {
      setTimeout(
        () => {
          listeners.forEach((cbk) => cbk({ data: JSON.stringify(snap) }));
        },
        (idx + 1) * 10,
      );
    });

    return createEventSourceMethods(listeners);
  }

  function createContinuousStubEventSource(): {
    eventSource: BuildStatusEventSource;
    emitSnapshot: (snap: { functions: Record<string, unknown> }) => void;
  } {
    const listeners: Array<(e: BuildSnapshotEvent) => void> = [];
    let closed = false;

    return {
      eventSource: {
        ...createEventSourceMethods(listeners),
        close() {
          closed = true;
          listeners.length = 0;
        },
      },
      emitSnapshot(snap: { functions: Record<string, unknown> }) {
        if (!closed) {
          listeners.forEach((cbk) => cbk({ data: JSON.stringify(snap) }));
        }
      },
    };
  }

  it('parses a build-status frame into a keyed map', async () => {
    const eventSource = createStubEventSource([
      {
        functions: {
          'alice/fn': { buildStatus: 'Building' },
          'alice/gn': { buildStatus: 'Failed', runURL: 'u' },
        },
      },
    ]);

    const { result } = renderHook(() => useBuildStatus(0, eventSource));

    await waitFor(() => expect(Object.keys(result.current.statuses).length).toBe(2));
    expect(result.current.statuses['alice/fn']?.buildStatus).toBe('Building');
    expect(result.current.statuses['alice/gn']?.buildStatus).toBe('Failed');
    expect(result.current.statuses['alice/gn']?.runURL).toBe('u');
  });

  it('closes the stream on unmount, stopping updates', async () => {
    const { eventSource, emitSnapshot } = createContinuousStubEventSource();

    const { result, unmount } = renderHook(() => useBuildStatus(0, eventSource));

    emitSnapshot({ functions: { 'a/b': { buildStatus: 'Building' } } });
    await waitFor(() => expect(Object.keys(result.current.statuses).length).toBe(1));

    unmount();

    emitSnapshot({ functions: { 'c/d': { buildStatus: 'Succeeded' } } });

    expect(Object.keys(result.current.statuses).length).toBe(1);
  });

  it('updates state when event source emits', async () => {
    const { eventSource, emitSnapshot } = createContinuousStubEventSource();
    const { result } = renderHook(() => useBuildStatus(0, eventSource));

    emitSnapshot({
      functions: {
        'bob/repo': { buildStatus: 'Succeeded', conclusion: 'success' },
      },
    });

    await waitFor(() => expect(Object.keys(result.current.statuses).length).toBe(1));
    expect(result.current.statuses['bob/repo']?.buildStatus).toBe('Succeeded');
  });

  it('updates state multiple times as snapshots arrive over time', async () => {
    vi.useFakeTimers();
    const eventSource = createSequentialStubEventSource([
      { functions: { 'x/y': { buildStatus: 'Building' } } },
      { functions: { 'x/y': { buildStatus: 'Succeeded' } } },
    ]);

    const { result } = renderHook(() => useBuildStatus(0, eventSource));

    await vi.advanceTimersByTimeAsync(15);
    expect(result.current.statuses['x/y']?.buildStatus).toBe('Building');

    await vi.advanceTimersByTimeAsync(15);
    expect(result.current.statuses['x/y']?.buildStatus).toBe('Succeeded');
  });

  it('closes the stream when connectionId changes', async () => {
    const { eventSource, emitSnapshot } = createContinuousStubEventSource();
    let closeCalled = false;
    const originalClose = eventSource.close.bind(eventSource);
    eventSource.close = () => {
      closeCalled = true;
      originalClose();
    };

    const { result, rerender } = renderHook(({ connId }) => useBuildStatus(connId, eventSource), {
      initialProps: { connId: 0 },
    });

    emitSnapshot({ functions: { 'a/b': { buildStatus: 'Building' } } });
    await waitFor(() => expect(Object.keys(result.current.statuses).length).toBe(1));

    rerender({ connId: 1 });

    expect(closeCalled).toBe(true);
  });

  it('captures error events from the event source', async () => {
    const { eventSource, emitError } = createErrorCapturingStubEventSource();
    const { result } = renderHook(() => useBuildStatus(0, eventSource));

    emitError({ message: 'Connection failed' } as ErrorEvent);

    await waitFor(() => expect(result.current.error).toBe('Connection failed'));
  });

  it('preserves statuses while error is present', async () => {
    const { eventSource, emitSnapshot, emitError } = createErrorCapturingStubEventSource();
    const { result } = renderHook(() => useBuildStatus(0, eventSource));

    emitSnapshot({ functions: { 'repo/owner': { buildStatus: 'Building' } } });
    await waitFor(() => expect(Object.keys(result.current.statuses).length).toBe(1));

    emitError({ message: 'Network error' } as ErrorEvent);
    await waitFor(() => expect(result.current.error).toBe('Network error'));

    // Statuses should still be present
    expect(result.current.statuses['repo/owner']?.buildStatus).toBe('Building');
  });

  it('clears error when open event is emitted', async () => {
    const { eventSource, emitError, emitOpen } = createErrorCapturingStubEventSource();
    const { result } = renderHook(() => useBuildStatus(0, eventSource));

    emitError({ message: 'Connection failed' } as ErrorEvent);
    await waitFor(() => expect(result.current.error).toBe('Connection failed'));

    emitOpen();
    await waitFor(() => expect(result.current.error).toBeUndefined());
  });

  it('does not set up stream when connectionId is undefined', () => {
    const eventSource = {
      addEventListener: vi.fn(),
      close: vi.fn(),
    } as unknown as BuildStatusEventSource;

    const { result } = renderHook(() => useBuildStatus(undefined, eventSource));

    // Event source should not be registered with
    expect(eventSource.addEventListener).not.toHaveBeenCalled();
    expect(eventSource.close).not.toHaveBeenCalled();

    // Hook returns empty state
    expect(result.current.statuses).toEqual({});
    expect(result.current.error).toBeUndefined();
  });

  it('sets error when build-status event data is malformed JSON', async () => {
    const { eventSource, emitSnapshot, emitRaw } = createErrorCapturingStubEventSource();
    const { result } = renderHook(() => useBuildStatus(0, eventSource));

    emitSnapshot({ functions: { 'a/b': { buildStatus: 'Building' } } });
    await waitFor(() => expect(Object.keys(result.current.statuses).length).toBe(1));

    emitRaw('invalid json data');
    await waitFor(() => expect(result.current.error).toBe('Invalid build status data'));

    // Statuses are preserved
    expect(Object.keys(result.current.statuses).length).toBe(1);
  });

  function createErrorCapturingStubEventSource(): {
    eventSource: BuildStatusEventSource;
    emitSnapshot: (snap: { functions: Record<string, unknown> }) => void;
    emitError: (err: ErrorEvent) => void;
    emitOpen: () => void;
    emitRaw: (data: string) => void;
  } {
    const listeners: Array<(e: BuildSnapshotEvent) => void> = [];
    const errorListeners: Array<(e: ErrorEvent) => void> = [];
    const openListeners: Array<() => void> = [];
    let closed = false;

    return {
      eventSource: {
        addEventListener(
          event: 'build-status' | 'error' | 'open',
          cbk: ((e: BuildSnapshotEvent) => void) | ((e: ErrorEvent) => void) | (() => void),
        ) {
          if (event === 'build-status') {
            listeners.push(cbk as (e: BuildSnapshotEvent) => void);
          } else if (event === 'error') {
            errorListeners.push(cbk as (e: ErrorEvent) => void);
          } else if (event === 'open') {
            openListeners.push(cbk as () => void);
          }
        },
        close() {
          closed = true;
          listeners.length = 0;
        },
      },
      emitSnapshot(snap: { functions: Record<string, unknown> }) {
        if (!closed) {
          listeners.forEach((cbk) => cbk({ data: JSON.stringify(snap) }));
        }
      },
      emitError(err: ErrorEvent) {
        if (!closed) {
          errorListeners.forEach((cbk) => cbk(err));
        }
      },
      emitOpen() {
        if (!closed) {
          openListeners.forEach((cbk) => cbk());
        }
      },
      emitRaw(data: string) {
        if (!closed) {
          listeners.forEach((cbk) => cbk({ data }));
        }
      },
    };
  }
});
