import { DocumentTitle, ListPageHeader } from '@openshift-console/dynamic-plugin-sdk';
import { PageSection } from '@patternfly/react-core';
import { useTranslation } from 'react-i18next';
import { useParams } from 'react-router-dom-v5-compat';

export default function FunctionEditPage() {
  const { t } = useTranslation('plugin__console-functions-plugin');
  const { name } = useParams<{ name: string }>();

  return (
    <>
      <DocumentTitle>{t('Edit function')}</DocumentTitle>
      <ListPageHeader title={`${t('Edit function')}: ${name}`} />
      <PageSection>{t('Coming soon.')}</PageSection>
    </>
  );
}
