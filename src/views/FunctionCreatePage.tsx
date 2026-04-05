import { DocumentTitle, ListPageHeader } from '@openshift-console/dynamic-plugin-sdk';
import { PageSection } from '@patternfly/react-core';
import { useTranslation } from 'react-i18next';

export default function FunctionCreatePage() {
  const { t } = useTranslation('plugin__console-functions-plugin');

  return (
    <>
      <DocumentTitle>{t('Create function')}</DocumentTitle>
      <ListPageHeader title={t('Create function')} />
      <PageSection>{t('Coming soon.')}</PageSection>
    </>
  );
}
