import { Page } from '@playwright/test';

const COVERAGE_DIR = '.e2e/coverage';

export function isCoverageEnabled(): boolean {
  return process.env.E2E_COVERAGE === 'true';
}

// Shared MCR options. The same config must be used for both add() and generate()
// so that entry filtering and source-map remapping are applied consistently.
function coverageOptions() {
  return {
    outputDir: COVERAGE_DIR,
    reports: ['v8', 'console-summary', 'lcov'],

    // Keep only JS entries served by the plugin proxy (not the console shell itself).
    entryFilter: (entry: { url: string }) => entry.url.includes('console-functions-plugin'),

    // After sourcePath remapping, keep only our TypeScript source files.
    sourceFilter: (sourcePath: string) =>
      sourcePath.startsWith('src/') && /\.tsx?$/.test(sourcePath),

    // MCR strips the webpack:// protocol before calling sourcePath, so paths
    // arrive as "console-functions-plugin/common/services/foo.ts". Webpack
    // context is src/, so we replace the plugin prefix with src/.
    sourcePath: (filePath: string) => filePath.replace(/^console-functions-plugin\//, 'src/'),
  };
}

export async function startCoverage(page: Page): Promise<void> {
  if (!isCoverageEnabled()) return;
  await page.coverage.startJSCoverage({ resetOnNavigation: false });
}

export async function stopAndAddCoverage(page: Page): Promise<void> {
  if (!isCoverageEnabled()) return;

  const entries = await page.coverage.stopJSCoverage();
  const { CoverageReport } = await import('monocart-coverage-reports');
  const report = new CoverageReport(coverageOptions());
  await report.add(entries);
}

export async function cleanCoverageCache(): Promise<void> {
  if (!isCoverageEnabled()) return;

  const { CoverageReport } = await import('monocart-coverage-reports');
  const report = new CoverageReport(coverageOptions());
  await report.cleanCache();
}

export async function generateCoverageReport(): Promise<void> {
  if (!isCoverageEnabled()) return;

  const { CoverageReport } = await import('monocart-coverage-reports');
  const report = new CoverageReport(coverageOptions());
  await report.generate();
}
