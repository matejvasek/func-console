import { GitHubService } from './GitHubService';
import { OctokitGitHubService } from './OctokitGitHubService';

const instance = new OctokitGitHubService();

export function useGitHubService(): GitHubService {
  return instance;
}
