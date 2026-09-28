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

// Error response
export function watchBuildsStub(err: { message: string; status: number }): void;

// Static snapshot response
export function watchBuildsStub(snapshot: BuildSnapshot['functions']): void;

// Dynamic stream from async iterable (for state transition tests)
export function watchBuildsStub(iterable: AsyncIterable<BuildSnapshot['functions']>): void;

export function watchBuildsStub(
  val:
    | BuildSnapshot['functions']
    | { message: string; status: number }
    | AsyncIterable<BuildSnapshot['functions']>,
) {
  if (typeof val === 'object' && Symbol.asyncIterator in val) {
    watchBuildsStreamStub(val);
  } else if ('message' in val && 'status' in val) {
    watchBuildsErrorStub(val as { message: string; status: number });
  } else {
    watchBuildsSnapshotStub(val as BuildSnapshot['functions']);
  }
}

function watchBuildsErrorStub(err: { message: string; status: number }) {
  server.use(
    http.get(`${BACKEND_API}/api/v1/func/build/watch`, () =>
      HttpResponse.json({ error: err.message }, { status: err.status }),
    ),
  );
}

function watchBuildsSnapshotStub(snapshot: BuildSnapshot['functions']) {
  const encoder = new TextEncoder();
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      const frame = `event: build-status\ndata: ${JSON.stringify({ functions: snapshot })}\n\n`;
      controller.enqueue(encoder.encode(frame));
    },
  });
  server.use(
    http.get(
      `${BACKEND_API}/api/v1/func/build/watch`,
      () =>
        new Response(stream, {
          headers: { 'Content-Type': 'text/event-stream' },
        }),
    ),
  );
}

function watchBuildsStreamStub(iterable: AsyncIterable<BuildSnapshot['functions']>) {
  server.use(
    http.get(`${BACKEND_API}/api/v1/func/build/watch`, async () => {
      const encoder = new TextEncoder();
      const stream = new ReadableStream<Uint8Array>({
        async start(controller) {
          try {
            for await (const functions of iterable) {
              const frame = `event: build-status\ndata: ${JSON.stringify({ functions })}\n\n`;
              controller.enqueue(encoder.encode(frame));
            }
            controller.close();
          } catch (e) {
            controller.error(e);
          }
        },
      });
      return new Response(stream, {
        headers: { 'Content-Type': 'text/event-stream' },
      });
    }),
  );
}
