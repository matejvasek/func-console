import {
  DocumentTitle,
  ListPageHeader,
  K8sResourceKind,
} from '@openshift-console/dynamic-plugin-sdk';
import { Button, Content, ContentVariants, PageSection, Spinner } from '@patternfly/react-core';
import { Link, useNavigate } from 'react-router-dom-v5-compat';
import { useTranslation } from 'react-i18next';
import { useEffect, useState, useMemo } from 'react';
import { FunctionsEmptyState } from '../components/EmptyState';
import { FunctionStatus, FunctionTable, FunctionTableItem } from '../components/FunctionTable';
import { useSourceControl } from '../services/source-control/useSourceControl';
import { useClusterService } from '../services/cluster/useClusterService';

export default function FunctionsListPage() {
  const { t } = useTranslation('plugin__console-functions-plugin');
  const { functions, loaded, onEdit } = useFunctionListPage();

  return (
    <>
      <DocumentTitle>{t('Functions')}</DocumentTitle>
      <ListPageHeader title={t('Functions')} />
      <PageSection>
        {!loaded && (
          <Spinner aria-label={t('Loading')} style={{ display: 'block', margin: '4rem auto' }} />
        )}
        {loaded && functions.length === 0 && <FunctionsEmptyState />}
        {loaded && functions.length > 0 && (
          <>
            <Content component={ContentVariants.p}>
              {t(
                'Serverless functions in your repository and deployed to your cluster. Manage lifecycle, monitor status, and scale on demand.',
              )}
            </Content>
            <Content component={ContentVariants.p}>
              <Button
                variant="primary"
                component={(props) => <Link {...props} to="/functions/create" />}
              >
                {t('Create new function')}
              </Button>
            </Content>
            <FunctionTable functions={functions} onEdit={onEdit} />
          </>
        )}
      </PageSection>
    </>
  );
}

function useFunctionListPage(): {
  functions: FunctionTableItem[];
  loaded: boolean;
  onEdit: (name: string) => void;
} {
  const sourceControl = useSourceControl();
  const { deployments, loaded: clusterLoaded } = useClusterService();
  const navigate = useNavigate();

  const [functionItems, setFunctionItems] = useState<FunctionTableItem[]>([]);
  const [reposLoaded, setReposLoaded] = useState(false);

  useEffect(() => {
    let ignore = false;

    async function loadFunctionTableItems() {
      const repos = await sourceControl.listFunctionRepos();
      const items = await Promise.all(
        repos.map(async (repo) => {
          const funcYaml = await sourceControl.fetchFileContent(repo, 'func.yaml');
          const { namespace, runtime } = parseNamespaceAndRuntime(funcYaml, repo.name);
          return newItem(repo.name, namespace, runtime);
        }),
      );
      if (!ignore) {
        setFunctionItems(items);
        setReposLoaded(true);
      }
    }

    loadFunctionTableItems().catch(() => {
      if (!ignore) {
        setReposLoaded(true);
      }
    });
    return () => {
      ignore = true;
    };
  }, [sourceControl]);

  const functions = useMemo(
    () =>
      functionItems.map((item) => {
        const deployment = deployments.find(
          (d) => d.metadata?.labels?.['function.knative.dev/name'] === item.name,
        );
        return deployment ? enrichItem(item, deployment) : item;
      }),
    [functionItems, deployments],
  );

  const loaded = reposLoaded && clusterLoaded;

  const onEdit = (name: string) => navigate(`/functions/edit/${name}`);
  return { functions, loaded, onEdit };
}

function parseNamespaceAndRuntime(
  funcYaml: string,
  repoName: string,
): {
  namespace: string;
  runtime: string;
} {
  const runtimeMatch = funcYaml.match(/^runtime:\s*(.+)$/m);
  const namespaceMatch = funcYaml.match(/^namespace:\s*(.+)$/m);
  if (!runtimeMatch) throw new Error(`func.yaml in ${repoName} missing runtime field`);
  return { namespace: namespaceMatch?.[1]?.trim() ?? '', runtime: runtimeMatch[1].trim() };
}

function newItem(repoName: string, namespace: string, runtime: string): FunctionTableItem {
  return {
    name: repoName,
    namespace,
    runtime,
    status: 'NotDeployed' as const,
    replicas: 0,
  };
}

function enrichItem(item: FunctionTableItem, deployment: K8sResourceKind): FunctionTableItem {
  return {
    ...item,
    status: deriveStatus(deployment),
    url: `http://${item.name}.${deployment.metadata?.namespace}.svc`,
    replicas: deployment.status?.readyReplicas ?? 0,
    deployment,
  };
}

function deriveStatus(deployment: K8sResourceKind): FunctionStatus {
  const desired = deployment.spec?.replicas ?? 0;
  const ready = deployment.status?.readyReplicas ?? 0;
  const conditions = deployment.status?.conditions ?? [];

  const hasFailed = conditions.some(
    (c: { type: string; status: string }) => c.type === 'Available' && c.status === 'False',
  );
  if (hasFailed) return 'Error';

  if (ready === desired && desired > 0) return 'Running';
  if (ready === 0 && desired === 0) return 'ScaledToZero';
  if (ready < desired) return 'Deploying';

  return 'Unknown';
}
