import { GithubService } from './GithubService';
import { SourceControlService } from './SourceControlService';

// PAT injected via webpack DefinePlugin from GITHUB_PAT env variable.
// For dev/testing: export GITHUB_PAT=ghp_... before running yarn start.
// DO NOT hardcode a real PAT here — this file is committed.
const pat = typeof __GITHUB_PAT__ !== 'undefined' ? __GITHUB_PAT__ : '';
const instance = new GithubService(pat);

export function useSourceControlService(): SourceControlService {
  return instance;
}
