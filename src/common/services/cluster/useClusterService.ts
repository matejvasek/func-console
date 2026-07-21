import { K8sResourceKind, useK8sWatchResource } from '@openshift-console/dynamic-plugin-sdk';
import { useMemo } from 'react';
import { ClusterFunction, FunctionStatus } from '../types';
import { OcpClusterService } from './OcpClusterService';

const instance = new OcpClusterService();

const FUNCTION_NAME_LABEL = 'function.knative.dev/name';
const REVISION_LABEL = 'serving.knative.dev/revision';
const DEPLOYER_ANNOTATION = 'function.knative.dev/deployer';
const KEDA_DEPLOYER = 'keda';

function isCrdMissingError(error: unknown): boolean {
  if (!error) return false;
  const err = error as { name?: string; code?: number };
  return err.name === 'NoModelError' || err.code === 404;
}

interface ClusterService {
  functions: ReadonlyMap<string, ClusterFunction>;
  loaded: boolean;
  error: unknown;
  generateKubeconfig: (namespace: string) => Promise<string>;
}

interface ClusterFunctionResult {
  functions: ReadonlyMap<string, ClusterFunction>;
  loaded: boolean;
  error: unknown;
}

export function useClusterService(functionNames: string[] = []): ClusterService {
  const kn = useKnativeClusterFunctions(functionNames);
  const keda = useKedaClusterFunctions(functionNames);

  const functions = useMemo(
    () => new Map([...kn.functions, ...keda.functions]),
    [kn.functions, keda.functions],
  );

  return {
    functions,
    loaded: kn.loaded && keda.loaded,
    error: kn.error || keda.error,
    generateKubeconfig: instance.generateKubeconfig.bind(instance),
  };
}

function useLabelSelector(functionNames: string[]) {
  return useMemo(
    () => ({
      matchExpressions: [{ key: FUNCTION_NAME_LABEL, operator: 'In', values: functionNames }],
    }),
    [functionNames],
  );
}

function useKnativeClusterFunctions(functionNames: string[]): ClusterFunctionResult {
  const selector = useLabelSelector(functionNames);

  const knSvcConfig = useMemo(
    () =>
      functionNames.length > 0
        ? {
            groupVersionKind: { group: 'serving.knative.dev', version: 'v1', kind: 'Service' },
            isList: true,
            selector,
          }
        : null,
    [functionNames, selector],
  );

  const depConfig = useMemo(
    () =>
      functionNames.length > 0
        ? {
            groupVersionKind: { group: 'apps', version: 'v1', kind: 'Deployment' },
            isList: true,
            selector,
          }
        : null,
    [functionNames, selector],
  );

  const [knSvcs, knLoaded, knError] = useK8sWatchResource<K8sResourceKind[]>(knSvcConfig);
  const [deps, depLoaded, depError] = useK8sWatchResource<K8sResourceKind[]>(depConfig);

  const functions = useMemo(() => {
    const safeKnSvcs = knLoaded ? (knSvcs ?? []) : [];
    const safeDeps = depLoaded ? (deps ?? []) : [];
    return listKnativeClusterFunctions(safeKnSvcs, safeDeps);
  }, [knSvcs, knLoaded, deps, depLoaded]);

  return {
    functions,
    loaded: knLoaded && depLoaded,
    error: (isCrdMissingError(knError) ? null : knError) || depError,
  };
}

function useKedaClusterFunctions(functionNames: string[]): ClusterFunctionResult {
  const selector = useLabelSelector(functionNames);

  const svcConfig = useMemo(
    () =>
      functionNames.length > 0
        ? {
            groupVersionKind: { group: '', version: 'v1', kind: 'Service' },
            isList: true,
            selector,
          }
        : null,
    [functionNames, selector],
  );

  const hsoConfig = useMemo(
    () =>
      functionNames.length > 0
        ? {
            groupVersionKind: {
              group: 'http.keda.sh',
              version: 'v1alpha1',
              kind: 'HTTPScaledObject',
            },
            isList: true,
            selector,
          }
        : null,
    [functionNames, selector],
  );

  const depConfig = useMemo(
    () =>
      functionNames.length > 0
        ? {
            groupVersionKind: { group: 'apps', version: 'v1', kind: 'Deployment' },
            isList: true,
            selector,
          }
        : null,
    [functionNames, selector],
  );

  const [svcs, svcLoaded, svcError] = useK8sWatchResource<K8sResourceKind[]>(svcConfig);
  const [hsos, , hsoError] = useK8sWatchResource<K8sResourceKind[]>(hsoConfig);
  const [deps, depLoaded, depError] = useK8sWatchResource<K8sResourceKind[]>(depConfig);

  const functions = useMemo(() => {
    const safeSvcs = svcLoaded ? (svcs ?? []) : [];
    const safeHsos = hsos ?? [];
    const safeDeps = depLoaded ? (deps ?? []) : [];
    return listKedaClusterFunctions(safeSvcs, safeHsos, safeDeps);
  }, [svcs, svcLoaded, hsos, deps, depLoaded]);

  return {
    functions,
    loaded: svcLoaded && depLoaded,
    error: svcError || (isCrdMissingError(hsoError) ? null : hsoError) || depError,
  };
}

function listKnativeClusterFunctions(
  knSvcs: K8sResourceKind[],
  deployments: K8sResourceKind[],
): ReadonlyMap<string, ClusterFunction> {
  const entries = knSvcs.map((ksvc): [string, ClusterFunction] => {
    const name = ksvc.metadata?.labels?.[FUNCTION_NAME_LABEL] ?? ksvc.metadata?.name ?? '';
    const latestRevision = ksvc.status?.latestReadyRevisionName;

    const deployment = latestRevision
      ? deployments.find((d) => d.metadata?.labels?.[REVISION_LABEL] === latestRevision)
      : deployments.find((d) => d.metadata?.labels?.[FUNCTION_NAME_LABEL] === name);

    return [
      name,
      {
        name,
        status: deriveKnativeStatus(ksvc, deployment),
        url: ksvc.status?.url,
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

function listKedaClusterFunctions(
  services: K8sResourceKind[],
  httpScaledObjects: K8sResourceKind[],
  deployments: K8sResourceKind[],
): ReadonlyMap<string, ClusterFunction> {
  const entries = services
    .filter((svc) => svc.metadata?.annotations?.[DEPLOYER_ANNOTATION] === KEDA_DEPLOYER)
    .map((svc): [string, ClusterFunction] => {
      const name = svc.metadata?.labels?.[FUNCTION_NAME_LABEL] ?? svc.metadata?.name ?? '';
      const hso = httpScaledObjects.find((h) => h.metadata?.name === name);
      const deployment = deployments.find(
        (d) => d.metadata?.labels?.[FUNCTION_NAME_LABEL] === name,
      );
      const host = (hso?.spec as { hosts?: string[] })?.hosts?.[0];
      return [
        name,
        {
          name,
          status: deriveKedaStatus(hso, deployment),
          url: host ? `http://${host}:8080` : undefined,
          replicas: deployment?.status?.readyReplicas ?? 0,
          mainResource: svc,
        },
      ];
    });

  return new Map(entries);
}

function deriveKedaStatus(
  hso: K8sResourceKind | undefined,
  deployment: K8sResourceKind | undefined,
): FunctionStatus {
  if (!hso) return 'Deploying';

  const conditions = hso.status?.conditions ?? [];
  const ready = conditions.find((c: { type: string }) => c.type === 'Ready');
  if (!ready) return 'Deploying';

  if (ready.status === 'True') {
    const readyReplicas = deployment?.status?.readyReplicas ?? 0;
    if (readyReplicas === 0) return 'ScaledToZero';
    return 'Running';
  }

  if (ready.status === 'False') return 'Error';

  return 'Deploying';
}
