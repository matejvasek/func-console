import { SourceRepo } from '../types';

export interface SourceControlService {
  listFunctionRepos(): Promise<SourceRepo[]>;
  fetchFileContent(repo: SourceRepo, path: string): Promise<string>;
}
