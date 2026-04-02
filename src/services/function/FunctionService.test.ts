import { consoleFetchJSON } from '@openshift-console/dynamic-plugin-sdk';
import { BackendFunctionService } from './BackendFunctionService';
import { FileEntry, FunctionConfig } from '../types';

jest.mock('@openshift-console/dynamic-plugin-sdk', () => ({
  consoleFetchJSON: {
    post: jest.fn(),
  },
}));

afterEach(() => {
  jest.restoreAllMocks();
});

const config: FunctionConfig = {
  name: 'my-func',
  runtime: 'node',
  registry: 'quay.io/test',
  namespace: 'default',
  branch: 'main',
};

const files: FileEntry[] = [
  { path: 'func.yaml', mode: '100644', content: 'name: my-func', type: 'blob' },
  { path: 'index.js', mode: '100644', content: 'module.exports = {}', type: 'blob' },
];

describe('BackendFunctionService', () => {
  it('calls consoleFetchJSON.post with the proxy URL and returns generated files', async () => {
    (consoleFetchJSON.post as jest.Mock).mockResolvedValue(files);

    const service = new BackendFunctionService();
    const result = await service.generateFunction(config);

    expect(consoleFetchJSON.post).toHaveBeenCalledWith(
      '/api/proxy/plugin/console-functions-plugin/backend/api/function/create',
      config,
    );
    expect(result).toEqual(files);
  });

  it('throws when consoleFetchJSON.post rejects', async () => {
    (consoleFetchJSON.post as jest.Mock).mockRejectedValue(
      new Error('failed to initialize function'),
    );

    const service = new BackendFunctionService();

    await expect(service.generateFunction(config)).rejects.toThrow(
      'failed to initialize function',
    );
  });
});
