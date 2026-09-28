import { FunctionBackendService } from './FunctionBackendService';
import { FunctionService } from './FunctionService';

const instance = new FunctionBackendService();

export function useFunctionService(): FunctionService {
  return instance;
}
