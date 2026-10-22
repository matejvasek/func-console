import { test, expect } from '../../fixtures/authenticated-page';
import { navigateToFunctionsList } from '../../helpers/navigation';
import { deleteRepoOnFakeGithub, seedRepo, setWorkflowRun } from '../../helpers/fakegithub';
import { simulateGitHubActionsDeploy, deleteFunction } from '../../helpers/cluster';
import { E2E_USER, PRESEEDED_FUNC_NAMESPACE } from '../../helpers/constants';

const FUNC_NAME = 'workflow-status-func';
const BRANCH = 'main';

test.describe('Workflow run status', () => {
  test.beforeEach(async () => {
    await seedRepo(
      E2E_USER,
      FUNC_NAME,
      BRANCH,
      ['serverless-function'],
      [
        {
          path: 'func.yaml',
          mode: '100644',
          content: `name: ${FUNC_NAME}\nruntime: node\nnamespace: ${PRESEEDED_FUNC_NAMESPACE}\n`,
        },
      ],
    );
  });

  test.afterEach(async () => {
    await deleteRepoOnFakeGithub(E2E_USER, FUNC_NAME);
  });

  test('shows a workflow failure indicator when the deploy workflow fails', async ({ page }) => {
    await test.step('trigger a workflow run', async () => {
      await setWorkflowRun(E2E_USER, FUNC_NAME, BRANCH, {
        headSha: 'sha-in-progress',
        status: 'in_progress',
      });
    });

    await test.step('verify the function shows a progress indicator', async () => {
      await navigateToFunctionsList(page);
      const grid = page.getByRole('grid', { name: 'Functions' });
      await expect(grid).toBeVisible({ timeout: 30_000 });

      const row = grid.locator(`tbody tr:has(td:text-is("${FUNC_NAME}"))`);
      await expect(row).toBeVisible();
      // NotDeployed is always primary; workflow activity is a secondary indicator
      await expect(row.getByText('Not deployed')).toBeVisible({ timeout: 20_000 });
      await expect(row.getByRole('img', { name: /workflow run in progress/i })).toBeVisible({
        timeout: 20_000,
      });
    });

    await test.step('verify the failure indicator appears after the workflow fails', async () => {
      // The list streams over SSE (~3s poll cadence), so the status should update
      // without a manual refresh.
      await setWorkflowRun(E2E_USER, FUNC_NAME, BRANCH, {
        headSha: 'sha-failed',
        status: 'completed',
        conclusion: 'failure',
      });

      const grid = page.getByRole('grid', { name: 'Functions' });
      const row = grid.locator(`tbody tr:has(td:text-is("${FUNC_NAME}"))`);
      // NotDeployed stays primary; failure shown as secondary indicator with link
      await expect(row.getByText('Not deployed')).toBeVisible({ timeout: 20_000 });
      const runLink = row.locator('a[href*="/actions/runs/"]');
      await expect(runLink).toBeVisible();
    });
  });

  test('clears the progress indicator after the workflow succeeds', async ({ page }) => {
    await test.step('trigger a workflow run', async () => {
      await setWorkflowRun(E2E_USER, FUNC_NAME, BRANCH, {
        headSha: 'sha-in-progress',
        status: 'in_progress',
      });
    });

    await test.step('verify the progress indicator is visible', async () => {
      await navigateToFunctionsList(page);
      const grid = page.getByRole('grid', { name: 'Functions' });
      await expect(grid).toBeVisible({ timeout: 30_000 });

      const row = grid.locator(`tbody tr:has(td:text-is("${FUNC_NAME}"))`);
      await expect(row.getByRole('img', { name: /workflow run in progress/i })).toBeVisible({
        timeout: 20_000,
      });
    });

    await test.step('verify the indicator disappears after the workflow succeeds', async () => {
      await setWorkflowRun(E2E_USER, FUNC_NAME, BRANCH, {
        headSha: 'sha-success',
        status: 'completed',
        conclusion: 'success',
      });

      const grid = page.getByRole('grid', { name: 'Functions' });
      const row = grid.locator(`tbody tr:has(td:text-is("${FUNC_NAME}"))`);
      // Spinner disappears, NotDeployed remains (no stale Deploying)
      await expect(row.getByRole('img', { name: /workflow run in progress/i })).not.toBeVisible({
        timeout: 20_000,
      });
      await expect(row.getByText('Not deployed')).toBeVisible();
    });
  });

  test('preserves running status when a redeployment workflow fails', async ({ page }) => {
    await test.step('deploy the function to the cluster', async () => {
      await simulateGitHubActionsDeploy(page, FUNC_NAME, PRESEEDED_FUNC_NAMESPACE, 'node');
    });

    await test.step('fail a redeployment workflow', async () => {
      await setWorkflowRun(E2E_USER, FUNC_NAME, BRANCH, {
        headSha: 'sha-redeploy-failed',
        status: 'completed',
        conclusion: 'failure',
      });
    });

    await test.step('verify the function is still running with a failure indicator', async () => {
      await navigateToFunctionsList(page);
      const grid = page.getByRole('grid', { name: 'Functions' });
      await expect(grid).toBeVisible({ timeout: 30_000 });

      const row = grid.locator(`tbody tr:has(td:text-is("${FUNC_NAME}"))`);
      // Running is preserved (non-destructive); failure is a secondary indicator
      await expect(row.getByText('Running')).toBeVisible({ timeout: 20_000 });
      const runLink = row.locator('a[href*="/actions/runs/"]');
      await expect(runLink).toBeVisible({ timeout: 20_000 });
    });

    await test.step('cleanup', async () => {
      await deleteFunction(page, FUNC_NAME, PRESEEDED_FUNC_NAMESPACE);
    });
  });
});
