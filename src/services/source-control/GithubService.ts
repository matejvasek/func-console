import { Octokit } from '@octokit/rest';
import { FileEntry, RepoInfo, SourceRepo } from '../types';
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

  async push(repo: RepoInfo, files: FileEntry[], message: string): Promise<void> {
    const { owner, repo: repoName, branch } = repo;

    const treeEntries = await Promise.all(
      files.map(async (file) => {
        const { data: blob } = await this.octokit.git.createBlob({
          owner,
          repo: repoName,
          content: file.content,
          encoding: 'utf-8',
        });
        return {
          path: file.path,
          mode: file.mode,
          type: file.type as 'blob',
          sha: blob.sha,
        };
      }),
    );

    const { data: tree } = await this.octokit.git.createTree({
      owner,
      repo: repoName,
      tree: treeEntries,
    });

    const { data: commit } = await this.octokit.git.createCommit({
      owner,
      repo: repoName,
      message,
      tree: tree.sha,
      parents: [],
    });

    await this.octokit.git.createRef({
      owner,
      repo: repoName,
      ref: `refs/heads/${branch}`,
      sha: commit.sha,
    });
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
