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
	startWatchStream := func(stub scm.Client, factory ticker.Factory) *bufio.Reader {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /watch", buildWatchWithStub(stub, handler.WithHeartbeatTickerFactory(factory)))
		ts := httptest.NewServer(mux)
		DeferCleanup(ts.Close)

		req, err := http.NewRequest(http.MethodGet, ts.URL+"/watch", nil)
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("X-SCM-Token", "pat")
		resp, err := ts.Client().Do(req)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { resp.Body.Close() })
		Expect(resp.Header.Get("Content-Type")).To(Equal("text/event-stream"))
		return bufio.NewReader(resp.Body)
	}

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
		ch := make(chan scm.WorkflowRunsOrErr)
		stub := &scm.ClientStub{
			OnWatchWorkflowRuns: func(ctx context.Context, workflowFile string) (scm.WorkflowWatch, error) {
				return &scm.StubWatch{C: ch}, nil
			},
		}

		beat, factory := ticker.CreateFakeTickerFactory()
		reader := startWatchStream(stub, factory)

		go beat()
		line, ok := readLineWithin(reader)
		Expect(ok).To(BeTrue(), "expected a heartbeat line")
		Expect(line).To(Equal(":"))
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
		firstData, ok := readSSEDataWithin(reader)
		Expect(ok).To(BeTrue(), "expected a frame for the first snapshot")

		var first map[string]handler.WorkflowRunDTO
		Expect(json.Unmarshal([]byte(firstData), &first)).To(Succeed())
		Expect(first).To(HaveKey("alice/fn"))
		Expect(first["alice/fn"].Status).To(Equal("Building"))
		Expect(first["alice/fn"].URL).To(BeEmpty())
		Expect(first["alice/fn"].Error).To(BeEmpty())

		ch <- scm.WorkflowRunsOrErr{
			Runs: map[string]scm.WorkflowRun{
				"alice/fn": {
					BuildStatus: scm.Failed,
					HTMLURL:     "https://github.com/alice/fn/actions/runs/1",
				},
			},
		}
		secondData, ok := readSSEDataWithin(reader)
		Expect(ok).To(BeTrue(), "expected a frame for the second snapshot")

		var second map[string]handler.WorkflowRunDTO
		Expect(json.Unmarshal([]byte(secondData), &second)).To(Succeed())
		Expect(second["alice/fn"].Status).To(Equal("Failed"))
		Expect(second["alice/fn"].URL).To(Equal("https://github.com/alice/fn/actions/runs/1"))
		Expect(second["alice/fn"].Error).To(BeEmpty())
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
		frameData, ok := readSSEDataWithin(reader)
		Expect(ok).To(BeTrue(), "expected a frame for the snapshot")

		var frame map[string]handler.WorkflowRunDTO
		Expect(json.Unmarshal([]byte(frameData), &frame)).To(Succeed())
		Expect(frame["alice/fn"].Status).To(Equal("None"))
		Expect(frame["alice/fn"].URL).To(BeEmpty())
		Expect(frame["alice/fn"].Error).To(BeEmpty())
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
		firstData, ok := readSSEDataWithin(reader)
		Expect(ok).To(BeTrue(), "expected an initial frame")

		var first map[string]handler.WorkflowRunDTO
		Expect(json.Unmarshal([]byte(firstData), &first)).To(Succeed())
		Expect(first["alice/fn"].Status).To(Equal("Building"))

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

		// Verify the error event is sent
		line, ok := readLineWithin(reader)
		Expect(ok).To(BeTrue(), "expected an error event line")
		Expect(line).To(Equal("event: app-error"))

		dataLine, ok := readLineWithin(reader)
		Expect(ok).To(BeTrue(), "expected a data line")
		Expect(dataLine).To(HavePrefix("data: "))

		var errorEvent handler.ErrorDTO
		jsonStr := strings.TrimPrefix(dataLine, "data: ")
		Expect(json.Unmarshal([]byte(jsonStr), &errorEvent)).To(Succeed())
		Expect(errorEvent.Message).To(Equal("Unable to fetch build status. Please try again later."))
		Expect(errorEvent.IsAuthError).To(BeFalse())

		firstData, ok := readSSEDataWithin(reader)
		Expect(ok).To(BeTrue())

		var first map[string]handler.WorkflowRunDTO
		Expect(json.Unmarshal([]byte(firstData), &first)).To(Succeed())
		Expect(first["alice/fn"].Status).To(Equal("Succeeded"))
		Expect(first["alice/fn"].URL).To(Equal("example.com/run/1"))
		Expect(first["alice/fn"].Error).To(BeEmpty())
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

// readSSEDataWithin runs readSSEData with a timeout so a handler that never
// emits fails fast instead of blocking until the spec timeout. It returns the
// payload and true on success, or "" and false if the timeout elapses first.
func readSSEDataWithin(reader *bufio.Reader) (string, bool) {
	ch := make(chan string, 1)
	go func() { ch <- readSSEData(reader) }()
	select {
	case data := <-ch:
		return data, true
	case <-time.After(time.Second * 2):
		return "", false
	}
}

// readLineWithin reads a single line (newline trimmed) with a timeout, so a
// handler that never writes fails fast instead of blocking until the spec
// timeout. Returns "" and false if the timeout elapses first.
func readLineWithin(reader *bufio.Reader) (string, bool) {
	ch := make(chan string, 1)
	go func() {
		line, err := reader.ReadString('\n')
		if err != nil {
			ch <- ""
			return
		}
		ch <- strings.TrimRight(line, "\n")
	}()
	select {
	case line := <-ch:
		return line, true
	case <-time.After(time.Second * 2):
		return "", false
	}
}

// readSSEData reads frames until it finds one with a data: line and returns that payload.
func readSSEData(reader *bufio.Reader) string {
	var data []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return strings.Join(data, "\n")
		}
		line = strings.TrimRight(line, "\n")
		if line == "" {
			if len(data) > 0 {
				return strings.Join(data, "\n")
			}
			continue // heartbeat or blank separator, keep reading
		}
		if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
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
