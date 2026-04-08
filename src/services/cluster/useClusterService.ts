import {
  K8sResourceKind,
  useK8sWatchResource,
  useActiveNamespace,
} from '@openshift-console/dynamic-plugin-sdk';

export interface ClusterService {
  deployments: K8sResourceKind[];
  loaded: boolean;
  error: unknown;
}

const ALL_NAMESPACES = '#ALL_NS#';

export function useClusterService(): ClusterService {
  const [activeNamespace] = useActiveNamespace();

  const namespace = activeNamespace === ALL_NAMESPACES ? undefined : activeNamespace;

  const [data, loaded, error] = useK8sWatchResource<K8sResourceKind[]>({
    groupVersionKind: { group: 'apps', version: 'v1', kind: 'Deployment' },
    ...(namespace && { namespace }),
    isList: true,
    selector: {
      matchExpressions: [{ key: 'function.knative.dev/name', operator: 'Exists' }],
    },
  });

  const deployments = loaded ? (data ?? []) : [];

  return { deployments, loaded, error };
}
