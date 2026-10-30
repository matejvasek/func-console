package handler

import (
	"context"
	"crypto/sha256"
	"log/slog"
	"reflect"
	"slices"
	"sync"

	"github.com/openshift/faas-console-plugin/backend/functions"
	"github.com/openshift/faas-console-plugin/backend/scm"
)

type scmToken string
type watcherKey [32]byte

func newWorkflowWatchSharer(createSCMClient scm.ClientFactory) *workflowWatchSharer {
	sharer := &workflowWatchSharer{
		createSCMClient: createSCMClient,
		watchers:        make(map[watcherKey]*watcherEntry),
		addCh:           make(chan addReq),
		delCh:           make(chan delReq),
		done:            make(chan struct{}),
	}
	go sharer.run()
	return sharer
}

func (s *workflowWatchSharer) GetWatch(ctx context.Context, token scmToken) (scm.WorkflowWatch, error) {
	w := &subWatch{
		key:   watcherKey(sha256.Sum256([]byte(token))),
		delCh: s.delCh,
		ch:    make(chan scm.WorkflowRunsEvent, 32),
	}
	errChan := make(chan error)
	s.addCh <- addReq{
		w:   w,
		t:   token,
		ctx: ctx,
		err: errChan,
	}
	err := <-errChan
	if err != nil {
		return nil, err
	}
	slog.Info("GetWatch OK")
	return w, nil
}

func (s *workflowWatchSharer) Close() error {
	close(s.done)
	return nil
}

type workflowWatchSharer struct {
	createSCMClient scm.ClientFactory
	watchers        map[watcherKey]*watcherEntry
	addCh           chan addReq
	delCh           chan delReq
	done            chan struct{}
}

type watcherEntry struct {
	key        watcherKey
	upstream   scm.WorkflowWatch
	downstream []*subWatch
	lastEvent  *scm.WorkflowRunsEvent
}

const (
	idxAdd = iota
	idxDel
	idxDone
	dynOffset
)

func (s *workflowWatchSharer) run() {
	slog.Info("run")
	for {
		cases, indices := s.buildSelectCases()
		idx, val, ok := reflect.Select(cases)
		switch v := val.Interface().(type) {
		case addReq:
			s.add(v)
		case delReq:
			s.del(v)
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
			var wg sync.WaitGroup
			for _, w := range e.downstream {
				wg.Go(func() {
					w.send(v)
				})
			}
			wg.Wait()
		default:
			slog.Warn("unknown type")
			return
		}
	}
}

func (s *workflowWatchSharer) buildSelectCases() (cases []reflect.SelectCase, indices []*watcherEntry) {
	cases = make([]reflect.SelectCase, len(s.watchers)+3)
	indices = make([]*watcherEntry, len(s.watchers))
	cases[idxAdd] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(s.addCh),
	}
	cases[idxDel] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(s.delCh),
	}
	cases[idxDone] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(s.done),
	}
	var i int
	for _, e := range s.watchers {
		cases[i+dynOffset] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(e.upstream.ResultChan()),
		}
		indices[i] = e
		i++
	}
	return
}

type addReq struct {
	w   *subWatch
	t   scmToken
	ctx context.Context
	err chan error
}

func (s *workflowWatchSharer) add(req addReq) {
	slog.Info("add")
	defer close(req.err)
	e := s.watchers[req.w.key]
	if e == nil {
		e = &watcherEntry{
			key: req.w.key,
		}
	}
	if e.upstream == nil {
		scmClient := s.createSCMClient(string(req.t))
		up, err := scmClient.WatchWorkflowRuns(req.ctx, functions.WorkflowFilename)
		if err != nil {
			req.err <- err
			return
		}
		e.upstream = up
	}
	e.downstream = append(e.downstream, req.w)
	if e.lastEvent != nil {
		req.w.send(*e.lastEvent)
	}
	s.watchers[req.w.key] = e
	slog.Info("downstream count", "n", len(e.downstream))
}

type delReq struct {
	w *subWatch
}

func (s *workflowWatchSharer) del(req delReq) {
	slog.Info("del")
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
		e.upstream.Stop()
		e.upstream = nil
		delete(s.watchers, req.w.key)
	}
}

type subWatch struct {
	key   watcherKey
	delCh chan delReq
	o     sync.Once
	ch    chan scm.WorkflowRunsEvent
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
		s.delCh <- delReq{w: s}
	})
}
