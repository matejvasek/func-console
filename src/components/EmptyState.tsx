import {
  Button,
  EmptyState,
  EmptyStateActions,
  EmptyStateBody,
  EmptyStateFooter,
} from '@patternfly/react-core';
import { CubesIcon } from '@patternfly/react-icons';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom-v5-compat';

export function FunctionsEmptyState() {
  const { t } = useTranslation('plugin__console-functions-plugin');

  return (
    <EmptyState headingLevel="h2" icon={CubesIcon} titleText={t('No functions found')}>
      <EmptyStateBody>{t('Create a serverless function to get started.')}</EmptyStateBody>
      <EmptyStateFooter>
        <EmptyStateActions>
          <Button
            variant="primary"
            component={(props) => <Link {...props} to="/faas/create" />}
          >
            {t('Create function')}
          </Button>
        </EmptyStateActions>
      </EmptyStateFooter>
    </EmptyState>
  );
}
