import {
  DocumentTitle,
  isAllNamespacesKey,
  K8sResourceKind,
  ListPageHeader,
  NamespaceBar,
  useActiveNamespace,
  useK8sWatchResource,
  WatchK8sResource,
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
import { useContext, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useNavigate } from 'react-router';
import { FunctionsEmptyState } from './components/EmptyState';
import { FunctionTable, FunctionTableItem } from './components/FunctionTable';
import { SetupGuide } from './components/SetupGuide';
import { UserAvatar } from '../../common/components/UserAvatar';
import { AuthContext, AuthProvider } from '../../common/context/AuthProvider';
import {
  ClusterFunction,
  FUNCTION_NAME_LABEL,
  FunctionListItem,
  FunctionStatus,
  REVISION_LABEL,
} from '../../common/types';
import { listFunctions } from '../../common/clients/functionsClient';
import { errorMessage } from '../../common/utils/utils';

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
    error,
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
        {error && (
          <Alert variant="danger" title={t('Error listing functions')} isInline>
            {error}
          </Alert>
        )}
        {!loaded && (
          <Spinner aria-label={t('Loading')} style={{ display: 'block', margin: '4rem auto' }} />
        )}
        {loaded && functions.length === 0 && (
          <FunctionsEmptyState isCreateDisabled={!isAuthenticated} />
        )}
        {loaded && functions.length > 0 && (
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
  functions: FunctionTableItem[];
  loaded: boolean;
  refreshing: boolean;
  onEdit: (name: string) => void;
  onRefresh: () => void;
  isAuthenticated: boolean;
  error: string;
  showNamespace: boolean;
} {
  const { isAuthenticated, connectionId } = useContext(AuthContext);
  const navigate = useNavigate();

  const [namespace] = useActiveNamespace();

  const [functionItems, setFunctionItems] = useState<FunctionTableItem[]>([]);
  const [namespaceLoaded, setNamespaceLoaded] = useState(isAuthenticated ? false : true);
  const [prevNamespace, setPrevNamespace] = useState(namespace);

  const [prevConnectionId, setPrevConnectionId] = useState(connectionId);

  const [error, setError] = useState<string>('');
  const [refreshing, setRefreshing] = useState(false);

  // Reset state when connection changes (initial connect or user switch)
  if (connectionId !== prevConnectionId) {
    setPrevConnectionId(connectionId);
    setFunctionItems([]);
    setError('');
    setNamespaceLoaded(false);
  }

  // Reset state when namespace changes
  if (namespace !== prevNamespace) {
    setPrevNamespace(namespace);
    setFunctionItems([]);
    setError('');
    setNamespaceLoaded(false);
  }

  async function onRefresh() {
    if (!isAuthenticated) return;
    setRefreshing(true);

    try {
      const items = await loadFunctionTableItems(namespace);
      setFunctionItems(items);
      setNamespaceLoaded(true);
      setError('');
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setRefreshing(false);
    }
  }

  useEffect(() => {
    if (!isAuthenticated) return;

    let ignore = false;

    async function doLoad() {
      let items: FunctionTableItem[];

      try {
        items = await loadFunctionTableItems(namespace);
      } catch (err) {
        if (!ignore) {
          setNamespaceLoaded(true);
          setError(errorMessage(err));
        }
        return;
      }
      if (ignore) return;

      setFunctionItems(items);
      setNamespaceLoaded(true);
      setError('');
    }

    doLoad();
    return () => {
      ignore = true;
    };
  }, [isAuthenticated, connectionId, namespace]);

  const functionNames = useMemo(() => functionItems.map((item) => item.name), [functionItems]);

  const [ksvcConfig, depConfig] = useMemo(() => {
    // if namespace === #ALL_NS# then we pass 'undefined' to watcher which equals
    // to 'get resources from all namespaces'
    const ns = isAllNamespacesKey(namespace) ? undefined : namespace;
    return [newKsvcWatchConfig(functionNames, ns), newDeploymentWatchConfig(functionNames, ns)];
  }, [functionNames, namespace]);

  const [knSvcs, knLoaded] = useK8sWatchResource<K8sResourceKind[]>(ksvcConfig);
  const [deps, depLoaded] = useK8sWatchResource<K8sResourceKind[]>(depConfig);

  const clusterFunctions = useMemo(() => {
    const safeKnSvcs = knLoaded ? (knSvcs ?? []) : [];
    const safeDeps = depLoaded ? (deps ?? []) : [];
    return listKnativeClusterFunctions(safeKnSvcs, safeDeps);
  }, [knSvcs, knLoaded, deps, depLoaded]);

  const functions = useMemo(
    () =>
      functionItems.map((item) => {
        // keyed by namespace/name - the same function name can exist in multiple namespaces
        const cf = clusterFunctions.get(`${item.namespace}/${item.name}`);
        return cf ? enrichItem(item, cf) : item;
      }),
    [functionItems, clusterFunctions],
  );

  const reposLoaded = !isAuthenticated || (namespaceLoaded && prevNamespace === namespace);
  const loaded = reposLoaded && knLoaded && depLoaded;

  const onEdit = (name: string) => navigate(`/faas/edit/${name}`);

  return {
    functions,
    loaded,
    refreshing,
    onEdit,
    onRefresh,
    isAuthenticated,
    error,
    showNamespace: isAllNamespacesKey(namespace),
  };
}

function newKsvcWatchConfig(functionNames: string[], namespace?: string): WatchK8sResource | null {
  return functionNames.length > 0
    ? {
        groupVersionKind: { group: 'serving.knative.dev', version: 'v1', kind: 'Service' },
        isList: true,
        namespace,
        selector: {
          matchExpressions: [{ key: FUNCTION_NAME_LABEL, operator: 'In', values: functionNames }],
        },
      }
    : null;
}

function newDeploymentWatchConfig(
  functionNames: string[],
  namespace?: string,
): WatchK8sResource | null {
  return functionNames.length > 0
    ? {
        groupVersionKind: { group: 'apps', version: 'v1', kind: 'Deployment' },
        isList: true,
        namespace,
        selector: {
          matchExpressions: [{ key: FUNCTION_NAME_LABEL, operator: 'In', values: functionNames }],
        },
      }
    : null;
}

function listKnativeClusterFunctions(
  knSvcs: K8sResourceKind[],
  deployments: K8sResourceKind[],
): ReadonlyMap<string, ClusterFunction> {
  const entries = knSvcs.map((ksvc): [string, ClusterFunction] => {
    const name = ksvc.metadata?.labels?.[FUNCTION_NAME_LABEL] ?? ksvc.metadata?.name ?? '';
    const namespace = ksvc.metadata?.namespace ?? '';
    const latestRevision = ksvc.status?.latestReadyRevisionName;

    const nsDeployments = deployments.filter((d) => d.metadata?.namespace === namespace);
    const deployment = latestRevision
      ? nsDeployments.find((d) => d.metadata?.labels?.[REVISION_LABEL] === latestRevision)
      : nsDeployments.find((d) => d.metadata?.labels?.[FUNCTION_NAME_LABEL] === name);

    return [
      `${namespace}/${name}`,
      {
        name,
        namespace,
        status: deriveKnativeStatus(ksvc, deployment),
        url: ksvc.status?.url ?? '',
        replicas: deployment?.status?.readyReplicas ?? 0,
        mainResource: ksvc,
      },
    ];
  });

  return new Map(entries);
}

function deriveKnativeStatus(
  ksvc: K8sResourceKind,
  deployment: K8sResourceKind | undefined,
): FunctionStatus {
  if (!deployment) return 'Deploying';

  const conditions = ksvc.status?.conditions ?? [];
  const ready = conditions.find((c: { type: string }) => c.type === 'Ready');
  if (!ready) return 'Deploying';

  if (ready.status === 'True') {
    const desired = deployment.spec?.replicas ?? 0;
    const readyReplicas = deployment.status?.readyReplicas ?? 0;
    if (desired === 0 && readyReplicas === 0) return 'ScaledToZero';
    return 'Running';
  }

  if (ready.status === 'False') return 'Error';

  return 'Deploying';
}

async function loadFunctionTableItems(namespace: string): Promise<FunctionTableItem[]> {
  const items = await listFunctions(namespace);
  return items.map((item) => newItem(item));
}

function newItem(item: FunctionListItem): FunctionTableItem {
  return {
    name: item.name || item.repoName,
    repoName: item.repoName,
    namespace: item.namespace,
    runtime: item.runtime,
    status: item.err ? 'Error' : 'NotDeployed',
    url: '',
    replicas: 0,
    source: item.source,
    err: item.err,
  };
}

// enrichItem merges cluster state into a repo item. If the item already has an
// error (e.g. namespace check failed), the Error status is preserved so the
// tooltip remains visible, but url/replicas/mainResource are still filled in
// from the cluster.
// Note: a repo item with a func.yaml parse error cannot be enriched
// (empty namespace prevents a cluster match) and will appear as a duplicate
// alongside its deployed counterpart until SRVOCF-1138 is resolved.
function enrichItem(item: FunctionTableItem, cf: ClusterFunction): FunctionTableItem {
  return {
    ...item,
    status: item.err ? item.status : cf.status,
    url: cf.url,
    replicas: cf.replicas,
    mainResource: cf.mainResource,
  };
}
