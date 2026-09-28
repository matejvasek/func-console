import { http, HttpResponse } from 'msw';

const GITHUB_API = 'https://api.github.com';

/** Default GitHub API handlers. Override per test by using server.use(). */
export const handlers = [
  // GET /user - authenticated user
  http.get(`${GITHUB_API}/user`, () =>
    HttpResponse.json({ login: 'twoGiants' }),
  ),

  // GET /search/repositories - search for function repos
  http.get(`${GITHUB_API}/search/repositories`, () =>
    HttpResponse.json({ total_count: 0, items: [] }),
  ),
];
