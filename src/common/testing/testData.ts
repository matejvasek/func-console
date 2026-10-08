import { FunctionListItem } from '../types';

export function repoListItem(
  repoName: string,
  name?: string,
  namespace = 'demo',
  runtime = 'go',
): FunctionListItem {
  return {
    owner: 'twoGiants',
    repoName,
    repoURL: `https://github.com/twoGiants/${repoName}`,
    defaultBranch: 'main',
    name: name ?? repoName,
    namespace,
    runtime,
    source: 'repo',
  };
}
