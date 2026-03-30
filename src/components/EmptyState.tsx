import {
  EmptyState,
  EmptyStateActions,
  EmptyStateBody,
  EmptyStateFooter,
} from '@patternfly/react-core';
import { CubesIcon } from '@patternfly/react-icons';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';

export function FunctionsEmptyState() {
  const { t } = useTranslation('plugin__console-functions-plugin');

  return (
    <EmptyState headingLevel="h2" icon={CubesIcon} titleText={t('No functions found')}>
      <EmptyStateBody>
        {t('Create a serverless function to get started.')}
      </EmptyStateBody>
      <EmptyStateFooter>
        <EmptyStateActions>
          <Link to="/functions/create">{t('Create function')}</Link>
        </EmptyStateActions>
      </EmptyStateFooter>
    </EmptyState>
  );
}
