import { render, screen } from '@testing-library/react';
import { useClusterService } from './useClusterService';

const mockUseK8sWatchResource = jest.fn();
const mockUseActiveNamespace = jest.fn();

jest.mock('@openshift-console/dynamic-plugin-sdk', () => ({
  useK8sWatchResource: (...args: unknown[]) => mockUseK8sWatchResource(...args),
  useActiveNamespace: () => mockUseActiveNamespace(),
}));

afterEach(() => {
  jest.restoreAllMocks();
});

const mockDeployment = {
  apiVersion: 'apps/v1',
  kind: 'Deployment',
  metadata: {
    name: 'func-demo-26',
    namespace: 'demo',
    labels: {
      'function.knative.dev/name': 'func-demo-26',
      'function.knative.dev/runtime': 'go',
    },
  },
  spec: { replicas: 1 },
  status: { readyReplicas: 1 },
};

function TestConsumer() {
  const { deployments, loaded, error } = useClusterService();
  return (
    <>
      <span data-testid="loaded">{String(loaded)}</span>
      <span data-testid="error">{String(error)}</span>
      <span data-testid="count">{deployments.length}</span>
      {deployments.map((d) => (
        <span key={d.metadata?.name} data-testid="deployment">
          {d.metadata?.name}
        </span>
      ))}
    </>
  );
}

describe('useClusterService', () => {
  it('returns raw deployments when loaded', () => {
    mockUseActiveNamespace.mockReturnValue(['demo']);
    mockUseK8sWatchResource.mockReturnValue([[mockDeployment], true, null]);

    render(<TestConsumer />);

    expect(screen.getByTestId('loaded')).toHaveTextContent('true');
    expect(screen.getByTestId('error')).toHaveTextContent('null');
    expect(screen.getByTestId('count')).toHaveTextContent('1');
    expect(screen.getByTestId('deployment')).toHaveTextContent('func-demo-26');
  });

  it('returns empty deployments when not loaded', () => {
    mockUseActiveNamespace.mockReturnValue(['demo']);
    mockUseK8sWatchResource.mockReturnValue([[], false, null]);

    render(<TestConsumer />);

    expect(screen.getByTestId('loaded')).toHaveTextContent('false');
    expect(screen.getByTestId('count')).toHaveTextContent('0');
  });

  it('omits namespace when all namespaces are active', () => {
    mockUseActiveNamespace.mockReturnValue(['#ALL_NS#']);
    mockUseK8sWatchResource.mockReturnValue([[], true, null]);

    render(<TestConsumer />);

    expect(mockUseK8sWatchResource).toHaveBeenCalledWith(
      expect.not.objectContaining({ namespace: '#ALL_NS#' }),
    );
  });

  it('passes namespace when a specific namespace is active', () => {
    mockUseActiveNamespace.mockReturnValue(['demo']);
    mockUseK8sWatchResource.mockReturnValue([[], true, null]);

    render(<TestConsumer />);

    expect(mockUseK8sWatchResource).toHaveBeenCalledWith(
      expect.objectContaining({ namespace: 'demo' }),
    );
  });
});
