export interface FileEntry {
  path: string;
  mode: '100644' | '100755' | '120000';
  content: string;
  type: 'blob';
}

export interface FunctionConfig {
  name: string;
  runtime: FunctionRuntime;
  registry: string;
  namespace: string;
  branch: string;
}

export type FunctionRuntime = 'node' | 'python' | 'go' | 'quarkus';

export interface RepoInfo {
  owner: string;
  repo: string;
  branch: string;
}
