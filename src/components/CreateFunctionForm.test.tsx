import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { CreateFunctionForm } from './CreateFunctionForm';

jest.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

afterEach(() => {
  jest.restoreAllMocks();
});

describe('CreateFunctionForm', () => {
  const onSubmit = jest.fn();
  const onCancel = jest.fn();

  beforeEach(() => {
    onSubmit.mockClear();
    onCancel.mockClear();
  });

  it('renders all form fields', () => {
    render(
      <CreateFunctionForm onSubmit={onSubmit} onCancel={onCancel} isSubmitting={false} />,
    );

    expect(screen.getByRole('textbox', { name: /Owner/ })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: /Repository/ })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: /Branch/ })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: /^Name$/ })).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: /Language/ })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: /Registry/ })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: /Namespace/ })).toBeInTheDocument();
  });

  it('renders Create and Cancel buttons', () => {
    render(
      <CreateFunctionForm onSubmit={onSubmit} onCancel={onCancel} isSubmitting={false} />,
    );

    expect(screen.getByRole('button', { name: /Create/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Cancel/ })).toBeInTheDocument();
  });

  it('disables Create button when required fields are empty', () => {
    render(
      <CreateFunctionForm onSubmit={onSubmit} onCancel={onCancel} isSubmitting={false} />,
    );

    expect(screen.getByRole('button', { name: /Create/ })).toBeDisabled();
  });

  it('disables Create button when isSubmitting is true', () => {
    render(
      <CreateFunctionForm onSubmit={onSubmit} onCancel={onCancel} isSubmitting={true} />,
    );

    expect(screen.getByRole('button', { name: /Create/ })).toBeDisabled();
  });

  it('calls onCancel when Cancel is clicked', async () => {
    const user = userEvent.setup();

    render(
      <CreateFunctionForm onSubmit={onSubmit} onCancel={onCancel} isSubmitting={false} />,
    );

    await user.click(screen.getByRole('button', { name: /Cancel/ }));
    expect(onCancel).toHaveBeenCalled();
  });

  it('calls onSubmit with form data when form is filled and Create is clicked', async () => {
    const user = userEvent.setup();

    render(
      <CreateFunctionForm onSubmit={onSubmit} onCancel={onCancel} isSubmitting={false} />,
    );

    await user.type(screen.getByRole('textbox', { name: /Owner/ }), 'testuser');
    await user.type(screen.getByRole('textbox', { name: /Repository/ }), 'my-repo');
    await user.type(screen.getByRole('textbox', { name: /Branch/ }), 'main');
    await user.type(screen.getByRole('textbox', { name: /^Name$/ }), 'my-func');
    await user.type(screen.getByRole('textbox', { name: /Registry/ }), 'quay.io/test');
    await user.type(screen.getByRole('textbox', { name: /Namespace/ }), 'default');

    await user.click(screen.getByRole('button', { name: /Create/ }));

    expect(onSubmit).toHaveBeenCalledWith({
      owner: 'testuser',
      repo: 'my-repo',
      branch: 'main',
      name: 'my-func',
      runtime: 'node',
      registry: 'quay.io/test',
      namespace: 'default',
    });
  });
});
