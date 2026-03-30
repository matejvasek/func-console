import { DocumentTitle, ListPageHeader } from '@openshift-console/dynamic-plugin-sdk';
import { PageSection } from '@patternfly/react-core';
import { useTranslation } from 'react-i18next';
import { FunctionsEmptyState } from '../components/EmptyState';

export default function FunctionsListPage() {
  const { t } = useTranslation('plugin__console-functions-plugin');

  return (
    <>
      <DocumentTitle>{t('Functions')}</DocumentTitle>
      <ListPageHeader title={t('Functions')} />
      <PageSection>
        <FunctionsEmptyState />
      </PageSection>
    </>
  );
}
