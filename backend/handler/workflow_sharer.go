package handler

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"runtime/debug"
	"slices"
	"sync"

	"github.com/openshift/faas-console-plugin/backend/functions"
	"github.com/openshift/faas-console-plugin/backend/scm"
)

type scmToken string
type watcherKey [32]byte

// errSharerClosed is returned by Subscribe once the sharer has been closed, so
// that callers fail instead of blocking on a run loop that is never coming back.
var errSharerClosed = errors.New("workflow watch sharer is closed")

// subscriberBuffer is how far a subscriber may fall behind before the sharer
// starts dropping its events. Events are whole snapshots rather than
// increments, so a drop costs freshness, not correctness, right up until the
// upstream goes quiet: it only emits on change, so a subscriber that lost the
// last one keeps a stale picture until something else moves.
const subscriberBuffer = 32

func newWorkflowWatchSharer(createSCMClient scm.ClientFactory) *workflowWatchSharer {
	sharer := &workflowWatchSharer{
		createSCMClient: createSCMClient,
		watchers:        make(map[watcherKey]*watcherEntry),
		subscribeCh:     make(chan subscribeRequest),
		unsubscribeCh:   make(chan unsubscribeRequest),
		upstreamSetupCh: make(chan upstreamSetupResult),
		done:            make(chan struct{}),
	}
	go sharer.run()
	return sharer
}

func (s *workflowWatchSharer) Subscribe(ctx context.Context, token scmToken) (scm.WorkflowWatch, error) {
	w := &subWatch{
		key:           watcherKey(sha256.Sum256([]byte(token))),
		unsubscribeCh: s.unsubscribeCh,
		done:          s.done,
		ch:            make(chan scm.WorkflowRunsEvent, subscriberBuffer),
	}
	errChan := make(chan error, 1)
	select {
	case s.subscribeCh <- subscribeRequest{w: w, t: token, err: errChan}:
	case <-ctx.Done():
		return nil, fmt.Errorf("context done: %w", ctx.Err())
	case <-s.done:
		return nil, errSharerClosed
	}
	select {
	case err := <-errChan:
		if err != nil {
			// The subWatch is already on the entry's downstream list, so it has to
			// come off again or it pins the entry for the life of the process.
			w.Stop()
			return nil, err
		}
	case <-ctx.Done():
		w.Stop()
		return nil, fmt.Errorf("context done: %w", ctx.Err())
	case <-s.done:
		return nil, errSharerClosed
	}
	slog.Info("Subscribe OK")
	return w, nil
}

func (s *workflowWatchSharer) Close() error {
	s.closeOnce.Do(func() { close(s.done) })
	return nil
}

type workflowWatchSharer struct {
	createSCMClient scm.ClientFactory
	watchers        map[watcherKey]*watcherEntry
	subscribeCh     chan subscribeRequest
	unsubscribeCh   chan unsubscribeRequest
	upstreamSetupCh chan upstreamSetupResult
	done            chan struct{}
	closeOnce       sync.Once
}

type watcherEntry struct {
	key        watcherKey
	upstream   scm.WorkflowWatch
	downstream []*subWatch
	lastEvent  *scm.WorkflowRunsEvent

	initializing bool
	pending      []chan<- error
	cancel       func()
}

const (
	subscribeIdx = iota
	unsubscribeIdx
	upstreamSetupResultIdx
	doneIdx
	dynOffset
)

func (s *workflowWatchSharer) run() {
	slog.Info("run")
	for {
		cases, indices := s.buildSelectCases()
		idx, val, ok := reflect.Select(cases)
		// A receive from done yields struct{}{}, which matches no case below, so
		// shutdown has to be recognised by index rather than by type.
		if idx == doneIdx {
			s.shutdown()
			return
		}
		switch v := val.Interface().(type) {
		case subscribeRequest:
			s.subscribe(v)
		case unsubscribeRequest:
			s.unsubscribe(v)
		case upstreamSetupResult:
			s.completeUpstreamSetup(v)
		case scm.WorkflowRunsEvent:
			slog.Info("case scm.WorkflowRunsEvent")
			e := indices[idx-dynOffset]
			if !ok {
				slog.Info("upstream has been closed")
				for _, w := range e.downstream {
					close(w.ch)
				}
				e.downstream = nil
				e.upstream = nil
				delete(s.watchers, e.key)
				continue
			}
			e.lastEvent = &v

			for _, w := range e.downstream {
				w.send(v)
			}

		default:
			slog.Warn("unknown type")
			return
		}
	}
}

// shutdown stops every upstream watch and closes every subscriber channel, so
// connected handlers see EOF rather than hanging. Setup goroutines still in
// flight stop their own watch once they notice done is closed.
func (s *workflowWatchSharer) shutdown() {
	slog.Info("shutting down sharer")
	for key, e := range s.watchers {
		if e.upstream != nil {
			e.upstream.Stop()
		}
		if e.cancel != nil {
			e.cancel()
		}
		for _, w := range e.downstream {
			close(w.ch)
		}
		delete(s.watchers, key)
	}
}

func (s *workflowWatchSharer) buildSelectCases() (cases []reflect.SelectCase, indices []*watcherEntry) {
	cases = make([]reflect.SelectCase, dynOffset, len(s.watchers)+dynOffset)
	indices = make([]*watcherEntry, len(s.watchers))
	cases[subscribeIdx] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(s.subscribeCh),
	}
	cases[unsubscribeIdx] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(s.unsubscribeCh),
	}
	cases[upstreamSetupResultIdx] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(s.upstreamSetupCh),
	}
	cases[doneIdx] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(s.done),
	}
	var i int
	for _, e := range s.watchers {
		if e.upstream != nil {
			c := reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(e.upstream.ResultChan()),
			}
			cases = append(cases, c)
			indices[i] = e
			i++
		}
	}
	return
}

type subscribeRequest struct {
	w   *subWatch
	t   scmToken
	err chan<- error
}

func (s *workflowWatchSharer) subscribe(req subscribeRequest) {
	slog.Info("subscribe")
	e := s.watchers[req.w.key]
	if e == nil {
		e = &watcherEntry{
			key: req.w.key,
		}
		s.watchers[req.w.key] = e
	}
	if e.upstream == nil {
		// Start asynchronous upstream watch initialization.
		// When setup is done upstreamSetupResult is emitted and main loop handles it.
		s.startUpstreamSetup(e, req.t, req.err)
	} else {
		// Upstream is already up and running.
		close(req.err)
	}
	e.downstream = append(e.downstream, req.w)
	if e.lastEvent != nil {
		req.w.send(*e.lastEvent)
	}
	slog.Info("downstream count", "n", len(e.downstream))
}

// upstreamSetupResult identifies its entry by pointer, not by key. A key alone
// would let a result from an abandoned setup be applied to a later entry for
// the same token.
type upstreamSetupResult struct {
	entry    *watcherEntry
	upstream scm.WorkflowWatch
	err      error
}

func (s *workflowWatchSharer) startUpstreamSetup(e *watcherEntry, t scmToken, errChan chan<- error) {
	e.pending = append(e.pending, errChan)
	if e.initializing {
		return
	}
	e.initializing = true
	var ctx context.Context
	ctx, e.cancel = context.WithCancel(context.Background())
	go func() {
		upstream, err := s.createUpstream(ctx, t)
		select {
		case s.upstreamSetupCh <- upstreamSetupResult{entry: e, upstream: upstream, err: err}:
		case <-s.done:
			// The sharer shut down while setup was running, so there is nobody
			// left to take ownership of the watch.
			if upstream != nil {
				upstream.Stop()
			}
		}
	}()
}

// createUpstream turns a panic from the SCM factory or the watch call into an
// error. It runs on its own goroutine, out of reach of the per-request recovery
// net/http installs, where a panic would otherwise take the process down.
func (s *workflowWatchSharer) createUpstream(ctx context.Context, t scmToken) (upstream scm.WorkflowWatch, err error) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		slog.Error("panic while creating upstream workflow watch", "panic", r, "stack", string(debug.Stack()))
		upstream, err = nil, fmt.Errorf("panic while creating upstream workflow watch: %v", r)
	}()
	return s.createSCMClient(string(t)).WatchWorkflowRuns(ctx, functions.WorkflowFilename)
}

func (s *workflowWatchSharer) completeUpstreamSetup(v upstreamSetupResult) {
	e := v.entry
	if s.watchers[e.key] != e {
		// The entry was dropped, or already replaced by a later one for the same
		// token, while setup was in flight. Setup may still have succeeded, and
		// cancelling its context does not stop a watch that was already created,
		// so nothing but this would ever stop it.
		if v.upstream != nil {
			v.upstream.Stop()
		}
		return
	}
	e.upstream = v.upstream
	e.initializing = false
	e.cancel = nil
	for _, p := range e.pending {
		p <- v.err
	}
	e.pending = nil
}

type unsubscribeRequest struct {
	w *subWatch
}

func (s *workflowWatchSharer) unsubscribe(req unsubscribeRequest) {
	slog.Info("unsubscribe")
	e := s.watchers[req.w.key]
	if e == nil {
		slog.Info("e is nil")
		return
	}
	idx := slices.Index(e.downstream, req.w)
	if idx == -1 {
		return
	}
	e.downstream = slices.Delete(e.downstream, idx, idx+1)
	close(req.w.ch)
	if len(e.downstream) == 0 {
		slog.Info("shutting down upstream")
		if e.upstream != nil {
			e.upstream.Stop()
			e.upstream = nil
		}
		if e.cancel != nil {
			e.cancel()
			e.cancel = nil
		}
		delete(s.watchers, req.w.key)
	}
}

type subWatch struct {
	key           watcherKey
	unsubscribeCh chan<- unsubscribeRequest
	done          <-chan struct{}
	o             sync.Once
	ch            chan scm.WorkflowRunsEvent
}

func (s *subWatch) send(e scm.WorkflowRunsEvent) {
	slog.Info("send")
	select {
	case s.ch <- e:
	default:
		slog.Warn("failed to deliver event")
	}
}

func (s *subWatch) ResultChan() <-chan scm.WorkflowRunsEvent {
	return s.ch
}

func (s *subWatch) Stop() {
	s.o.Do(func() {
		select {
		case s.unsubscribeCh <- unsubscribeRequest{w: s}:
		case <-s.done:
			// The run loop is gone and has already closed s.ch.
		}
	})
}
