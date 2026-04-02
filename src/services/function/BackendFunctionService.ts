import { consoleFetchJSON } from '@openshift-console/dynamic-plugin-sdk';
import { FileEntry, FunctionConfig } from '../types';
import { FunctionService } from './FunctionService';

const PROXY_BASE = '/api/proxy/plugin/console-functions-plugin/backend';

export class BackendFunctionService implements FunctionService {
  async generateFunction(config: FunctionConfig): Promise<FileEntry[]> {
    return consoleFetchJSON.post(`${PROXY_BASE}/api/function/create`, config);
  }
}
