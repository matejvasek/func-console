package handler

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift/faas-console-plugin/backend/scm"
)

var _ = Describe("HandleBuildWatch", func() {
	// The poll and rediscover loops are owned by scm.Client.WatchWorkflowRuns
	// (exercised in the github package); the handler only owns SSE transport.
	// Pin the heartbeat far out so it never interleaves with the assertions.
	pinHeartbeat := func() {
		orig := buildHeartbeatInterval
		buildHeartbeatInterval = time.Hour
		DeferCleanup(func() { buildHeartbeatInterval = orig })
	}

	// startWatchStream mounts the handler on a test server, opens the SSE
	// stream, asserts the event-stream content type, and returns a reader over
	// the response body.
	startWatchStream := func() *bufio.Reader {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /watch", (&Handlers{}).HandleBuildWatch)
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

	It("returns 401 without an SCM token", func() {
		withSCMStub(&scm.ClientStub{})
		req := httptest.NewRequest(http.MethodGet, "/watch", nil)
		w := httptest.NewRecorder()
		(&Handlers{}).HandleBuildWatch(w, req)
		Expect(w.Code).To(Equal(http.StatusUnauthorized))
	})

	It("returns 401 when the SCM token is rejected during discovery", func() {
		withSCMStub(&scm.ClientStub{
			OnWatchWorkflowRuns: func(ctx context.Context, workflowFile string) (<-chan []scm.RepoRun, error) {
				return nil, scm.ErrUnauthorized
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/watch", nil)
		req.Header.Set("X-SCM-Token", "pat")
		w := httptest.NewRecorder()
		(&Handlers{}).HandleBuildWatch(w, req)
		Expect(w.Code).To(Equal(http.StatusUnauthorized))
	})

	It("returns 502 when discovery fails with a non-auth error", func() {
		withSCMStub(&scm.ClientStub{
			OnWatchWorkflowRuns: func(ctx context.Context, workflowFile string) (<-chan []scm.RepoRun, error) {
				return nil, errors.New("github unreachable")
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/watch", nil)
		req.Header.Set("X-SCM-Token", "pat")
		w := httptest.NewRecorder()
		(&Handlers{}).HandleBuildWatch(w, req)
		Expect(w.Code).To(Equal(http.StatusBadGateway))
	})

	It("emits a heartbeat comment on the heartbeat interval", func() {
		orig := buildHeartbeatInterval
		buildHeartbeatInterval = 10 * time.Millisecond
		DeferCleanup(func() { buildHeartbeatInterval = orig })

		// The watch never emits a snapshot, so the only output is the heartbeat
		// that keeps the SSE connection alive.
		ch := make(chan []scm.RepoRun)
		withSCMStub(&scm.ClientStub{
			OnWatchWorkflowRuns: func(ctx context.Context, workflowFile string) (<-chan []scm.RepoRun, error) {
				return ch, nil
			},
		})

		reader := startWatchStream()

		line, ok := readLineWithin(reader, 2*time.Second)
		Expect(ok).To(BeTrue(), "expected a heartbeat line")
		Expect(line).To(Equal(":"))
	})

	It("emits an SSE frame per snapshot, keyed by owner/repo in build vocabulary", func() {
		pinHeartbeat()

		ch := make(chan []scm.RepoRun, 4)
		withSCMStub(&scm.ClientStub{
			OnWatchWorkflowRuns: func(ctx context.Context, workflowFile string) (<-chan []scm.RepoRun, error) {
				return ch, nil
			},
		})

		reader := startWatchStream()

		ch <- []scm.RepoRun{{
			Repo: scm.Repo{Owner: "alice", Name: "fn"},
			Run:  &scm.WorkflowRun{Status: "in_progress"},
		}}
		first, ok := readSSEDataWithin(reader, 2*time.Second)
		Expect(ok).To(BeTrue(), "expected a frame for the first snapshot")
		Expect(first).To(ContainSubstring(`"alice/fn":{"buildStatus":"Building"}`))

		ch <- []scm.RepoRun{{
			Repo: scm.Repo{Owner: "alice", Name: "fn"},
			Run: &scm.WorkflowRun{
				Status: "completed", Conclusion: "failure",
				HTMLURL: "https://github.com/alice/fn/actions/runs/1",
			},
		}}
		second, ok := readSSEDataWithin(reader, 2*time.Second)
		Expect(ok).To(BeTrue(), "expected a frame for the second snapshot")
		Expect(second).To(ContainSubstring(`"buildStatus":"Failed"`))
		Expect(second).To(ContainSubstring(`"runURL":"https://github.com/alice/fn/actions/runs/1"`))
	})

	It("reports None for a repo with no run", func() {
		pinHeartbeat()

		ch := make(chan []scm.RepoRun, 1)
		withSCMStub(&scm.ClientStub{
			OnWatchWorkflowRuns: func(ctx context.Context, workflowFile string) (<-chan []scm.RepoRun, error) {
				return ch, nil
			},
		})

		reader := startWatchStream()

		ch <- []scm.RepoRun{{Repo: scm.Repo{Owner: "alice", Name: "fn"}, Run: nil}}
		frame, ok := readSSEDataWithin(reader, 2*time.Second)
		Expect(ok).To(BeTrue(), "expected a frame for the snapshot")
		Expect(frame).To(ContainSubstring(`"alice/fn":{"buildStatus":"None"}`))
	})

	It("ends the stream when the watch channel closes", func() {
		pinHeartbeat()

		ch := make(chan []scm.RepoRun, 1)
		withSCMStub(&scm.ClientStub{
			OnWatchWorkflowRuns: func(ctx context.Context, workflowFile string) (<-chan []scm.RepoRun, error) {
				return ch, nil
			},
		})

		reader := startWatchStream()

		ch <- []scm.RepoRun{{
			Repo: scm.Repo{Owner: "alice", Name: "fn"},
			Run:  &scm.WorkflowRun{Status: "in_progress"},
		}}
		first, ok := readSSEDataWithin(reader, 2*time.Second)
		Expect(ok).To(BeTrue(), "expected an initial frame")
		Expect(first).To(ContainSubstring(`"buildStatus":"Building"`))

		// Closing the channel signals the watch ended (e.g. the token was revoked
		// mid-stream); the handler ends the SSE stream, so the body reaches EOF.
		close(ch)
		errCh := make(chan error, 1)
		go func() {
			for {
				if _, err := reader.ReadString('\n'); err != nil {
					errCh <- err
					return
				}
			}
		}()
		select {
		case err := <-errCh:
			Expect(err).To(MatchError(io.EOF))
		case <-time.After(2 * time.Second):
			Fail("expected the stream to close after the watch channel closed")
		}
	})
})

var _ = Describe("deriveBuildStatus", func() {
	DescribeTable("maps run status and conclusion to a build status",
		func(status, conclusion, expected string) {
			Expect(deriveBuildStatus(&scm.WorkflowRun{Status: status, Conclusion: conclusion})).To(Equal(expected))
		},
		Entry("queued -> Building", "queued", "", "Building"),
		Entry("in_progress -> Building", "in_progress", "", "Building"),
		Entry("waiting -> Building", "waiting", "", "Building"),
		Entry("requested -> Building", "requested", "", "Building"),
		Entry("pending -> Building", "pending", "", "Building"),
		Entry("completed+success -> Succeeded", "completed", "success", "Succeeded"),
		Entry("completed+failure -> Failed", "completed", "failure", "Failed"),
		Entry("completed+cancelled -> Failed", "completed", "cancelled", "Failed"),
		Entry("completed+timed_out -> Failed", "completed", "timed_out", "Failed"),
		Entry("completed+skipped -> None", "completed", "skipped", "None"),
		Entry("completed+neutral -> None", "completed", "neutral", "None"),
		Entry("completed+stale -> None", "completed", "stale", "None"),
		Entry("completed+action_required -> None", "completed", "action_required", "None"),
		Entry("unknown status -> None", "bogus", "", "None"),
	)

	It("maps a nil run to None", func() {
		Expect(deriveBuildStatus(nil)).To(Equal("None"))
	})
})

// readSSEDataWithin runs readSSEData with a timeout so a handler that never
// emits fails fast instead of blocking until the spec timeout. It returns the
// payload and true on success, or "" and false if the timeout elapses first.
func readSSEDataWithin(reader *bufio.Reader, timeout time.Duration) (string, bool) {
	ch := make(chan string, 1)
	go func() { ch <- readSSEData(reader) }()
	select {
	case data := <-ch:
		return data, true
	case <-time.After(timeout):
		return "", false
	}
}

// readLineWithin reads a single line (newline trimmed) with a timeout, so a
// handler that never writes fails fast instead of blocking until the spec
// timeout. Returns "" and false if the timeout elapses first.
func readLineWithin(reader *bufio.Reader, timeout time.Duration) (string, bool) {
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
	case <-time.After(timeout):
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
