import { GithubService } from './GithubService';
import { FileEntry, RepoInfo, SourceRepo } from '../types';

const mockSearch = jest.fn();
const mockGetContent = jest.fn();
const mockCreateBlob = jest.fn();
const mockCreateTree = jest.fn();
const mockCreateCommit = jest.fn();
const mockCreateRef = jest.fn();

jest.mock('@octokit/rest', () => ({
  Octokit: jest.fn().mockImplementation(() => ({
    users: { getAuthenticated: jest.fn().mockResolvedValue({ data: { login: 'twoGiants' } }) },
    search: { repos: mockSearch },
    repos: { getContent: mockGetContent },
    git: {
      createBlob: mockCreateBlob,
      createTree: mockCreateTree,
      createCommit: mockCreateCommit,
      createRef: mockCreateRef,
    },
  })),
}));

afterEach(() => {
  jest.restoreAllMocks();
});

describe('GithubService', () => {
  it('lists function repos tagged with serverless-function topic', async () => {
    mockSearch.mockResolvedValue({
      data: {
        items: [
          {
            owner: { login: 'twoGiants' },
            name: 'my-func',
            html_url: 'https://github.com/twoGiants/my-func',
            default_branch: 'main',
          },
        ],
      },
    });

    const svc = new GithubService('fake-token');
    const repos: SourceRepo[] = await svc.listFunctionRepos();

    expect(repos).toEqual([
      {
        owner: 'twoGiants',
        name: 'my-func',
        url: 'https://github.com/twoGiants/my-func',
        defaultBranch: 'main',
      },
    ]);
    expect(mockSearch).toHaveBeenCalledWith({ q: 'topic:serverless-function user:twoGiants' });
  });

  it('fetches file content from a repo', async () => {
    mockGetContent.mockResolvedValue({
      data: { content: btoa('name: my-func\nruntime: go\n'), encoding: 'base64' },
    });

    const svc = new GithubService('fake-token');
    const content = await svc.fetchFileContent(
      {
        owner: 'twoGiants',
        name: 'my-func',
        url: 'https://github.com/twoGiants/my-func',
        defaultBranch: 'main',
      },
      'func.yaml',
    );

    expect(content).toBe('name: my-func\nruntime: go\n');
    expect(mockGetContent).toHaveBeenCalledWith({
      owner: 'twoGiants',
      repo: 'my-func',
      path: 'func.yaml',
    });
  });

  describe('push', () => {
    const repoInfo: RepoInfo = { owner: 'twoGiants', repo: 'my-func', branch: 'main' };
    const files: FileEntry[] = [
      { path: 'func.yaml', mode: '100644', content: 'name: my-func', type: 'blob' },
    ];

    beforeEach(() => {
      mockCreateBlob.mockResolvedValue({ data: { sha: 'blob-sha-123' } });
      mockCreateTree.mockResolvedValue({ data: { sha: 'tree-sha-123' } });
      mockCreateCommit.mockResolvedValue({ data: { sha: 'commit-sha-123' } });
      mockCreateRef.mockResolvedValue({});
    });

    it('creates an initial commit with the provided files', async () => {
      const svc = new GithubService('fake-token');
      await svc.push(repoInfo, files, 'Initialize function');

      expect(mockCreateBlob).toHaveBeenCalledWith({
        owner: 'twoGiants',
        repo: 'my-func',
        content: 'name: my-func',
        encoding: 'utf-8',
      });
      expect(mockCreateTree).toHaveBeenCalledWith({
        owner: 'twoGiants',
        repo: 'my-func',
        tree: [{ path: 'func.yaml', mode: '100644', type: 'blob', sha: 'blob-sha-123' }],
      });
      expect(mockCreateCommit).toHaveBeenCalledWith({
        owner: 'twoGiants',
        repo: 'my-func',
        message: 'Initialize function',
        tree: 'tree-sha-123',
        parents: [],
      });
      expect(mockCreateRef).toHaveBeenCalledWith({
        owner: 'twoGiants',
        repo: 'my-func',
        ref: 'refs/heads/main',
        sha: 'commit-sha-123',
      });
    });

    it('propagates errors from intermediate API calls', async () => {
      mockCreateTree.mockRejectedValue(new Error('Validation Failed'));
      const svc = new GithubService('fake-token');

      await expect(svc.push(repoInfo, files, 'Initialize function')).rejects.toThrow(
        'Validation Failed',
      );
    });
  });
});
