import { OctokitGitHubService } from './OctokitGitHubService';
import { FileEntry, RepoInfo } from '../types';

const mockCreateBlob = jest.fn();
const mockCreateTree = jest.fn();
const mockCreateCommit = jest.fn();
const mockCreateRef = jest.fn();

jest.mock('@octokit/rest', () => ({
  Octokit: jest.fn().mockImplementation(() => ({
    git: {
      createBlob: mockCreateBlob,
      createTree: mockCreateTree,
      createCommit: mockCreateCommit,
      createRef: mockCreateRef,
    },
  })),
}));

afterEach(() => {
  jest.clearAllMocks();
});

const repo: RepoInfo = { owner: 'testuser', repo: 'my-repo', branch: 'main' };
const pat = 'ghp_test123';
const files: FileEntry[] = [
  { path: 'func.yaml', mode: '100644', content: 'name: my-func', type: 'blob' },
];

describe('OctokitGitHubService', () => {
  beforeEach(() => {
    mockCreateBlob.mockResolvedValue({
      data: { sha: 'blob-sha-123' },
    });
    mockCreateTree.mockResolvedValue({
      data: { sha: 'tree-sha-123' },
    });
    mockCreateCommit.mockResolvedValue({
      data: { sha: 'commit-sha-123' },
    });
    mockCreateRef.mockResolvedValue({});
  });

  it('creates an initial commit with the generated files', async () => {
    const service = new OctokitGitHubService();
    await service.pushFiles(repo, pat, files, 'Initialize function');

    expect(mockCreateBlob).toHaveBeenCalledWith({
      owner: 'testuser',
      repo: 'my-repo',
      content: 'name: my-func',
      encoding: 'utf-8',
    });
    expect(mockCreateTree).toHaveBeenCalledWith({
      owner: 'testuser',
      repo: 'my-repo',
      tree: [{ path: 'func.yaml', mode: '100644', type: 'blob', sha: 'blob-sha-123' }],
    });
  });

  it('propagates errors from intermediate API calls', async () => {
    mockCreateTree.mockRejectedValue(new Error('Validation Failed'));
    const service = new OctokitGitHubService();

    await expect(
      service.pushFiles(repo, pat, files, 'Initialize function'),
    ).rejects.toThrow('Validation Failed');
  });
});
