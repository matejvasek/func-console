package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/openshift/faas-console-plugin/backend/config"
	"github.com/openshift/faas-console-plugin/backend/functions"
	"github.com/openshift/faas-console-plugin/backend/scm"
)

// Tunable so tests can drive the SSE loop quickly. Polling and repo
// re-discovery are owned by scm.Client.WatchWorkflowRuns; the handler only
// keeps the client-facing heartbeat.
var buildHeartbeatInterval = 15 * time.Second

type buildStatusItem struct {
	BuildStatus   string `json:"buildStatus"` // Building | Succeeded | Failed | None
	Conclusion    string `json:"conclusion,omitempty"`
	RunURL        string `json:"runURL,omitempty"`
	FailureReason string `json:"failureReason,omitempty"`
	HeadSHA       string `json:"headSHA,omitempty"`
}

// buildSnapshot keys each function's status by its "owner/name" full name, the
// same identifier the frontend correlates on. encoding/json emits the map keys
// sorted, so the SSE frame stays byte-stable across unchanged polls.
type buildSnapshot struct {
	Functions map[string]buildStatusItem `json:"functions"`
}

func (h *Handlers) HandleBuildWatch(w http.ResponseWriter, r *http.Request) {
	pat, ok := extractSCMToken(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "X-SCM-Token header is required")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	client := config.SCMRegistry.Client(scm.DefaultPlatform, pat)
	ctx := r.Context()

	// WatchWorkflowRuns discovers repos synchronously, so auth failures surface
	// here (as a normal HTTP status) before we switch the response to SSE.
	runs, err := client.WatchWorkflowRuns(ctx, functions.WorkflowFilename)
	if err != nil {
		if errors.Is(err, scm.ErrUnauthorized) {
			writeError(w, http.StatusUnauthorized, "invalid SCM token")
			return
		}
		slog.Error("build watch: watch workflow runs failed", "err", err)
		writeError(w, http.StatusBadGateway, "failed to list repositories")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	// Flush the response head immediately so the client's request completes and
	// it can start reading, rather than blocking until the first snapshot frame.
	flusher.Flush()

	heartbeat := time.NewTicker(buildHeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if _, err := io.WriteString(w, ":\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case snapshot, ok := <-runs:
			if !ok {
				// The watch ended (context cancelled or the token was revoked
				// mid-stream). End the SSE stream so the client reconnects and
				// its initial request hits a 401, triggering its re-auth path.
				return
			}
			data, err := json.Marshal(toSnapshot(snapshot))
			if err != nil {
				slog.Warn("build watch: marshal snapshot failed", "err", err)
				continue
			}
			if err := writeSnapshotEvent(w, data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// toSnapshot maps a scm snapshot of repo runs into the wire DTO the frontend
// consumes, keyed by "owner/name" and translating each run into the build
// vocabulary. The map is always non-nil so an empty snapshot encodes as {}.
func toSnapshot(runs []scm.RepoRun) buildSnapshot {
	items := make(map[string]buildStatusItem, len(runs))
	for _, rr := range runs {
		items[rr.Repo.FullName()] = toBuildStatusItem(rr.Run)
	}
	return buildSnapshot{Functions: items}
}

func toBuildStatusItem(run *scm.WorkflowRun) buildStatusItem {
	item := buildStatusItem{BuildStatus: deriveBuildStatus(run)}
	if run != nil {
		item.Conclusion = run.Conclusion
		item.RunURL = run.HTMLURL
		item.FailureReason = run.FailureReason
		item.HeadSHA = run.HeadSHA
	}
	return item
}

func deriveBuildStatus(run *scm.WorkflowRun) string {
	if run == nil {
		return "None"
	}
	switch run.Status {
	// Every pre-completion status (including the gated "waiting"/"requested"/
	// "pending" states) means a run exists but has not finished, so the build is
	// still in flight.
	case "queued", "in_progress", "waiting", "requested", "pending":
		return "Building"
	case "completed":
		switch run.Conclusion {
		case "success":
			return "Succeeded"
		case "failure", "cancelled", "timed_out":
			return "Failed"
		default:
			// Non-failure outcomes like "skipped", "neutral", "stale", or
			// "action_required" are not build failures; report no build signal
			// so the frontend falls back to the cluster-derived status rather
			// than showing a red "Build failed" badge.
			return "None"
		}
	default:
		return "None"
	}
}

// writeSnapshotEvent writes the already-marshaled snapshot bytes as an SSE frame.
func writeSnapshotEvent(w io.Writer, data []byte) error {
	if _, err := fmt.Fprintf(w, "event: build-status\ndata: %s\n\n", data); err != nil {
		return fmt.Errorf("write build-status event: %w", err)
	}
	return nil
}
