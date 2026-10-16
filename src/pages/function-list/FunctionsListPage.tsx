import {
  DocumentTitle,
  isAllNamespacesKey,
  ListPageHeader,
  NamespaceBar,
  useActiveNamespace,
} from '@openshift-console/dynamic-plugin-sdk';
import {
  Alert,
  Button,
  Content,
  ContentVariants,
  PageSection,
  Spinner,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
} from '@patternfly/react-core';
import { SyncAltIcon } from '@patternfly/react-icons';
import { useContext, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useNavigate } from 'react-router';
import { UserAvatar } from '../../common/components/UserAvatar';
import { AuthContext, AuthProvider } from '../../common/context/AuthProvider';
import { useFunctions } from '../../common/hooks/useFunctions';
import { Function } from '../../common/types';
import { FunctionsEmptyState } from './components/EmptyState';
import { FunctionTable } from './components/FunctionTable';
import { SetupGuide } from './components/SetupGuide';

export default function FunctionsListPage() {
  return (
    <AuthProvider>
      <FunctionsListPageContent />
    </AuthProvider>
  );
}

function FunctionsListPageContent() {
  const { t } = useTranslation('plugin__console-functions-plugin');
  const {
    functions,
    loaded,
    refreshing,
    onEdit,
    onRefresh,
    isAuthenticated,
    errors,
    showNamespace,
  } = useFunctionListPage();

  return (
    <>
      <DocumentTitle>{t('Functions')}</DocumentTitle>
      <NamespaceBar />
      <ListPageHeader title={t('Functions')}>
        <UserAvatar enableReconnect />
      </ListPageHeader>
      <PageSection>
        {errors.length > 0 &&
          errors.map((err, i) => (
            <Alert key={i} variant="danger" title={t('Error')} isInline>
              {err}
            </Alert>
          ))}
        {!loaded && !refreshing && (
          <Spinner aria-label={t('Loading')} style={{ display: 'block', margin: '4rem auto' }} />
        )}
        {loaded && functions.length === 0 && (
          <FunctionsEmptyState isCreateDisabled={!isAuthenticated} />
        )}
        {(loaded || refreshing) && functions.length > 0 && (
          <>
            <Content component={ContentVariants.p}>
              {t(
                'Serverless functions in your repository and deployed to your cluster. Manage lifecycle, monitor status, and scale on demand.',
              )}{' '}
              <SetupGuide />
            </Content>
            <Toolbar>
              <ToolbarContent>
                <ToolbarItem>
                  {!isAuthenticated ? (
                    <Button variant="primary" isDisabled>
                      {t('Create new function')}
                    </Button>
                  ) : (
                    <Button
                      variant="primary"
                      component={(props) => <Link {...props} to="/faas/create" />}
                    >
                      {t('Create new function')}
                    </Button>
                  )}
                </ToolbarItem>
                <ToolbarItem variant="separator" />
                <ToolbarItem>
                  <Button
                    variant="plain"
                    aria-label={t('Refresh')}
                    onClick={onRefresh}
                    isLoading={refreshing}
                    spinnerAriaLabel={t('Refreshing')}
                    isDisabled={refreshing}
                    icon={<SyncAltIcon />}
                  />
                </ToolbarItem>
              </ToolbarContent>
            </Toolbar>
            <FunctionTable functions={functions} onEdit={onEdit} showNamespace={showNamespace} />
          </>
        )}
      </PageSection>
    </>
  );
}

function useFunctionListPage(): {
  functions: Function[];
  loaded: boolean;
  refreshing: boolean;
  onEdit: (name: string) => void;
  onRefresh: () => void;
  isAuthenticated: boolean;
  errors: string[];
  showNamespace: boolean;
} {
  const { isAuthenticated, connectionId } = useContext(AuthContext);
  const navigate = useNavigate();
  const [namespace] = useActiveNamespace();
  const [refreshKey, setRefreshKey] = useState(0);

  const { functions, loaded, refreshing, errors } = useFunctions(
    namespace,
    isAuthenticated ? connectionId : undefined,
    refreshKey,
  );

  function onRefresh() {
    if (!isAuthenticated) return;
    setRefreshKey((k) => k + 1);
  }

  const onEdit = (name: string) => navigate(`/faas/edit/${name}`);

  return {
    functions,
    loaded,
    refreshing,
    onEdit,
    onRefresh,
    isAuthenticated,
    errors: errors ?? [],
    showNamespace: isAllNamespacesKey(namespace),
  };
}
