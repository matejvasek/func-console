import { Octokit } from '@octokit/rest';
import { SourceRepo } from '../types';
import { SourceControlService } from './SourceControlService';

export class GithubService implements SourceControlService {
  private octokit: Octokit;
  private username: string | undefined;

  constructor(pat: string) {
    this.octokit = new Octokit({ auth: pat });
  }

  async listFunctionRepos(): Promise<SourceRepo[]> {
    if (!this.username) {
      const { data: user } = await this.octokit.users.getAuthenticated();
      this.username = user.login;
    }

    const { data } = await this.octokit.search.repos({
      q: `topic:serverless-function user:${this.username}`,
    });

    return data.items.map((item) => ({
      owner: item.owner?.login ?? '',
      name: item.name,
      url: item.html_url,
      defaultBranch: item.default_branch,
    }));
  }

  async fetchFileContent(repo: SourceRepo, path: string): Promise<string> {
    const { data } = await this.octokit.repos.getContent({
      owner: repo.owner,
      repo: repo.name,
      path,
    });

    if (!('content' in data)) {
      throw new Error(`${path} is not a file`);
    }
    return atob(data.content);
  }
}
