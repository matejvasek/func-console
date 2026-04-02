import { Octokit } from '@octokit/rest';
import { FileEntry, RepoInfo } from '../types';
import { GitHubService } from './GitHubService';

export class OctokitGitHubService implements GitHubService {
  async pushFiles(
    repo: RepoInfo,
    pat: string,
    files: FileEntry[],
    message: string,
  ): Promise<void> {
    const octokit = new Octokit({ auth: pat });
    const { owner, repo: repoName, branch } = repo;

    // 1. Create blobs for each file in parallel
    const treeEntries = await Promise.all(
      files.map(async (file) => {
        const { data: blob } = await octokit.git.createBlob({
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

    // 2. Create a tree (no base_tree — only our files)
    const { data: tree } = await octokit.git.createTree({
      owner,
      repo: repoName,
      tree: treeEntries,
    });

    // 3. Create an initial commit (no parents)
    const { data: commit } = await octokit.git.createCommit({
      owner,
      repo: repoName,
      message,
      tree: tree.sha,
      parents: [],
    });

    // 4. Create the branch ref
    await octokit.git.createRef({
      owner,
      repo: repoName,
      ref: `refs/heads/${branch}`,
      sha: commit.sha,
    });
  }
}
