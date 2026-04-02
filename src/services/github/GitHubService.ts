import { FileEntry, RepoInfo } from '../types';

export interface GitHubService {
  pushFiles(
    repo: RepoInfo,
    pat: string,
    files: FileEntry[],
    message: string,
  ): Promise<void>;
}
