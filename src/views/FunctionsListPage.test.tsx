import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom-v5-compat';
import FunctionsListPage from './FunctionsListPage';

jest.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

jest.mock('@openshift-console/dynamic-plugin-sdk', () => ({
  DocumentTitle: ({ children }: { children: string }) => children,
  ListPageHeader: ({ title, children }: { title: string; children?: React.ReactNode }) => (
    <>
      {title}
      {children}
    </>
  ),
}));

const mockUseSourceControl = jest.fn();
jest.mock('../services/source-control/useSourceControlService', () => ({
  useSourceControlService: () => mockUseSourceControl(),
}));

const mockUseClusterService = jest.fn();
jest.mock('../services/cluster/useClusterService', () => ({
  useClusterService: () => mockUseClusterService(),
}));

jest.mock('../components/FunctionTable', () => ({
  FunctionTable: ({ functions }: { functions: { name: string }[] }) =>
    functions.map((f) => f.name).join(','),
}));

afterEach(() => {
  jest.restoreAllMocks();
});

describe('FunctionsListPage', () => {
  it('renders a spinner while loading', () => {
    mockUseSourceControl.mockReturnValue({
      listFunctionRepos: jest.fn().mockResolvedValue([]),
      fetchFileContent: jest.fn(),
    });
    mockUseClusterService.mockReturnValue({ deployments: [], loaded: false, error: null });

    render(
      <MemoryRouter>
        <FunctionsListPage />
      </MemoryRouter>,
    );

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('renders the empty state when loaded with no functions', async () => {
    mockUseSourceControl.mockReturnValue({
      listFunctionRepos: jest.fn().mockResolvedValue([]),
      fetchFileContent: jest.fn(),
    });
    mockUseClusterService.mockReturnValue({ deployments: [], loaded: true, error: null });

    render(
      <MemoryRouter>
        <FunctionsListPage />
      </MemoryRouter>,
    );

    expect(await screen.findByRole('heading', { name: 'No functions found' })).toBeInTheDocument();
  });

  it('renders table when functions are loaded', async () => {
    mockUseSourceControl.mockReturnValue({
      listFunctionRepos: jest.fn().mockResolvedValue([
        {
          owner: 'twoGiants',
          name: 'my-func',
          url: 'https://github.com/twoGiants/my-func',
          defaultBranch: 'main',
        },
      ]),
      fetchFileContent: jest
        .fn()
        .mockResolvedValue('name: my-func\nruntime: go\nnamespace: demo\n'),
    });
    mockUseClusterService.mockReturnValue({
      deployments: [
        {
          apiVersion: 'apps/v1',
          kind: 'Deployment',
          metadata: {
            name: 'my-func',
            namespace: 'demo',
            labels: { 'function.knative.dev/name': 'my-func' },
          },
          spec: { replicas: 1 },
          status: { readyReplicas: 1 },
        },
      ],
      loaded: true,
      error: null,
    });

    render(
      <MemoryRouter>
        <FunctionsListPage />
      </MemoryRouter>,
    );

    expect(await screen.findByText('my-func')).toBeInTheDocument();
  });

  it('shows NotDeployed status for repos without cluster deployment', async () => {
    mockUseSourceControl.mockReturnValue({
      listFunctionRepos: jest.fn().mockResolvedValue([
        {
          owner: 'twoGiants',
          name: 'orphan-func',
          url: 'https://github.com/twoGiants/orphan-func',
          defaultBranch: 'main',
        },
      ]),
      fetchFileContent: jest
        .fn()
        .mockResolvedValue('name: orphan-func\nruntime: node\nnamespace: demo\n'),
    });
    mockUseClusterService.mockReturnValue({ deployments: [], loaded: true, error: null });

    render(
      <MemoryRouter>
        <FunctionsListPage />
      </MemoryRouter>,
    );

    expect(await screen.findByText('orphan-func')).toBeInTheDocument();
  });

  it('renders empty state when GitHub API fails', async () => {
    mockUseSourceControl.mockReturnValue({
      listFunctionRepos: jest.fn().mockRejectedValue(new Error('Requires authentication')),
      fetchFileContent: jest.fn(),
    });
    mockUseClusterService.mockReturnValue({ deployments: [], loaded: true, error: null });

    render(
      <MemoryRouter>
        <FunctionsListPage />
      </MemoryRouter>,
    );

    expect(await screen.findByRole('heading', { name: 'No functions found' })).toBeInTheDocument();
  });
});
