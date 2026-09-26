import { http, HttpResponse } from 'msw';
import { BACKEND_API } from '../testing/constants';
import { server } from '../testing/mswServer';
import { FunctionListItem } from '../types';
import { BuildSnapshot } from '../clients/functionsClient';

// -----------------------------------------------------------------------------
// Test Doubles ----------------------------------------------------------------
// -----------------------------------------------------------------------------

export function listFunctionsStub(
  {
    responses,
    errorResponse,
    wait,
  }: {
    responses?: FunctionListItem[];
    errorResponse?: { message: string; status: number };
    wait?: Promise<void>;
  } = {
    responses: [],
  },
) {
  server.use(
    http.get(`${BACKEND_API}/api/v1/func/list`, async ({ request }) => {
      if (errorResponse?.message && errorResponse?.status)
        return HttpResponse.json(
          { message: errorResponse?.message },
          { status: errorResponse.status },
        );

      if (wait) await wait;

      const url = new URL(request.url);

      const all = url.searchParams.get('all');
      if (all === 'true') return HttpResponse.json(responses);

      const namespace = url.searchParams.get('namespace');
      if (!namespace)
        return HttpResponse.json({ message: 'namespace can not be empty' }, { status: 400 });

      return HttpResponse.json(responses?.filter((item) => item.namespace === namespace));
    }),
  );
}

// Overload signature 1 — stream error response
export function watchBuildsStub(err: { message: string; status: number }): void;

// Overload signature 2 — successful SSE stream with build status snapshot
export function watchBuildsStub(snapshot: BuildSnapshot['functions']): void;

export function watchBuildsStub(
  val: BuildSnapshot['functions'] | { message: string; status: number },
) {
  if ('message' in val && 'status' in val) {
    // Error response
    server.use(
      http.get(`${BACKEND_API}/api/v1/func/build/watch`, () =>
        HttpResponse.json({ error: val.message }, { status: val.status }),
      ),
    );
  } else {
    // SSE stream response with snapshot
    const frame = `event: build-status\ndata: ${JSON.stringify({ functions: val })}\n\n`;
    server.use(
      http.get(`${BACKEND_API}/api/v1/func/build/watch`, () =>
        HttpResponse.text(frame, {
          headers: { 'Content-Type': 'text/event-stream' },
        }),
      ),
    );
  }
}
