import { FileEntry, FunctionConfig } from '../types';

export interface FunctionService {
  generateFunction(config: FunctionConfig): Promise<FileEntry[]>;
}
