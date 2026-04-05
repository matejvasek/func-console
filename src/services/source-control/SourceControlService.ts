import { RepoInfo } from '../types';

export interface SourceControlService {
  listFunctionRepos(): Promise<RepoInfo[]>;
  fetchFileContent(repo: RepoInfo, path: string): Promise<string>;
}
