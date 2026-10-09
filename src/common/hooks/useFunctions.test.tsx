import { renderHook, waitFor } from '@testing-library/react';
import { listFunctionsStub } from '../testing/functionsClientStub';
import { server } from '../testing/mswServer';
import { repoListItem } from '../testing/testData';
import { useFunctions } from './useFunctions';

// vi.mock is hoisted above imports, so regular imports aren't available in the factory.
// vi.hoisted runs before vi.mock, making the sdkTestDoubles available to the factory.
// https://vitest.dev/api/vi.html#vi-hoisted
const sdkTestDoubles = await vi.hoisted(async () => import('../../common/testing/sdkTestDoubles'));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock('@openshift-console/dynamic-plugin-sdk', () => ({
  consoleFetch: sdkTestDoubles.consoleFetchFake,
  consoleFetchJSON: sdkTestDoubles.consoleFetchJSONFake,
  useK8sWatchResource: sdkTestDoubles.useK8sWatchResourceStub,
  isAllNamespacesKey: sdkTestDoubles.isAllNamespaceKeyFake,
}));

describe('useFunctions', () => {
  const namespace = 'demo';

  afterEach(() => {
    sdkTestDoubles.reset();
    server.resetHandlers();
  });

  describe('Function[] after fetching metadata', () => {
    it('builds initial Function[] from function metadata list', async () => {
      listFunctionsStub({ responses: [repoListItem({ repoName: 'my-func' })] });

      const { result } = renderHook(() => useFunctions(namespace, 0));

      await waitFor(() => expect(result.current.loaded).toBe(true));
      expect(result.current.functions).toHaveLength(1);

      const fn = result.current.functions[0];
      expect(fn.name).toBe('my-func');
      expect(fn.namespace).toBe(namespace);
      expect(fn.status.cluster.status).toBe('NotDeployed');
      expect(fn.status.workflow.status).toBe('None');
      expect(fn.url).toBeUndefined();
      expect(fn.replicas).toBeUndefined();
      expect(fn.mainResource).toBeUndefined();
    });

    it('builds multiple functions from function metadata list', async () => {
      listFunctionsStub({
        responses: [
          repoListItem({ repoName: 'func-a', name: 'func-a', namespace, runtime: 'node' }),
          repoListItem({ repoName: 'func-b', name: 'func-b', namespace, runtime: 'go' }),
        ],
      });

      const { result } = renderHook(() => useFunctions(namespace, 0));

      await waitFor(() => expect(result.current.loaded).toBe(true));
      expect(result.current.functions).toHaveLength(2);
      expect(result.current.functions[0].name).toBe('func-a');
      expect(result.current.functions[1].name).toBe('func-b');
    });

    it('reports loaded when list fetch completes', async () => {
      listFunctionsStub({ responses: [repoListItem({ repoName: 'my-func' })] });

      const { result } = renderHook(() => useFunctions(namespace, 0));

      await waitFor(() => expect(result.current.loaded).toBe(true));
      expect(result.current.functions).toHaveLength(1);
    });

    it('reports loaded with empty functions when connectionId is undefined', () => {
      const { result } = renderHook(() => useFunctions(namespace, undefined));

      expect(result.current.loaded).toBe(true);
      expect(result.current.functions).toHaveLength(0);
    });

    it('surfaces list fetch error', async () => {
      listFunctionsStub({
        errorResponse: { message: 'server error', status: 500 },
      });

      const { result } = renderHook(() => useFunctions(namespace, 0));

      await waitFor(() => expect(result.current.loaded).toBe(true));
      expect(result.current.errors).toHaveLength(1);
      expect(result.current.errors![0]).toContain('server error');
    });

    it('returns empty errors when list fetch succeeds', async () => {
      listFunctionsStub({ responses: [repoListItem({ repoName: 'my-func' })] });

      const { result } = renderHook(() => useFunctions(namespace, 0));

      await waitFor(() => expect(result.current.loaded).toBe(true));
      expect(result.current.errors).toHaveLength(0);
    });

    it('resets functions when namespace changes', async () => {
      listFunctionsStub({ responses: [repoListItem({ repoName: 'my-func' })] });

      const { result, rerender } = renderHook(({ ns, connId }) => useFunctions(ns, connId), {
        initialProps: { ns: namespace, connId: 0 },
      });

      await waitFor(() => expect(result.current.functions).toHaveLength(1));

      listFunctionsStub({
        responses: [
          repoListItem({ repoName: 'other-func', name: 'other-func', namespace: 'prod' }),
        ],
      });
      rerender({ ns: 'prod', connId: 0 });

      await waitFor(() => {
        expect(result.current.functions).toHaveLength(1);
        expect(result.current.functions[0].name).toBe('other-func');
      });
    });

    it('resets functions when connectionId changes', async () => {
      listFunctionsStub({ responses: [repoListItem({ repoName: 'my-func' })] });

      const { result, rerender } = renderHook(({ ns, connId }) => useFunctions(ns, connId), {
        initialProps: { ns: namespace, connId: 0 },
      });

      await waitFor(() => expect(result.current.functions).toHaveLength(1));

      listFunctionsStub({ responses: [] });
      rerender({ ns: namespace, connId: 1 });

      await waitFor(() => expect(result.current.functions).toHaveLength(0));
    });

    it('does not fetch when connectionId is undefined', () => {
      const { result } = renderHook(() => useFunctions(namespace, undefined));

      expect(result.current.loaded).toBe(true);
      expect(result.current.functions).toHaveLength(0);
      expect(result.current.errors).toHaveLength(0);
    });

    it('re-fetches functions when refreshKey changes', async () => {
      listFunctionsStub({ responses: [repoListItem({ repoName: 'my-func' })] });

      const { result, rerender } = renderHook(
        ({ ns, connId, refresh }) => useFunctions(ns, connId, refresh),
        { initialProps: { ns: namespace, connId: 0, refresh: 0 } },
      );

      await waitFor(() => expect(result.current.functions).toHaveLength(1));

      listFunctionsStub({
        responses: [repoListItem({ repoName: 'my-func' }), repoListItem({ repoName: 'new-func' })],
      });
      rerender({ ns: namespace, connId: 0, refresh: 1 });

      await waitFor(() => expect(result.current.functions).toHaveLength(2));
      expect(result.current.functions[1].name).toBe('new-func');
    });

    it('does not reset state when refreshKey changes', async () => {
      listFunctionsStub({ responses: [repoListItem({ repoName: 'my-func' })] });

      const { result, rerender } = renderHook(
        ({ ns, connId, refresh }) => useFunctions(ns, connId, refresh),
        { initialProps: { ns: namespace, connId: 0, refresh: 0 } },
      );

      await waitFor(() => expect(result.current.functions).toHaveLength(1));

      let continueWithRequest = () => {};
      listFunctionsStub({
        responses: [repoListItem({ repoName: 'my-func' }), repoListItem({ repoName: 'new-func' })],
        wait: new Promise<void>((r) => {
          continueWithRequest = r;
        }),
      });

      rerender({ ns: namespace, connId: 0, refresh: 1 });

      // Functions should still be visible while re-fetch is in flight (no reset to empty)
      expect(result.current.functions).toHaveLength(1);

      continueWithRequest();
      await waitFor(() => expect(result.current.functions).toHaveLength(2));
    });
  });
});
