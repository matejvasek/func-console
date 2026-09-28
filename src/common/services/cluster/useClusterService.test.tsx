import { render, screen } from '@testing-library/react';
import { useClusterService } from './useClusterService';

const mockUseK8sWatchResource = vi.fn();

vi.mock('@openshift-console/dynamic-plugin-sdk', () => ({
  useK8sWatchResource: (...args: unknown[]) => mockUseK8sWatchResource(...args),
}));

const mockKsvc = {
  apiVersion: 'serving.knative.dev/v1',
  kind: 'Service',
  metadata: {
    name: 'my-func',
    namespace: 'demo',
    labels: { 'function.knative.dev/name': 'my-func' },
  },
  status: {
    url: 'https://my-func-demo.apps.example.com',
    latestReadyRevisionName: 'my-func-00001',
    conditions: [{ type: 'Ready', status: 'True' }],
  },
};

const mockDeployment = {
  apiVersion: 'apps/v1',
  kind: 'Deployment',
  metadata: {
    name: 'my-func-00001-deployment',
    namespace: 'demo',
    labels: {
      'function.knative.dev/name': 'my-func',
      'serving.knative.dev/revision': 'my-func-00001',
    },
  },
  spec: { replicas: 1 },
  status: { readyReplicas: 1 },
};

function TestConsumer({ functionNames = [] }: { functionNames?: string[] }) {
  const { knativeServices, deployments, loaded, error } = useClusterService(functionNames);
  return (
    <>
      <span data-testid="loaded">{String(loaded)}</span>
      <span data-testid="error">{String(error)}</span>
      <span data-testid="ksvc-count">{knativeServices.length}</span>
      <span data-testid="dep-count">{deployments.length}</span>
      {knativeServices.map((s) => (
        <span key={s.metadata?.name} data-testid="ksvc">
          {s.metadata?.name}
        </span>
      ))}
      {deployments.map((d) => (
        <span key={d.metadata?.name} data-testid="deployment">
          {d.metadata?.name}
        </span>
      ))}
    </>
  );
}

describe('useClusterService', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('passes null config when function names are empty', () => {
    mockUseK8sWatchResource.mockReturnValue([[], true, null]);

    render(<TestConsumer />);

    expect(mockUseK8sWatchResource).toHaveBeenCalledWith(null);
    expect(screen.getByTestId('loaded')).toHaveTextContent('true');
    expect(screen.getByTestId('ksvc-count')).toHaveTextContent('0');
    expect(screen.getByTestId('dep-count')).toHaveTextContent('0');
  });

  it('watches Knative Services with In selector for given function names', () => {
    mockUseK8sWatchResource
      .mockReturnValueOnce([[mockKsvc], true, null])
      .mockReturnValueOnce([[], true, null]);

    render(<TestConsumer functionNames={['my-func']} />);

    expect(mockUseK8sWatchResource).toHaveBeenCalledWith({
      groupVersionKind: { group: 'serving.knative.dev', version: 'v1', kind: 'Service' },
      isList: true,
      selector: {
        matchExpressions: [
          { key: 'function.knative.dev/name', operator: 'In', values: ['my-func'] },
        ],
      },
    });
    expect(screen.getByTestId('ksvc-count')).toHaveTextContent('1');
    expect(screen.getByTestId('ksvc')).toHaveTextContent('my-func');
  });

  it('watches Deployments with In selector for given function names', () => {
    mockUseK8sWatchResource
      .mockReturnValueOnce([[], true, null])
      .mockReturnValueOnce([[mockDeployment], true, null]);

    render(<TestConsumer functionNames={['my-func']} />);

    expect(mockUseK8sWatchResource).toHaveBeenCalledWith({
      groupVersionKind: { group: 'apps', version: 'v1', kind: 'Deployment' },
      isList: true,
      selector: {
        matchExpressions: [
          { key: 'function.knative.dev/name', operator: 'In', values: ['my-func'] },
        ],
      },
    });
    expect(screen.getByTestId('dep-count')).toHaveTextContent('1');
    expect(screen.getByTestId('deployment')).toHaveTextContent('my-func-00001-deployment');
  });

  it('returns empty arrays when not loaded', () => {
    mockUseK8sWatchResource
      .mockReturnValueOnce([[], false, null])
      .mockReturnValueOnce([[], false, null]);

    render(<TestConsumer functionNames={['my-func']} />);

    expect(screen.getByTestId('loaded')).toHaveTextContent('false');
    expect(screen.getByTestId('ksvc-count')).toHaveTextContent('0');
    expect(screen.getByTestId('dep-count')).toHaveTextContent('0');
  });
});
