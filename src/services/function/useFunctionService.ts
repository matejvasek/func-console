import { BackendFunctionService } from './BackendFunctionService';
import { FunctionService } from './FunctionService';

const instance = new BackendFunctionService();

export function useFunctionService(): FunctionService {
  return instance;
}
