import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import FunctionCreatePage from './FunctionCreatePage';

const mockGenerateFunction = jest.fn();
const mockPushFiles = jest.fn();
const mockNavigate = jest.fn();

jest.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

jest.mock('@openshift-console/dynamic-plugin-sdk', () => ({
  DocumentTitle: ({ children }: { children: string }) => children,
  ListPageHeader: ({ title }: { title: string }) => title,
}));

jest.mock('../services/function/useFunctionService', () => ({
  useFunctionService: () => ({ generateFunction: mockGenerateFunction }),
}));

jest.mock('../services/github/useGitHubService', () => ({
  useGitHubService: () => ({ pushFiles: mockPushFiles }),
}));

jest.mock('react-router-dom-v5-compat', () => ({
  useNavigate: () => mockNavigate,
}));

afterEach(() => {
  jest.clearAllMocks();
});

const fillForm = async (user: ReturnType<typeof userEvent.setup>) => {
  await user.type(screen.getByRole('textbox', { name: /Owner/ }), 'testuser');
  await user.type(screen.getByRole('textbox', { name: /Repository/ }), 'my-repo');
  await user.type(screen.getByRole('textbox', { name: /Branch/ }), 'main');
  await user.type(screen.getByLabelText(/Personal Access Token/), 'ghp_token');
  await user.type(screen.getByRole('textbox', { name: /^Name$/ }), 'my-func');
  await user.type(screen.getByRole('textbox', { name: /Registry/ }), 'quay.io/test');
  await user.type(screen.getByRole('textbox', { name: /Namespace/ }), 'default');
};

describe('FunctionCreatePage', () => {
  it('renders CreateFunctionForm', () => {
    render(
      <MemoryRouter>
        <FunctionCreatePage />
      </MemoryRouter>,
    );

    expect(screen.getByRole('textbox', { name: /Owner/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Create/ })).toBeInTheDocument();
  });

  it('calls generateFunction then pushFiles on submit, and navigates on success', async () => {
    const user = userEvent.setup();
    const files = [{ path: 'func.yaml', mode: '100644', content: 'name: f', type: 'blob' }];
    mockGenerateFunction.mockResolvedValue(files);
    mockPushFiles.mockResolvedValue(undefined);

    render(
      <MemoryRouter>
        <FunctionCreatePage />
      </MemoryRouter>,
    );

    await fillForm(user);
    await user.click(screen.getByRole('button', { name: /Create/ }));

    await waitFor(() => {
      expect(mockGenerateFunction).toHaveBeenCalledWith({
        name: 'my-func',
        runtime: 'node',
        registry: 'quay.io/test',
        namespace: 'default',
        branch: 'main',
      });
    });

    await waitFor(() => {
      expect(mockPushFiles).toHaveBeenCalledWith(
        { owner: 'testuser', repo: 'my-repo', branch: 'main' },
        'ghp_token',
        files,
        'Initialize Knative function project',
      );
    });

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/functions');
    });
  });

  it('shows an alert on error', async () => {
    const user = userEvent.setup();
    mockGenerateFunction.mockRejectedValue(new Error('Backend error'));

    render(
      <MemoryRouter>
        <FunctionCreatePage />
      </MemoryRouter>,
    );

    await fillForm(user);
    await user.click(screen.getByRole('button', { name: /Create/ }));

    await waitFor(() => {
      expect(screen.getByText('Backend error')).toBeInTheDocument();
    });
  });
});
