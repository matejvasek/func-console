import { K8sResourceCommon } from '@openshift-console/dynamic-plugin-sdk';
import { ClusterError } from '../../common/errors';
import { FunctionSource, WorkflowRun } from '../../common/types';

export interface Function {
  readonly owner: string;
  readonly repoName: string;
  readonly source: FunctionSource;
  readonly status: FunctionStatus;
  readonly routeURL: string;
  readonly name?: string;
  readonly namespace?: string;
  readonly runtime?: string;
  readonly replicas?: number;
  readonly mainResource?: K8sResourceCommon;
  readonly error?: string;
}

export interface FunctionStatus {
  cluster: ClusterStatus;
  workflow: WorkflowRun;
}

export interface ClusterStatus {
  status: 'NotDeployed' | 'Deploying' | 'Running' | 'ScaledToZero' | 'Undeploying' | 'Error';
  error?: ClusterError;
}

/** Returns a unique key for a function. Healthy entries are keyed by
 * namespace/name, broken entries (with error) by owner/repoName. The ~
 * prefix on broken keys prevents collisions when a K8s namespace happens
 * to match a GitHub owner name (e.g. both produce "alice/my-func"). */
export function functionKey(
  fn: Pick<Function, 'error' | 'owner' | 'repoName' | 'namespace' | 'name'>,
): string {
  return fn.error ? `~${fn.owner}/${fn.repoName}` : `${fn.namespace}/${fn.name}`;
}
