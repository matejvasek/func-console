package handler

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/faas-console-plugin/backend/scm"
)

// subscribeResult carries a Subscribe outcome off the goroutine that made the
// call, so assertions stay on the spec's goroutine.
type subscribeResult struct {
	watch scm.WorkflowWatch
	err   error
}

// newWatchStub returns an upstream whose channel is unbuffered, so a send from
// a spec only completes once the run loop has taken the event.
func newWatchStub() *scm.WorkflowWatchStub {
	return &scm.WorkflowWatchStub{C: make(chan scm.WorkflowRunsEvent)}
}

func runsEvent(url string) scm.WorkflowRunsEvent {
	return scm.WorkflowRunsEvent{
		Runs: map[string]scm.WorkflowRun{"acme/fn": {Status: scm.Succeeded, HTMLURL: url}},
	}
}

// fixedUpstream builds a client factory that hands the same watch to every
// setup and records how many times it was asked for one.
func fixedUpstream(w scm.WorkflowWatch, setups chan<- struct{}) scm.ClientFactory {
	return func(string) scm.Client {
		return &scm.ClientStub{OnWatchWorkflowRuns: func(context.Context, string) (scm.WorkflowWatch, error) {
			if setups != nil {
				setups <- struct{}{}
			}
			return w, nil
		}}
	}
}

var _ = Describe("workflowWatchSharer", func() {

	It("shares a single upstream between subscribers with the same token", func() {
		setups := make(chan struct{}, 4)
		upstream := newWatchStub()
		sharer := newWorkflowWatchSharer(fixedUpstream(upstream, setups))
		DeferCleanup(sharer.Close)

		first, err := sharer.Subscribe(context.Background(), "pat")
		Expect(err).NotTo(HaveOccurred())
		second, err := sharer.Subscribe(context.Background(), "pat")
		Expect(err).NotTo(HaveOccurred())
		Expect(setups).To(HaveLen(1))

		event := runsEvent("run-1")
		upstream.C <- event
		Eventually(first.ResultChan()).Should(Receive(Equal(event)))
		Eventually(second.ResultChan()).Should(Receive(Equal(event)))
	})

	It("keeps the watches for different tokens apart", func() {
		upstreams := map[string]*scm.WorkflowWatchStub{
			"pat-a": newWatchStub(),
			"pat-b": newWatchStub(),
		}
		sharer := newWorkflowWatchSharer(func(token string) scm.Client {
			w := upstreams[token]
			return &scm.ClientStub{OnWatchWorkflowRuns: func(context.Context, string) (scm.WorkflowWatch, error) {
				return w, nil
			}}
		})
		DeferCleanup(sharer.Close)

		a, err := sharer.Subscribe(context.Background(), "pat-a")
		Expect(err).NotTo(HaveOccurred())
		b, err := sharer.Subscribe(context.Background(), "pat-b")
		Expect(err).NotTo(HaveOccurred())

		event := runsEvent("run-a")
		upstreams["pat-a"].C <- event
		Eventually(a.ResultChan()).Should(Receive(Equal(event)))

		// Delivery to a is proof the run loop handled the event, so b has had its
		// chance to receive it and did not. One token cannot see another's runs.
		Expect(b.ResultChan()).NotTo(Receive())
	})

	It("coalesces subscribers that arrive while the upstream is still being set up", func() {
		setups := make(chan struct{}, 4)
		release := make(chan struct{})
		upstream := newWatchStub()
		sharer := newWorkflowWatchSharer(func(string) scm.Client {
			return &scm.ClientStub{OnWatchWorkflowRuns: func(context.Context, string) (scm.WorkflowWatch, error) {
				setups <- struct{}{}
				<-release
				return upstream, nil
			}}
		})
		DeferCleanup(sharer.Close)

		first := make(chan subscribeResult, 1)
		go func() {
			w, err := sharer.Subscribe(context.Background(), "pat")
			first <- subscribeResult{watch: w, err: err}
		}()
		Eventually(setups).Should(HaveLen(1))

		second := make(chan subscribeResult, 1)
		go func() {
			w, err := sharer.Subscribe(context.Background(), "pat")
			second <- subscribeResult{watch: w, err: err}
		}()

		// Setup is parked, so neither call can be answered yet. Waiting here also
		// leaves the run loop time to take the second request and put it on the
		// entry's pending list, which is the path under test.
		Consistently(second).ShouldNot(Receive())
		close(release)

		var r1, r2 subscribeResult
		Eventually(first).Should(Receive(&r1))
		Eventually(second).Should(Receive(&r2))
		Expect(r1.err).NotTo(HaveOccurred())
		Expect(r2.err).NotTo(HaveOccurred())
		Expect(setups).To(HaveLen(1))

		event := runsEvent("run-1")
		upstream.C <- event
		Eventually(r1.watch.ResultChan()).Should(Receive(Equal(event)))
		Eventually(r2.watch.ResultChan()).Should(Receive(Equal(event)))
	})

	It("replays the last event to a subscriber that joins later", func() {
		upstream := newWatchStub()
		sharer := newWorkflowWatchSharer(fixedUpstream(upstream, nil))
		DeferCleanup(sharer.Close)

		first, err := sharer.Subscribe(context.Background(), "pat")
		Expect(err).NotTo(HaveOccurred())

		event := runsEvent("run-1")
		upstream.C <- event
		Eventually(first.ResultChan()).Should(Receive(Equal(event)))

		// The run loop records the snapshot before it can take the next request,
		// so this subscriber finds a status waiting rather than an empty stream
		// until the upstream next changes.
		second, err := sharer.Subscribe(context.Background(), "pat")
		Expect(err).NotTo(HaveOccurred())
		Eventually(second.ResultChan()).Should(Receive(Equal(event)))
	})

	It("stops the upstream only once the last subscriber has left", func() {
		upstream := newWatchStub()
		sharer := newWorkflowWatchSharer(fixedUpstream(upstream, nil))
		DeferCleanup(sharer.Close)

		first, err := sharer.Subscribe(context.Background(), "pat")
		Expect(err).NotTo(HaveOccurred())
		second, err := sharer.Subscribe(context.Background(), "pat")
		Expect(err).NotTo(HaveOccurred())

		first.Stop()
		// A closed subscriber channel is the run loop's acknowledgement of the
		// unsubscribe, so by here the departure has been accounted for.
		Eventually(first.ResultChan()).Should(BeClosed())
		Expect(upstream.C).NotTo(BeClosed())

		event := runsEvent("run-1")
		upstream.C <- event
		Eventually(second.ResultChan()).Should(Receive(Equal(event)))

		second.Stop()
		Eventually(upstream.C).Should(BeClosed())
	})

	It("closes every subscriber when the upstream ends, and starts over for the next one", func() {
		upstreams := []*scm.WorkflowWatchStub{newWatchStub(), newWatchStub()}
		var next int
		sharer := newWorkflowWatchSharer(func(string) scm.Client {
			w := upstreams[next]
			next++
			return &scm.ClientStub{OnWatchWorkflowRuns: func(context.Context, string) (scm.WorkflowWatch, error) {
				return w, nil
			}}
		})
		DeferCleanup(sharer.Close)

		first, err := sharer.Subscribe(context.Background(), "pat")
		Expect(err).NotTo(HaveOccurred())
		second, err := sharer.Subscribe(context.Background(), "pat")
		Expect(err).NotTo(HaveOccurred())

		// A revoked token ends the upstream on its own, with nobody calling Stop.
		upstreams[0].Stop()
		Eventually(first.ResultChan()).Should(BeClosed())
		Eventually(second.ResultChan()).Should(BeClosed())

		third, err := sharer.Subscribe(context.Background(), "pat")
		Expect(err).NotTo(HaveOccurred())
		Expect(next).To(Equal(2))

		// The handler stops its watch on every exit path, so this lands after the
		// upstream already took the subscriber down. It must not close a channel
		// twice, nor disturb the entry that replaced the one it belonged to.
		first.Stop()
		second.Stop()

		event := runsEvent("run-1")
		upstreams[1].C <- event
		Eventually(third.ResultChan()).Should(Receive(Equal(event)))
	})

	It("drops events for a subscriber that stops reading rather than stalling the rest", func() {
		upstream := newWatchStub()
		sharer := newWorkflowWatchSharer(fixedUpstream(upstream, nil))
		DeferCleanup(sharer.Close)

		slow, err := sharer.Subscribe(context.Background(), "pat")
		Expect(err).NotTo(HaveOccurred())
		fast, err := sharer.Subscribe(context.Background(), "pat")
		Expect(err).NotTo(HaveOccurred())

		for i := range subscriberBuffer + 8 {
			upstream.C <- runsEvent(fmt.Sprintf("run-%d", i))
			// Draining one subscriber on every round shows that the backlog
			// building up on the other never holds up the fan-out.
			Eventually(fast.ResultChan()).Should(Receive())
		}

		// The buffer keeps the oldest events and the overflow is gone for good,
		// since the upstream only emits on change and will not resend them.
		Expect(slow.ResultChan()).To(HaveLen(subscriberBuffer))
		Expect(slow.ResultChan()).To(Receive(Equal(runsEvent("run-0"))))

		// The run loop is still serving everyone else.
		_, err = sharer.Subscribe(context.Background(), "pat")
		Expect(err).NotTo(HaveOccurred())
	})

	It("stops an upstream watch whose setup finishes after the last subscriber left", func() {
		// Setup reports that it started and then parks, so the sharer is guaranteed
		// to process the unsubscribe before the upstream shows up.
		started := make(chan struct{})
		release := make(chan struct{})
		upstream := newWatchStub()

		sharer := newWorkflowWatchSharer(func(string) scm.Client {
			return &scm.ClientStub{OnWatchWorkflowRuns: func(context.Context, string) (scm.WorkflowWatch, error) {
				close(started)
				<-release
				return upstream, nil
			}}
		})
		DeferCleanup(sharer.Close)

		ctx, cancel := context.WithCancel(context.Background())
		subscribed := make(chan error, 1)
		go func() {
			_, err := sharer.Subscribe(ctx, "pat")
			subscribed <- err
		}()

		<-started
		cancel()

		// Subscribe only returns once the sharer has taken its unsubscribe, so the
		// watcher entry is gone by the time setup is allowed to complete.
		Expect(<-subscribed).To(MatchError(context.Canceled))
		close(release)

		// Nothing holds the upstream any more. Cancelling the setup context does not
		// stop a watch that was already created, so the sharer has to stop it.
		Eventually(upstream.C).Should(BeClosed())
	})

	It("does not hand an abandoned setup result to a later subscriber for the same token", func() {
		// Each call parks until the spec hands it the watch to return, which lets
		// two generations of setup for one token be in flight at the same time.
		calls := make(chan chan scm.WorkflowWatch, 2)
		sharer := newWorkflowWatchSharer(func(string) scm.Client {
			return &scm.ClientStub{OnWatchWorkflowRuns: func(context.Context, string) (scm.WorkflowWatch, error) {
				reply := make(chan scm.WorkflowWatch)
				calls <- reply
				return <-reply, nil
			}}
		})
		DeferCleanup(sharer.Close)

		ctx, cancel := context.WithCancel(context.Background())
		first := make(chan error, 1)
		go func() {
			_, err := sharer.Subscribe(ctx, "pat")
			first <- err
		}()
		abandoned := <-calls
		cancel()
		Expect(<-first).To(MatchError(context.Canceled))

		// Same token, so the same watcher key, but a brand new entry and setup.
		second := make(chan subscribeResult, 1)
		go func() {
			w, err := sharer.Subscribe(context.Background(), "pat")
			second <- subscribeResult{watch: w, err: err}
		}()
		current := <-calls

		staleUpstream := newWatchStub()
		abandoned <- staleUpstream
		Eventually(staleUpstream.C).Should(BeClosed())

		liveUpstream := newWatchStub()
		current <- liveUpstream

		var r subscribeResult
		Eventually(second).Should(Receive(&r))
		Expect(r.err).NotTo(HaveOccurred())
		Expect(liveUpstream.C).NotTo(BeClosed())
	})

	It("drops the subscriber when setup fails, so a later upstream still gets stopped", func() {
		setupErr := errors.New("list repos: connection refused")
		upstream := newWatchStub()
		fail := true

		sharer := newWorkflowWatchSharer(func(string) scm.Client {
			return &scm.ClientStub{OnWatchWorkflowRuns: func(context.Context, string) (scm.WorkflowWatch, error) {
				if fail {
					return nil, setupErr
				}
				return upstream, nil
			}}
		})
		DeferCleanup(sharer.Close)

		_, err := sharer.Subscribe(context.Background(), "pat")
		Expect(err).To(MatchError(setupErr))

		// A failed subscriber left behind would keep the entry's downstream list
		// non-empty, so this upstream would never be torn down.
		fail = false
		w, err := sharer.Subscribe(context.Background(), "pat")
		Expect(err).NotTo(HaveOccurred())
		w.Stop()

		Eventually(upstream.C).Should(BeClosed())
	})

	It("stops upstreams and closes subscriber channels on Close", func() {
		upstream := newWatchStub()
		sharer := newWorkflowWatchSharer(func(string) scm.Client {
			return &scm.ClientStub{OnWatchWorkflowRuns: func(context.Context, string) (scm.WorkflowWatch, error) {
				return upstream, nil
			}}
		})

		w, err := sharer.Subscribe(context.Background(), "pat")
		Expect(err).NotTo(HaveOccurred())

		Expect(sharer.Close()).To(Succeed())

		Eventually(upstream.C).Should(BeClosed())
		Eventually(w.ResultChan()).Should(BeClosed())
		Expect(sharer.Close()).To(Succeed())
	})

	It("never blocks on a run loop that has shut down", func() {
		upstream := newWatchStub()
		sharer := newWorkflowWatchSharer(func(string) scm.Client {
			return &scm.ClientStub{OnWatchWorkflowRuns: func(context.Context, string) (scm.WorkflowWatch, error) {
				return upstream, nil
			}}
		})

		w, err := sharer.Subscribe(context.Background(), "pat")
		Expect(err).NotTo(HaveOccurred())
		Expect(sharer.Close()).To(Succeed())

		// A closed subscriber channel proves shutdown ran, so the run loop is gone
		// and nothing will ever receive on the request channels again.
		Eventually(w.ResultChan()).Should(BeClosed())

		subscribed := make(chan error, 1)
		go func() {
			_, err := sharer.Subscribe(context.Background(), "pat")
			subscribed <- err
		}()
		Eventually(subscribed).Should(Receive(MatchError(errSharerClosed)))

		stopped := make(chan struct{})
		go func() {
			w.Stop()
			close(stopped)
		}()
		Eventually(stopped).Should(BeClosed())
	})

	It("reports a panicking SCM factory as an error instead of crashing", func() {
		sharer := newWorkflowWatchSharer(func(string) scm.Client {
			panic("unregistered SCM platform")
		})
		DeferCleanup(sharer.Close)

		_, err := sharer.Subscribe(context.Background(), "pat")
		Expect(err).To(MatchError(ContainSubstring("unregistered SCM platform")))
	})
})
