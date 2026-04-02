import { useState } from 'react';
import {
  ActionGroup,
  Button,
  Form,
  FormGroup,
  FormSection,
  FormSelect,
  FormSelectOption,
  TextInput,
} from '@patternfly/react-core';
import { useTranslation } from 'react-i18next';
import { FunctionRuntime } from '../services/types';

export interface CreateFunctionFormData {
  owner: string;
  repo: string;
  branch: string;
  pat: string;
  name: string;
  runtime: FunctionRuntime;
  registry: string;
  namespace: string;
}

interface CreateFunctionFormProps {
  onSubmit: (data: CreateFunctionFormData) => void;
  onCancel: () => void;
  isSubmitting: boolean;
}

const runtimeOptions = [
  { value: 'node', label: 'Node.js' },
  { value: 'python', label: 'Python' },
  { value: 'go', label: 'Go' },
  { value: 'quarkus', label: 'Quarkus' },
];

export function CreateFunctionForm({ onSubmit, onCancel, isSubmitting }: CreateFunctionFormProps) {
  const { t } = useTranslation('plugin__console-functions-plugin');

  const [owner, setOwner] = useState('');
  const [repo, setRepo] = useState('');
  const [branch, setBranch] = useState('');
  const [pat, setPat] = useState('');
  const [name, setName] = useState('');
  const [runtime, setRuntime] = useState<FunctionRuntime>('node');
  const [registry, setRegistry] = useState('');
  const [namespace, setNamespace] = useState('');

  const isValid = owner && repo && branch && pat && name && registry && namespace;

  const handleSubmit = () => {
    onSubmit({ owner, repo, branch, pat, name, runtime, registry, namespace });
  };

  return (
    <Form onSubmit={(e) => { e.preventDefault(); handleSubmit(); }}>
      <FormSection title={t('GitHub Settings')}>
        <FormGroup label={t('Owner')} isRequired fieldId="owner">
          <TextInput
            id="owner"
            isRequired
            value={owner}
            onChange={(_e, val) => setOwner(val)}
          />
        </FormGroup>
        <FormGroup label={t('Repository')} isRequired fieldId="repo">
          <TextInput
            id="repo"
            isRequired
            value={repo}
            onChange={(_e, val) => setRepo(val)}
          />
        </FormGroup>
        <FormGroup label={t('Branch')} isRequired fieldId="branch">
          <TextInput
            id="branch"
            isRequired
            value={branch}
            onChange={(_e, val) => setBranch(val)}
          />
        </FormGroup>
        <FormGroup label={t('Personal Access Token')} isRequired fieldId="pat">
          <TextInput
            id="pat"
            type="password"
            isRequired
            value={pat}
            onChange={(_e, val) => setPat(val)}
          />
        </FormGroup>
      </FormSection>
      <FormSection title={t('Function Settings')}>
        <FormGroup label={t('Name')} isRequired fieldId="name">
          <TextInput
            id="name"
            isRequired
            value={name}
            onChange={(_e, val) => setName(val)}
          />
        </FormGroup>
        <FormGroup label={t('Language')} isRequired fieldId="runtime">
          <FormSelect
            id="runtime"
            value={runtime}
            onChange={(_e, val) => setRuntime(val as FunctionRuntime)}
            aria-label={t('Language')}
          >
            {runtimeOptions.map(({ value, label }) => (
              <FormSelectOption key={value} value={value} label={label} />
            ))}
          </FormSelect>
        </FormGroup>
        <FormGroup label={t('Registry')} isRequired fieldId="registry">
          <TextInput
            id="registry"
            isRequired
            value={registry}
            onChange={(_e, val) => setRegistry(val)}
          />
        </FormGroup>
        <FormGroup label={t('Namespace')} isRequired fieldId="namespace">
          <TextInput
            id="namespace"
            isRequired
            value={namespace}
            onChange={(_e, val) => setNamespace(val)}
          />
        </FormGroup>
      </FormSection>
      <ActionGroup>
        <Button
          type="submit"
          variant="primary"
          isDisabled={!isValid || isSubmitting}
          isLoading={isSubmitting}
        >
          {t('Create')}
        </Button>
        <Button variant="link" onClick={onCancel}>
          {t('Cancel')}
        </Button>
      </ActionGroup>
    </Form>
  );
}
