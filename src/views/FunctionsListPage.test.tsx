import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import FunctionsListPage from './FunctionsListPage';

jest.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

jest.mock('@openshift-console/dynamic-plugin-sdk', () => ({
  DocumentTitle: ({ children }: { children: string }) => children,
  ListPageHeader: ({ title }: { title: string }) => title,
}));

afterEach(() => {
  jest.restoreAllMocks();
});

describe('FunctionsListPage', () => {
  it('renders the empty state when no functions exist', () => {
    render(
      <MemoryRouter>
        <FunctionsListPage />
      </MemoryRouter>,
    );

    expect(
      screen.getByRole('heading', { name: 'No functions found' }),
    ).toBeInTheDocument();
  });

  it('renders a "Create function" link to /functions/create', () => {
    render(
      <MemoryRouter>
        <FunctionsListPage />
      </MemoryRouter>,
    );

    expect(
      screen.getByRole('link', { name: 'Create function' }),
    ).toHaveAttribute('href', '/functions/create');
  });
});
