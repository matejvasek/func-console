import { test, expect } from '../../fixtures/authenticated-page';
import { navigateToFunctionsList } from '../../helpers/navigation';
import { deleteRepoOnFakeGithub, seedRepo, setWorkflowRun } from '../../helpers/fakegithub';
import { E2E_USER, PRESEEDED_FUNC_NAMESPACE } from '../../helpers/constants';

const FUNC_NAME = 'build-status-func';
const BRANCH = 'main';

test.describe('Build status', () => {
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

  test('reflects an in-progress run then a failure over SSE', async ({ page }) => {
    await test.step('script an in-progress run', async () => {
      await setWorkflowRun(E2E_USER, FUNC_NAME, BRANCH, {
        headSha: 'sha-building',
        status: 'in_progress',
      });
    });

    await test.step('navigate to functions list and verify Not deployed with build indicator', async () => {
      await navigateToFunctionsList(page);
      const grid = page.getByRole('grid', { name: 'Functions' });
      await expect(grid).toBeVisible({ timeout: 30_000 });

      const row = grid.locator(`tbody tr:has(td:text-is("${FUNC_NAME}"))`);
      await expect(row).toBeVisible();
      // NotDeployed is always primary; build activity is a secondary indicator
      await expect(row.getByText('Not deployed')).toBeVisible({ timeout: 20_000 });
      await expect(row.getByRole('img', { name: /workflow run in progress/i })).toBeVisible({
        timeout: 20_000,
      });
    });

    await test.step('script a failing run and verify failure indicator over SSE', async () => {
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
});
