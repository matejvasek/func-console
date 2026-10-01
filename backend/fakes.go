package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/openshift/faas-console-plugin/backend/scm"
)

const namespace = "admin-playground"
const clusterAPIURL = "https://api.ocp.alnitak.xvasek.cz:6443"
const expectedPat = "iddqd"

// This only fakes SCM. I am not sure how to fake k8s/knative.
//
// For now, I call:
//
//	kn service create fn-testing-a \
//	  --image ghcr.io/knative/helloworld-go:latest \
//	  --label=function.knative.dev/name=fn-testing-a
//
//	kubectl delete ksvc/fn-testing-a
//
// to manipulate function Running state.
//
// However, this will be much easier once we do all k8s calls in backend.
//
// On the other hand we already have "Console Cluster Watch" doubles so for sake of testing you could monkey-patch
// the SDK Watches with our watch that changes ksvc deployment state according to some scripted scenario.
func createFakeSCM(pat string) scm.Client {
	log.Println("scmClientFactory")
	notImplemented := errors.New("not implemented")
	return &scm.ClientStub{
		OnGetUser: func(ctx context.Context) (*scm.User, error) {
			// The rest of endpoints do not check PAT, this is good when testing native EventSource.
			if pat != expectedPat {
				return nil, scm.ErrUnauthorized
			}
			return &scm.User{
				Login: "e2e-user",
			}, nil
		},
		OnListRepos: func(ctx context.Context) ([]scm.Repo, error) {
			return []scm.Repo{
				{
					Owner:         "e2e-user",
					Name:          "fn-testing-a",
					URL:           "a.example.com",
					DefaultBranch: "master",
				},
				{
					Owner:         "e2e-user",
					Name:          "fn-testing-b",
					URL:           "b.example.com",
					DefaultBranch: "master",
				},
			}, nil
		},
		OnGetFileContent: func(ctx context.Context, owner, repo, ref, path string) (string, error) {
			return funcYaml(repo), nil
		},
		OnGetFiles: func(ctx context.Context, owner, repo, ref string) ([]scm.FileEntry, error) {
			return []scm.FileEntry{
				{
					Path:    "func.yaml",
					Mode:    "100644",
					Content: funcYaml(repo),
					Type:    "blob",
				},
			}, nil
		},
		OnGetVariable: func(ctx context.Context, owner, repo, name string) (string, error) {
			if name == "CLUSTER_API_URL" {
				return clusterAPIURL, nil
			}
			return "", errors.New("not found")
		},
		OnWatchWorkflowRuns: func(ctx context.Context, workflowFile string) (scm.WorkflowWatch, error) {
			log.Println("OnWatchWorkflowRuns")
			return createFakeWatch(), nil
		},
		OnPushFiles: func(ctx context.Context, owner, repo, branch, message string, files []scm.FileEntry) error {
			return notImplemented
		},
		OnInitRepo: func(ctx context.Context, owner, name, branch string, topics []string) error {
			return notImplemented
		},
		OnStoreSecret: func(ctx context.Context, owner, repo, name, value string) error {
			return notImplemented
		},
		OnDeleteRepo: func(ctx context.Context, owner, repo string) error {
			return notImplemented
		},
		OnStoreVariable: func(ctx context.Context, owner, repo, name, value string) error {
			return notImplemented
		},
	}
}

const funcYamlFmt = `specVersion: 0.36.0
name: %s
runtime: python
created: 2026-10-01T18:02:00.341246744+02:00
namespace: %s
`

func funcYaml(repo string) string {
	return fmt.Sprintf(funcYamlFmt, repo, namespace)
}

func createFakeWatch() scm.WorkflowWatch {
	log.Println("newMockWatch")
	watch := fakeWatch{
		c:    make(chan scm.WorkflowRunsOrErr),
		done: make(chan struct{}),
	}
	go func() {
		defer func() {
			_ = recover() // just in case
		}()
		var events = []scm.WorkflowRunsOrErr{
			{
				Runs: map[string]scm.WorkflowRun{
					"e2e-user/fn-testing-a": {},
				},
			},
			{
				Runs: map[string]scm.WorkflowRun{
					"e2e-user/fn-testing-a": {BuildStatus: scm.Building},
				},
			},
			{
				Runs: map[string]scm.WorkflowRun{
					"e2e-user/fn-testing-a": {BuildStatus: scm.Failed},
				},
			},
			{
				Err: scm.ErrUnauthorized,
			},
		}
		for {
			for _, e := range events {
				ok := watch.Emit(e)
				if !ok {
					log.Println("watch closed")
					return // watcher closed
				}
				time.Sleep(time.Second * 2)
			}
		}
	}()
	return &watch
}

type fakeWatch struct {
	done chan struct{}
	c    chan scm.WorkflowRunsOrErr
	o    sync.Once
}

func (m *fakeWatch) ResultChan() <-chan scm.WorkflowRunsOrErr {
	return m.c
}

func (m *fakeWatch) Stop() {
	m.o.Do(func() {
		close(m.done)
	})
}

func (m *fakeWatch) Emit(event scm.WorkflowRunsOrErr) bool {
	select {
	case m.c <- event:
		return true
	case <-m.done:
		return false
	}
}
