package github

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift/faas-console-plugin/backend/scm"
)

var _ = Describe("WatchWorkflowRuns", func() {
	// Drive the poll loop fast; push rediscover out unless a test needs it.
	pinWatch := func(poll, rediscover time.Duration) {
		origPoll, origRe := watchPollInterval, watchRediscoverInterval
		watchPollInterval = poll
		watchRediscoverInterval = rediscover
		DeferCleanup(func() {
			watchPollInterval = origPoll
			watchRediscoverInterval = origRe
		})
	}

	It("revalidates each poll with If-None-Match so unchanged runs cost a free 304", func() {
		pinWatch(10*time.Millisecond, time.Hour)
		var mu sync.Mutex
		var conditional []string
		cl := newClient(watchFake("alice", []map[string]any{repoItem("alice", "fn", "main")},
			func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				inm := r.Header.Get("If-None-Match")
				conditional = append(conditional, inm)

				w.Header().Set("ETag", `"run-etag-v1"`)
				// A "fresh" response (like GitHub's max-age=60). The client must
				// still revalidate on every poll, otherwise a new build would be
				// hidden behind this window. This guards the forceRevalidate wrap.
				w.Header().Set("Cache-Control", "max-age=60")
				if inm == `"run-etag-v1"` {
					w.WriteHeader(http.StatusNotModified)
					return
				}
				writeRuns(w, map[string]any{"id": 42, "status": "in_progress"})
			}))

		ctx, cancel := context.WithCancel(context.Background())
		DeferCleanup(cancel)
		ch, err := cl.WatchWorkflowRuns(ctx, "func-deploy.yaml")
		Expect(err).NotTo(HaveOccurred())

		first, ok := recvWithin(ch, 2*time.Second)
		Expect(ok).To(BeTrue(), "expected an initial snapshot")
		Expect(first[0].Run.Status).To(Equal("in_progress"))

		// Let several poll cycles run.
		Eventually(func() int {
			mu.Lock()
			defer mu.Unlock()
			return len(conditional)
		}, 2*time.Second, 10*time.Millisecond).Should(BeNumerically(">=", 3))

		// The run never changes, so a working cache serves each 304 as the same
		// run and the snapshot never re-emits. A broken cache would yield an
		// empty 304 body (nil run) and a spurious re-emit.
		_, ok = recvWithin(ch, 300*time.Millisecond)
		Expect(ok).To(BeFalse(), "expected no re-emit while the 304s serve cached data")

		mu.Lock()
		defer mu.Unlock()
		// The first poll was unconditional; every later poll sent If-None-Match
		// and got a 304.
		Expect(conditional[0]).To(BeEmpty())
		for _, inm := range conditional[1:] {
			Expect(inm).To(Equal(`"run-etag-v1"`))
		}
	})

	It("returns an unauthorized error from the initial discovery", func() {
		pinWatch(10*time.Millisecond, time.Hour)
		cl := newClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"message": "Bad credentials"})
		})

		_, err := cl.WatchWorkflowRuns(context.Background(), "func-deploy.yaml")
		Expect(isUnauthorized(err)).To(BeTrue())
	})

	It("streams an initial snapshot keyed by repo, then re-emits only on change", func() {
		pinWatch(10*time.Millisecond, time.Hour)
		var mu sync.Mutex
		runCalls := 0
		cl := newClient(watchFake("alice", []map[string]any{repoItem("alice", "fn", "main")},
			func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				runCalls++
				if runCalls == 1 {
					writeRuns(w, map[string]any{"id": 1, "status": "in_progress"})
					return
				}
				writeRuns(w, map[string]any{"id": 2, "status": "completed", "conclusion": "success"})
			}))

		ctx, cancel := context.WithCancel(context.Background())
		DeferCleanup(cancel)
		ch, err := cl.WatchWorkflowRuns(ctx, "func-deploy.yaml")
		Expect(err).NotTo(HaveOccurred())

		first, ok := recvWithin(ch, 2*time.Second)
		Expect(ok).To(BeTrue(), "expected an initial snapshot")
		Expect(first).To(HaveLen(1))
		Expect(first[0].Repo.FullName()).To(Equal("alice/fn"))
		Expect(first[0].Run).NotTo(BeNil())
		Expect(first[0].Run.Status).To(Equal("in_progress"))

		second, ok := recvWithin(ch, 2*time.Second)
		Expect(ok).To(BeTrue(), "expected a second snapshot once the run changed")
		Expect(second[0].Run.Status).To(Equal("completed"))
		Expect(second[0].Run.Conclusion).To(Equal("success"))
	})

	It("does not re-emit while the run is unchanged", func() {
		pinWatch(10*time.Millisecond, time.Hour)
		cl := newClient(watchFake("alice", []map[string]any{repoItem("alice", "fn", "main")},
			func(w http.ResponseWriter, r *http.Request) {
				writeRuns(w, map[string]any{"id": 1, "status": "in_progress"})
			}))

		ctx, cancel := context.WithCancel(context.Background())
		DeferCleanup(cancel)
		ch, err := cl.WatchWorkflowRuns(ctx, "func-deploy.yaml")
		Expect(err).NotTo(HaveOccurred())

		_, ok := recvWithin(ch, 2*time.Second)
		Expect(ok).To(BeTrue(), "expected an initial snapshot")
		// Many poll cycles pass (poll is 10ms) with identical runs; the watch
		// suppresses the redundant snapshots.
		_, ok = recvWithin(ch, 300*time.Millisecond)
		Expect(ok).To(BeFalse(), "expected no re-emit while the run is unchanged")
	})

	It("carries a repo's last-known run forward across a transient poll error", func() {
		pinWatch(10*time.Millisecond, time.Hour)
		var mu sync.Mutex
		runCalls := 0
		cl := newClient(watchFake("alice", []map[string]any{repoItem("alice", "fn", "main")},
			func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				runCalls++
				if runCalls == 1 {
					writeRuns(w, map[string]any{"id": 1, "status": "in_progress"})
					return
				}
				// Transient server error on every later poll.
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"message": "boom"})
			}))

		ctx, cancel := context.WithCancel(context.Background())
		DeferCleanup(cancel)
		ch, err := cl.WatchWorkflowRuns(ctx, "func-deploy.yaml")
		Expect(err).NotTo(HaveOccurred())

		first, ok := recvWithin(ch, 2*time.Second)
		Expect(ok).To(BeTrue(), "expected an initial snapshot")
		Expect(first[0].Run.Status).To(Equal("in_progress"))
		// The last-known run is carried forward, so the snapshot is unchanged
		// and nothing new is emitted (no flicker to a nil run).
		_, ok = recvWithin(ch, 300*time.Millisecond)
		Expect(ok).To(BeFalse(), "expected no re-emit while the error is carried forward")
	})

	It("does not flip a failed run's reason to empty when the jobs lookup fails transiently", func() {
		pinWatch(10*time.Millisecond, time.Hour)
		var mu sync.Mutex
		jobsCalls := 0
		cl := newClient(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/user":
				json.NewEncoder(w).Encode(map[string]string{"login": "alice"})
			case r.URL.Path == "/search/repositories":
				json.NewEncoder(w).Encode(map[string]any{
					"total_count": 1,
					"items":       []map[string]any{repoItem("alice", "fn", "main")},
				})
			case strings.Contains(r.URL.Path, "/actions/workflows/"):
				// The runs list is identical on every poll: one failed run.
				writeRuns(w, map[string]any{
					"id": 1, "status": "completed", "conclusion": "failure", "head_sha": "sha1",
				})
			case strings.Contains(r.URL.Path, "/jobs"):
				// The separate failure-reason lookup succeeds once, then fails
				// transiently on every later poll.
				mu.Lock()
				jobsCalls++
				n := jobsCalls
				mu.Unlock()
				if n == 1 {
					json.NewEncoder(w).Encode(map[string]any{
						"jobs": []map[string]any{{
							"id": 1, "name": "build", "status": "completed", "conclusion": "failure",
							"steps": []map[string]any{
								{"name": "go test", "status": "completed", "conclusion": "failure", "number": 1},
							},
						}},
					})
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"message": "boom"})
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		})

		ctx, cancel := context.WithCancel(context.Background())
		DeferCleanup(cancel)
		ch, err := cl.WatchWorkflowRuns(ctx, "func-deploy.yaml")
		Expect(err).NotTo(HaveOccurred())

		first, ok := recvWithin(ch, 2*time.Second)
		Expect(ok).To(BeTrue(), "expected an initial snapshot")
		Expect(first[0].Run).NotTo(BeNil())
		Expect(first[0].Run.Conclusion).To(Equal("failure"))
		Expect(first[0].Run.FailureReason).To(Equal("build / go test"))

		// The run itself never changes; only the best-effort jobs lookup now
		// fails. The failure reason must be carried forward, not flipped to ""
		// and re-emitted (that is exactly the flicker carry-forward prevents).
		_, ok = recvWithin(ch, 300*time.Millisecond)
		Expect(ok).To(BeFalse(),
			"expected no re-emit: a transient jobs-lookup error should not flip FailureReason to empty")
	})

	It("closes the channel when the token is revoked at rediscover", func() {
		pinWatch(10*time.Millisecond, 10*time.Millisecond)
		var mu sync.Mutex
		userCalls := 0
		cl := newClient(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/user":
				mu.Lock()
				userCalls++
				n := userCalls
				mu.Unlock()
				// Initial discovery succeeds; the token is then revoked, so
				// every later discovery is unauthorized.
				if n > 1 {
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode(map[string]string{"message": "Bad credentials"})
					return
				}
				json.NewEncoder(w).Encode(map[string]string{"login": "alice"})
			case r.URL.Path == "/search/repositories":
				json.NewEncoder(w).Encode(map[string]any{
					"total_count": 1,
					"items":       []map[string]any{repoItem("alice", "fn", "main")},
				})
			case strings.Contains(r.URL.Path, "/actions/workflows/"):
				writeRuns(w, map[string]any{"id": 1, "status": "in_progress"})
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		})

		ctx, cancel := context.WithCancel(context.Background())
		DeferCleanup(cancel)
		ch, err := cl.WatchWorkflowRuns(ctx, "func-deploy.yaml")
		Expect(err).NotTo(HaveOccurred())

		_, ok := recvWithin(ch, 2*time.Second)
		Expect(ok).To(BeTrue(), "expected an initial snapshot")

		// The rediscover tick sees the revoked token and ends the watch, which
		// closes the channel.
		Eventually(func() bool {
			select {
			case _, open := <-ch:
				return !open
			case <-time.After(50 * time.Millisecond):
				return false
			}
		}, 2*time.Second, 10*time.Millisecond).Should(BeTrue(), "expected the channel to close")
	})
})

// watchFake routes the minimal endpoints WatchWorkflowRuns needs: the
// authenticated user, the repo search (items), and per-repo workflow runs.
func watchFake(login string, repos []map[string]any, onRuns http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user":
			json.NewEncoder(w).Encode(map[string]string{"login": login})
		case r.URL.Path == "/search/repositories":
			json.NewEncoder(w).Encode(map[string]any{"total_count": len(repos), "items": repos})
		case strings.Contains(r.URL.Path, "/actions/workflows/"):
			onRuns(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

// repoItem builds a single repo entry as returned by the GitHub search API.
func repoItem(owner, name, branch string) map[string]any {
	return map[string]any{
		"name":           name,
		"html_url":       "https://example.com/" + owner + "/" + name,
		"default_branch": branch,
		"owner":          map[string]any{"login": owner},
	}
}

// writeRuns encodes a workflow-runs list response.
func writeRuns(w http.ResponseWriter, runs ...map[string]any) {
	json.NewEncoder(w).Encode(map[string]any{"total_count": len(runs), "workflow_runs": runs})
}

// recvWithin receives one snapshot from ch or times out.
func recvWithin(ch <-chan []scm.RepoRun, timeout time.Duration) ([]scm.RepoRun, bool) {
	select {
	case snap := <-ch:
		return snap, true
	case <-time.After(timeout):
		return nil, false
	}
}
