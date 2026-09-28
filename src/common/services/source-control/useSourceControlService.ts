import { GithubService } from './GithubService';
import { SourceControlService } from './SourceControlService';

const instance = new GithubService(() => sessionStorage.getItem('func-console-pat') || '');

export function useSourceControlService(): SourceControlService {
  return instance;
}
