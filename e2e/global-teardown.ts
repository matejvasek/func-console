import { generateCoverageReport } from './helpers/coverage';

export default async function globalTeardown() {
  await generateCoverageReport();
}
