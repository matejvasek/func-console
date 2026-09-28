import { getLanguageFromPath } from './utils';

describe('getLanguageFromPath', () => {
  it.each([
    ['index.js', 'javascript'],
    ['handler.ts', 'typescript'],
    ['main.go', 'go'],
    ['app.py', 'python'],
    ['func.yaml', 'yaml'],
    ['config.yml', 'yaml'],
    ['package.json', 'json'],
    ['README.md', 'markdown'],
    ['Dockerfile', 'dockerfile'],
    ['.gitignore', 'plaintext'],
    ['Makefile', 'plaintext'],
    ['', 'plaintext'],
  ])('returns correct language for %s', (path, expected) => {
    expect(getLanguageFromPath(path)).toBe(expected);
  });
});
