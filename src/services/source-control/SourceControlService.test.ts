import { GithubService } from './GithubService';
import { SourceRepo } from '../types';

const mockSearch = jest.fn();
const mockGetContent = jest.fn();

jest.mock('@octokit/rest', () => ({
  Octokit: jest.fn().mockImplementation(() => ({
    users: { getAuthenticated: jest.fn().mockResolvedValue({ data: { login: 'twoGiants' } }) },
    search: { repos: mockSearch },
    repos: { getContent: mockGetContent },
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
});
