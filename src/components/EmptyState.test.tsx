import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom-v5-compat';
import { FunctionsEmptyState } from './EmptyState';

jest.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

afterEach(() => {
  jest.restoreAllMocks();
});

describe('FunctionsEmptyState', () => {
  it('renders a heading with "No functions found"', () => {
    render(
      <MemoryRouter>
        <FunctionsEmptyState />
      </MemoryRouter>,
    );

    expect(screen.getByRole('heading', { name: 'No functions found' })).toBeInTheDocument();
  });

  it('renders a "Create function" link pointing to /faas/create', () => {
    render(
      <MemoryRouter>
        <FunctionsEmptyState />
      </MemoryRouter>,
    );

    const link = screen.getByRole('link', { name: 'Create function' });
    expect(link).toHaveAttribute('href', '/faas/create');
  });
});
