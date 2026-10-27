package handler

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"

	"github.com/openshift/faas-console-plugin/backend/cluster"
	"github.com/openshift/faas-console-plugin/backend/config"
	"github.com/openshift/faas-console-plugin/backend/functions"
	"github.com/openshift/faas-console-plugin/backend/scm"
	"github.com/openshift/faas-console-plugin/backend/scm/credentials"
)

const (
	// testOCPToken stands in for the console user token the proxy forwards.
	testOCPToken = "ocp-token"
	// testNamespace is where the fake cluster keeps its credential Secrets.
	testNamespace = "console-functions-plugin"
)

func testCredentialProvider() credentials.MapProvider {
	return credentials.MapProvider{
		testOCPToken: credentials.Credential{
			Owner: "tester",
			Token: "test-pat",
			Type:  credentials.CredentialTypePAT,
		},
	}
}

// authenticate marks req as coming through the console proxy, which forwards
// the user token. That identity is now the only thing a request carries: the
// credential is found by it rather than by a handle the browser holds.
func authenticate(req *http.Request) *http.Request {
	req.Header.Set("Authorization", "Bearer "+testOCPToken)
	return req
}

func withSCMStub(stub scm.Client) {
	orig := config.SCMRegistry
	config.SCMRegistry = scm.Registry{
		scm.GitHub: func(token string) scm.Client { return stub },
	}
	DeferCleanup(func() { config.SCMRegistry = orig })
}

func withClusterStub(stub cluster.Client) {
	orig := newClusterClient
	newClusterClient = func(host, token string, caCert []byte) (cluster.Client, error) {
		return stub, nil
	}
	DeferCleanup(func() { newClusterClient = orig })
}

func withFunctionsClient(stub functions.Client) {
	orig := newFunctionsClient
	newFunctionsClient = func(host, token string, caCert []byte) (functions.Client, error) {
		return stub, nil
	}
	DeferCleanup(func() { newFunctionsClient = orig })
}

func withFunctionsClientError(err error) {
	orig := newFunctionsClient
	newFunctionsClient = func(host, token string, caCert []byte) (functions.Client, error) {
		return nil, err
	}
	DeferCleanup(func() { newFunctionsClient = orig })
}

func withClusterClientError(err error) {
	orig := newClusterClient
	newClusterClient = func(host, token string, caCert []byte) (cluster.Client, error) {
		return nil, err
	}
	DeferCleanup(func() { newClusterClient = orig })
}
