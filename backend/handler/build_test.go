package handler_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/faas-console-plugin/backend/handler"
	"github.com/openshift/faas-console-plugin/backend/scm"
	"github.com/openshift/faas-console-plugin/backend/ticker"
)

var _ = Describe("BuildWatch", func() {

	Describe("failures before stream starts", func() {
		It("returns 401 without an SCM token", func() {
			req := httptest.NewRequest(http.MethodGet, "/watch", nil)
			w := httptest.NewRecorder()
			buildWatchWithStub(&scm.ClientStub{})(w, req)
			Expect(w.Code).To(Equal(http.StatusUnauthorized))
		})

		It("returns 401 when the SCM token is rejected during discovery", func() {
			stub := &scm.ClientStub{
				OnWatchWorkflowRuns: func(ctx context.Context, workflowFile string) (scm.WorkflowWatch, error) {
					return nil, scm.ErrUnauthorized
				},
			}
			req := httptest.NewRequest(http.MethodGet, "/watch", nil)
			req.Header.Set("X-SCM-Token", "pat")
			w := httptest.NewRecorder()
			buildWatchWithStub(stub)(w, req)
			Expect(w.Code).To(Equal(http.StatusUnauthorized))
		})

		It("returns 502 when discovery fails with a non-auth error", func() {
			stub := &scm.ClientStub{
				OnWatchWorkflowRuns: func(ctx context.Context, workflowFile string) (scm.WorkflowWatch, error) {
					return nil, errors.New("github unreachable")
				},
			}
			req := httptest.NewRequest(http.MethodGet, "/watch", nil)
			req.Header.Set("X-SCM-Token", "pat")
			w := httptest.NewRecorder()
			buildWatchWithStub(stub)(w, req)
			Expect(w.Code).To(Equal(http.StatusBadGateway))
		})
	})

	It("emits a heartbeat comment on demand", func() {
		w := scm.StubWatch{C: make(chan scm.WorkflowRunsOrErr)}
		stub := &scm.ClientStub{
			OnWatchWorkflowRuns: func(ctx context.Context, workflowFile string) (scm.WorkflowWatch, error) {
				return &w, nil
			},
		}

		beat, factory := ticker.CreateFakeTickerFactory()
		reader := startWatchStream(stub, factory)

		go func() {
			beat()
			w.Stop()
		}()
		bs, err := io.ReadAll(reader)
		Expect(err).To(BeNil())
		Expect(bs).To(Equal([]byte{':', '\n', '\n'}))
	})

	It("emits an SSE frame per snapshot, keyed by owner/repo in build vocabulary", func() {
		ch := make(chan scm.WorkflowRunsOrErr, 4)
		stub := &scm.ClientStub{
			OnWatchWorkflowRuns: func(ctx context.Context, workflowFile string) (scm.WorkflowWatch, error) {
				return &scm.StubWatch{C: ch}, nil
			},
		}

		reader := startWatchStream(stub, ticker.SilentTickerFactory())

		ch <- scm.WorkflowRunsOrErr{
			Runs: map[string]scm.WorkflowRun{
				"alice/fn": {BuildStatus: scm.Building},
			},
		}
		ch <- scm.WorkflowRunsOrErr{
			Runs: map[string]scm.WorkflowRun{
				"alice/fn": {
					BuildStatus: scm.Failed,
					HTMLURL:     "https://github.com/alice/fn/actions/runs/1",
				},
			},
		}

		events := readWorkflowEventStream(reader)

		first := <-events
		Expect(first.buildStatus).To(HaveKey("alice/fn"))
		Expect(first.buildStatus["alice/fn"].Status).To(Equal("Building"))
		Expect(first.buildStatus["alice/fn"].URL).To(BeEmpty())
		Expect(first.buildStatus["alice/fn"].Error).To(BeEmpty())
		Expect(first.appError).To(BeNil())
		Expect(first.err).To(BeNil())

		second := <-events
		Expect(second.buildStatus["alice/fn"].Status).To(Equal("Failed"))
		Expect(second.buildStatus["alice/fn"].URL).To(Equal("https://github.com/alice/fn/actions/runs/1"))
		Expect(second.buildStatus["alice/fn"].Error).To(BeEmpty())
		Expect(second.appError).To(BeNil())
		Expect(second.err).To(BeNil())
	})

	It("omits the optional fields for a repo with no run", func() {
		ch := make(chan scm.WorkflowRunsOrErr, 1)
		stub := &scm.ClientStub{
			OnWatchWorkflowRuns: func(ctx context.Context, workflowFile string) (scm.WorkflowWatch, error) {
				return &scm.StubWatch{C: ch}, nil
			},
		}

		reader := startWatchStream(stub, ticker.SilentTickerFactory())

		ch <- scm.WorkflowRunsOrErr{
			Runs: map[string]scm.WorkflowRun{"alice/fn": {}},
		}

		events := readWorkflowEventStream(reader)
		evt := <-events
		Expect(evt.buildStatus["alice/fn"].Status).To(Equal("None"))
		Expect(evt.buildStatus["alice/fn"].URL).To(BeEmpty())
		Expect(evt.buildStatus["alice/fn"].Error).To(BeEmpty())
		Expect(evt.appError).To(BeNil())
		Expect(evt.err).To(BeNil())
	})

	It("ends the stream when the watch channel closes", func() {
		ch := make(chan scm.WorkflowRunsOrErr)
		tw := &scm.StubWatch{C: ch}
		stub := &scm.ClientStub{
			OnWatchWorkflowRuns: func(ctx context.Context, workflowFile string) (scm.WorkflowWatch, error) {
				return tw, nil
			},
		}

		reader := startWatchStream(stub, ticker.SilentTickerFactory())

		ch <- scm.WorkflowRunsOrErr{
			Runs: map[string]scm.WorkflowRun{"alice/fn": {BuildStatus: scm.Building}},
		}

		events := readWorkflowEventStream(reader)
		first := <-events
		Expect(first.buildStatus["alice/fn"].Status).To(Equal("Building"))
		Expect(first.err).To(BeNil())

		// Closing the channel signals the watch ended (e.g. the token was revoked
		// mid-stream); the handler ends the SSE stream, so the body reaches EOF.
		tw.Stop()
		errCh := make(chan error, 1)
		go func() {
			_, err := io.Copy(io.Discard, reader)
			errCh <- err
		}()
		select {
		case err := <-errCh:
			Expect(err).To(BeNil())
		case <-time.After(2 * time.Second):
			Fail("expected the stream to close after the watch channel closed")
		}
	})

	It("sends an SSE error event and continues the stream when the watch fails", func() {
		ch := make(chan scm.WorkflowRunsOrErr, 2)
		stub := &scm.ClientStub{
			OnWatchWorkflowRuns: func(ctx context.Context, workflowFile string) (scm.WorkflowWatch, error) {
				return &scm.StubWatch{C: ch}, nil
			},
		}

		reader := startWatchStream(stub, ticker.SilentTickerFactory())

		// Emit an error from the watch
		watchErr := errors.New("github API rate limited")
		ch <- scm.WorkflowRunsOrErr{Err: watchErr}
		ch <- scm.WorkflowRunsOrErr{Runs: map[string]scm.WorkflowRun{
			"alice/fn": {
				BuildStatus: scm.Succeeded,
				HTMLURL:     "example.com/run/1",
			},
		}}

		events := readWorkflowEventStream(reader)

		// Verify the error event is sent
		errorEvt := <-events
		Expect(errorEvt.appError).NotTo(BeNil())
		Expect(errorEvt.appError.Message).To(Equal("Unable to fetch build status. Please try again later."))
		Expect(errorEvt.appError.IsAuthError).To(BeFalse())
		Expect(errorEvt.buildStatus).To(BeNil())
		Expect(errorEvt.err).To(BeNil())

		// Verify the stream continues with build status
		statusEvt := <-events
		Expect(statusEvt.buildStatus).NotTo(BeEmpty())
		Expect(statusEvt.buildStatus["alice/fn"].Status).To(Equal("Succeeded"))
		Expect(statusEvt.buildStatus["alice/fn"].URL).To(Equal("example.com/run/1"))
		Expect(statusEvt.buildStatus["alice/fn"].Error).To(BeEmpty())
		Expect(statusEvt.appError).To(BeNil())
		Expect(statusEvt.err).To(BeNil())
	})

	It("calls watch.Stop() when the request context is cancelled to halt polling", func() {
		ch := make(chan scm.WorkflowRunsOrErr)
		stub := &scm.ClientStub{
			OnWatchWorkflowRuns: func(ctx context.Context, workflowFile string) (scm.WorkflowWatch, error) {
				return &scm.StubWatch{C: ch}, nil
			},
		}

		beat, factory := ticker.CreateFakeTickerFactory()
		mux := http.NewServeMux()
		mux.HandleFunc("GET /watch", buildWatchWithStub(stub, handler.WithHeartbeatTickerFactory(factory)))
		ts := httptest.NewServer(mux)
		DeferCleanup(ts.Close)

		ctx, cancel := context.WithCancel(context.Background())
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/watch", nil)
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("X-SCM-Token", "pat")

		resp, err := ts.Client().Do(req)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { resp.Body.Close() })

		reader := bufio.NewReader(resp.Body)
		go beat()
		_, err = reader.ReadString('\n')
		Expect(err).NotTo(HaveOccurred())

		cancel()

		select {
		case <-ch:
			// Success: Stop was called
		case <-time.After(2 * time.Second):
			Fail("expected watch.Stop() to be called when request context is cancelled")
		}
	})

	It("validate test auxiliaries", func() {
		r := strings.NewReader(`:
event: app-error
data: {"message": "some error",
data: "isAuthError": true}

:

event: build-status
data: {"alice/fn": {"status": "Building"}}

`)

		var events = readWorkflowEventStream(r)
		first := <-events
		Expect(first.appError).NotTo(BeNil())
		Expect(first.appError.Message).To(Equal("some error"))
		Expect(first.appError.IsAuthError).To(BeTrue())
		Expect(first.buildStatus).To(BeEmpty())
		Expect(first.err).To(BeNil())
		second := <-events
		Expect(second.buildStatus).NotTo(BeEmpty())
		Expect(second.buildStatus["alice/fn"].Status).To(Equal("Building"))
		Expect(second.appError).To(BeNil())
		Expect(second.err).To(BeNil())
	})
})

// buildWatchWithStub returns the handler wired to stub instead of the SCM
// registry it defaults to, ignoring the token the way the stubs ignore
// authentication. Any opts are applied after, so a spec can name its heartbeat.
func buildWatchWithStub(stub scm.Client, opts ...handler.WatchOption) http.HandlerFunc {
	withStub := handler.WithSCMFactory(func(string) scm.Client { return stub })
	return handler.BuildWatch(append([]handler.WatchOption{withStub}, opts...)...)
}

type workflowEvent struct {
	buildStatus map[string]handler.WorkflowRunDTO
	appError    *handler.ErrorDTO
	err         error
}

func readWorkflowEventStream(r io.Reader) <-chan workflowEvent {
	ch := make(chan workflowEvent)
	stream := readSSEEventStream(bufio.NewReader(r), map[string]reflect.Type{
		"app-error":    reflect.TypeFor[handler.ErrorDTO](),
		"build-status": reflect.TypeFor[map[string]handler.WorkflowRunDTO](),
	})
	go func() {
		defer close(ch)
		for e := range stream {
			if e.Err != nil {
				ch <- workflowEvent{err: e.Err}
				continue
			}
			switch d := e.Data.(type) {
			case map[string]handler.WorkflowRunDTO:
				ch <- workflowEvent{buildStatus: d}
			case handler.ErrorDTO:
				ch <- workflowEvent{appError: &d}
			default:
				ch <- workflowEvent{err: fmt.Errorf("unexpected event: (name: %q; type: %T)", e.Name, e.Data)}
			}
		}
	}()
	return ch
}

type event struct {
	Name string
	Data any
	Err  error
}

func readSSEEventStream(
	r io.Reader,
	eventMapping map[string]reflect.Type,
) <-chan event {
	out := make(chan event)

	go func() {
		defer close(out)

		for {
			name, raw, err := readSSEEvent[json.RawMessage](r)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					out <- event{Name: name, Err: err}
				}
				return
			}

			typ, ok := eventMapping[name]
			if !ok || typ == nil {
				out <- event{
					Name: name,
					Err:  fmt.Errorf("no data type registered for SSE event %q", name),
				}
				return
			}

			// New(T) yields *T, which Unmarshal can populate.
			// This also works when T itself is a pointer type.
			value := reflect.New(typ)
			if err := json.Unmarshal(raw, value.Interface()); err != nil {
				out <- event{
					Name: name,
					Err:  fmt.Errorf("cannot decode SSE event %q as %v: %w", name, typ, err),
				}
				return
			}

			out <- event{Name: name, Data: value.Elem().Interface()}
		}
	}()

	return out
}

func readSSEEvent[T any](r io.Reader) (name string, data T, err error) {
	var payload bytes.Buffer
	var hasData bool

	var reader = bufio.NewReader(r)

	for {
		line, readErr := readSSELine(reader)
		if readErr != nil {
			// EOF does not dispatch an unterminated event.
			if readErr == io.EOF {
				return "", data, io.EOF
			}
			return "", data, fmt.Errorf("cannot read SSE line: %w", readErr)
		}

		if len(line) == 0 {
			if !hasData {
				// Discard this block, including any event name.
				name = ""
				continue
			}

			if name == "" {
				name = "message"
			}

			// Each data field appends a newline; remove only the last one.
			body := payload.Bytes()
			body = body[:len(body)-1]
			if err := json.Unmarshal(body, &data); err != nil {
				var zero T
				return name, zero, fmt.Errorf("cannot deserialize event JSON: %w", err)
			}
			return name, data, nil
		}

		if line[0] == ':' {
			continue // A comment consumes only its own line.
		}

		var field []byte
		var value []byte
		var ok bool
		if field, value, ok = bytes.Cut(line, []byte{':'}); ok {
			if len(value) > 0 && value[0] == ' ' {
				value = value[1:] // Strip exactly one ASCII space.
			}
		}

		switch string(field) {
		case "event":
			name = string(value)
		case "data":
			hasData = true
			payload.Write(value)
			payload.WriteByte('\n')
		default:
			// Includes id/retry: this API does not expose that metadata.
		}
	}
}

// readSSELine reads one complete line without its terminator.
// Unlike ReadLine, it handles bare CR and has no buffer-fragment boundary.
func readSSELine(r *bufio.Reader) ([]byte, error) {
	var line []byte
	for {
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		switch b {
		case '\n':
			return line, nil
		case '\r':
			// CRLF is one terminator; leave any other byte unread.
			if next, err := r.Peek(1); err == nil && next[0] == '\n' {
				if _, err := r.ReadByte(); err != nil {
					return nil, err
				}
			}
			return line, nil
		default:
			line = append(line, b)
		}
	}
}

func startWatchStream(stub scm.Client, factory ticker.Factory) io.Reader {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /watch", buildWatchWithStub(stub, handler.WithHeartbeatTickerFactory(factory)))
	ts := httptest.NewServer(mux)
	DeferCleanup(ts.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/watch", nil)
	Expect(err).NotTo(HaveOccurred())
	req.Header.Set("X-SCM-Token", "pat")
	resp, err := ts.Client().Do(req) //nolint:bodyclose // DeferCleanup handles response body closure
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() { resp.Body.Close() })
	Expect(resp.Header.Get("Content-Type")).To(Equal("text/event-stream"))
	return resp.Body
}
