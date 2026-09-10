package github

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"time"

	ghlib "github.com/google/go-github/v90/github"
	"golang.org/x/sync/errgroup"

	"github.com/openshift/faas-console-plugin/backend/scm"
)

// Tunable so tests can drive the watch loop quickly.
var (
	watchPollInterval       = 3 * time.Second
	watchRediscoverInterval = 30 * time.Second
)

// WatchWorkflowRuns implements scm.Client. It discovers the caller's function
// repos once (synchronously, so auth failures are returned rather than lost in
// the goroutine), then streams a snapshot of each repo's latest run on every
// change. It owns the poll loop, periodic repo re-discovery, and carrying a
// repo's last-known run forward across transient per-repo errors.
func (c *ghClient) WatchWorkflowRuns(ctx context.Context, workflowFile string) (<-chan []scm.RepoRun, error) {
	repos, err := c.ListRepos(ctx)
	if err != nil {
		return nil, err
	}

	out := make(chan []scm.RepoRun)
	go func() {
		defer close(out)

		// Last-known run per repo key, carried forward when a per-repo poll fails
		// transiently: a flaky GitHub error would otherwise reset the run to nil
		// and flicker the build status.
		prevRuns := make(map[string]*scm.WorkflowRun)
		var prevSnapshot []scm.RepoRun

		// emit sends a snapshot only when it differs from the last one sent, so
		// the channel carries changes rather than every poll. Returns false when
		// the context is cancelled while sending.
		emit := func(snapshot []scm.RepoRun) bool {
			if reflect.DeepEqual(snapshot, prevSnapshot) {
				return true
			}
			select {
			case out <- snapshot:
				prevSnapshot = snapshot
				return true
			case <-ctx.Done():
				return false
			}
		}

		// pollAndEmit runs one poll, refreshes the carry-forward index from the
		// resulting snapshot (pruning repos that dropped out of discovery so the
		// map cannot grow unbounded on a long-lived stream), then emits.
		pollAndEmit := func() bool {
			snapshot := c.pollRuns(ctx, repos, workflowFile, prevRuns)
			next := make(map[string]*scm.WorkflowRun, len(snapshot))
			for _, rr := range snapshot {
				next[rr.Repo.FullName()] = rr.Run
			}
			prevRuns = next
			return emit(snapshot)
		}

		if !pollAndEmit() {
			return
		}

		poll := time.NewTicker(watchPollInterval)
		defer poll.Stop()
		rediscover := time.NewTicker(watchRediscoverInterval)
		defer rediscover.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-rediscover.C:
				latest, err := c.ListRepos(ctx)
				if err != nil {
					// A revoked/expired token makes this global call fail
					// unambiguously. End the stream so the caller can re-auth;
					// per-repo poll errors are merely carried forward and would
					// otherwise leave the client on stale status indefinitely.
					if errors.Is(err, scm.ErrUnauthorized) {
						slog.Info("watch workflow runs: token no longer authorized, ending stream")
						return
					}
					slog.Warn("watch workflow runs: rediscover failed", "err", err)
					continue
				}
				repos = latest
			case <-poll.C:
				if !pollAndEmit() {
					return
				}
			}
		}
	}()
	return out, nil
}

// pollRuns fetches the latest run for each repo concurrently and returns a
// snapshot sorted by repo key. A per-repo error carries that repo's last-known
// run forward from prevRuns (nil if none is known); the cause is logged, never
// surfaced on the channel. prevRuns is only read here (the caller updates it),
// so the concurrent reads are safe.
func (c *ghClient) pollRuns(ctx context.Context, repos []scm.Repo, workflowFile string, prevRuns map[string]*scm.WorkflowRun) []scm.RepoRun {
	snapshot := make([]scm.RepoRun, len(repos))
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(10)
	for i, repo := range repos {
		g.Go(func() error {
			run, err := c.latestWorkflowRun(ctx, repo.Owner, repo.Name, repo.DefaultBranch, workflowFile)
			if err != nil {
				slog.Warn("watch workflow runs: get run failed", "repo", repo.FullName(), "err", err)
				run = prevRuns[repo.FullName()]
			}
			snapshot[i] = scm.RepoRun{Repo: repo, Run: run}
			return nil
		})
	}
	_ = g.Wait()
	sort.Slice(snapshot, func(i, j int) bool { return snapshot[i].Repo.FullName() < snapshot[j].Repo.FullName() })
	return snapshot
}

func (c *ghClient) latestWorkflowRun(ctx context.Context, owner, repo, branch, workflowFile string) (*scm.WorkflowRun, error) {
	opts := &ghlib.ListWorkflowRunsOptions{
		Branch:      branch,
		ListOptions: ghlib.ListOptions{PerPage: 1},
	}
	runs, _, err := c.client.Actions.ListWorkflowRunsByFileName(ctx, owner, repo, workflowFile, opts)
	if err != nil {
		if isNotFound(err) {
			// The workflow file does not exist in this repo (e.g. a non-func repo,
			// or the func workflow has not been pushed yet). Treat it like a repo
			// with no runs rather than surfacing an error.
			return nil, nil
		}
		return nil, fmt.Errorf("list workflow runs for %s/%s (%s): %w", owner, repo, workflowFile, mapErr(err))
	}
	if len(runs.WorkflowRuns) == 0 {
		return nil, nil
	}

	// GitHub returns runs in created_at descending order by default, so with
	// PerPage 1 the single element WorkflowRuns[0] is the newest run.
	run := runs.WorkflowRuns[0]
	result := &scm.WorkflowRun{
		ID:         run.GetID(),
		Status:     run.GetStatus(),
		Conclusion: run.GetConclusion(),
		HeadSHA:    run.GetHeadSHA(),
		HTMLURL:    run.GetHTMLURL(),
	}
	if result.Conclusion == "failure" {
		reason, err := c.failureReason(ctx, owner, repo, result.ID)
		if err != nil {
			return nil, err
		}
		result.FailureReason = reason
	}
	return result, nil
}

// failureReason returns a "<job> / <step>" summary of the first failed step, or
// the failing job name, or "" when no failing job is found. It returns an error
// only when the jobs lookup itself fails; the caller propagates that so pollRuns
// carries the previous run (and its reason) forward, rather than flickering the
// reason to empty on a transient error.
func (c *ghClient) failureReason(ctx context.Context, owner, repo string, runID int64) (string, error) {
	jobs, _, err := c.client.Actions.ListWorkflowJobs(ctx, owner, repo, runID, nil)
	if err != nil {
		return "", fmt.Errorf("list workflow jobs for %s/%s (run %d): %w", owner, repo, runID, mapErr(err))
	}
	for _, job := range jobs.Jobs {
		if job.GetConclusion() != "failure" {
			continue
		}
		for _, step := range job.Steps {
			if step.GetConclusion() == "failure" {
				return job.GetName() + " / " + step.GetName(), nil
			}
		}
		return job.GetName(), nil
	}
	return "", nil
}
