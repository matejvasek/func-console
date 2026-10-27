package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/faas-console-plugin/backend/identity"
	"github.com/openshift/faas-console-plugin/backend/scm"
	"github.com/openshift/faas-console-plugin/backend/scm/credentials"
)

var _ = Describe("POST /api/v1/scm/github/credential", func() {
	BeforeEach(func() {
		withSCMStub(&scm.ClientStub{
			OnGetUser: func(ctx context.Context) (*scm.User, error) {
				return &scm.User{Login: "alice-gh", AvatarURL: "https://example.com/avatar"}, nil
			},
		})
	})

	// Asserted through the store rather than on what the handler passed it: the
	// credential it just wrote must be reachable by its owner and by nobody else.
	It("binds the credential to the OpenShift user that stored it", func() {
		h := &Handlers{scmCredentials: credentials.MapProvider{"": credentials.Credential{}}}

		w := httptest.NewRecorder()
		h.HandleSaveSCMCredential(w, saveTokenRequestEx("user-a-tok", "ghp_valid_a"))
		Expect(w.Code).To(Equal(http.StatusCreated))

		w = httptest.NewRecorder()
		h.HandleSaveSCMCredential(w, saveTokenRequestEx("user-b-tok", "ghp_valid_b"))
		Expect(w.Code).To(Equal(http.StatusCreated))

		credential, err := h.scmCredentials.GetCredential(context.Background(), "user-a-tok")
		Expect(err).NotTo(HaveOccurred())
		Expect(credential.Token).To(Equal("ghp_valid_a"))

		credential, err = h.scmCredentials.GetCredential(context.Background(), "user-b-tok")
		Expect(err).NotTo(HaveOccurred())
		Expect(credential.Token).To(Equal("ghp_valid_b"))
	})

	It("returns the GitHub profile, never the token", func() {
		w := httptest.NewRecorder()

		h := Handlers{scmCredentials: testCredentialProvider()}
		h.HandleSaveSCMCredential(w, saveTokenRequest("ghp_valid"))

		var resp map[string]string
		Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
		Expect(resp).To(HaveKeyWithValue("login", "alice-gh"))
		Expect(resp).To(HaveKeyWithValue("avatarUrl", "https://example.com/avatar"))
		Expect(resp).To(HaveLen(2)) // login, avatarUrl and nothing else
	})

	It("returns 400 when no token was sent", func() {
		w := httptest.NewRecorder()

		h := Handlers{scmCredentials: testCredentialProvider()}
		h.HandleSaveSCMCredential(w, saveTokenRequest(""))

		Expect(w.Code).To(Equal(http.StatusBadRequest))
	})

	// Not 401: the frontend reads that one status as "the connection is
	// gone" and disconnects, which would throw away a working session over a
	// mistyped PAT. The SCM provider refusing a token the user just supplied
	// is the same class of problem as it refusing a stored one.
	It("returns 403 when the SCM provider rejects the token", func() {
		withSCMStub(&scm.ClientStub{
			OnGetUser: func(ctx context.Context) (*scm.User, error) { return nil, scm.ErrUnauthorized },
		})
		w := httptest.NewRecorder()

		cp := testCredentialProvider()
		h := Handlers{scmCredentials: cp}
		h.HandleSaveSCMCredential(w, saveTokenRequest("ghp_bad"))

		Expect(w.Code).To(Equal(http.StatusForbidden))
		// A token GitHub refuses is never written, so the caller keeps whatever
		// they already had rather than losing it to a typo.
		Expect(cp).To(HaveLen(1))
	})

	It("returns 401 when the OpenShift token is rejected", func() {
		h := Handlers{scmCredentials: failingCredentialProvider(fmt.Errorf("%w: token rejected by the API server", identity.ErrUnauthenticated))}
		w := httptest.NewRecorder()

		h.HandleSaveSCMCredential(w, saveTokenRequest("ghp_valid"))

		Expect(w.Code).To(Equal(http.StatusUnauthorized))
	})

	// Not 401: the API server being unreachable says nothing about the caller,
	// and the frontend answers 401 by disconnecting. A blip must not cost the
	// user a connection they still have.
	It("returns 503 when the identity cannot be looked up at all", func() {
		h := Handlers{scmCredentials: failingCredentialProvider(errors.New("dial tcp: connection refused"))}
		w := httptest.NewRecorder()

		h.HandleSaveSCMCredential(w, saveTokenRequest("ghp_valid"))

		Expect(w.Code).To(Equal(http.StatusServiceUnavailable))
	})

	It("returns 401 when the console forwarded no user token", func() {
		h := Handlers{scmCredentials: testCredentialProvider()}
		req := saveTokenRequest("ghp_valid")
		req.Header.Del("Authorization")
		w := httptest.NewRecorder()

		h.HandleSaveSCMCredential(w, req)

		Expect(w.Code).To(Equal(http.StatusUnauthorized))
	})
})

var _ = Describe("GET /api/v1/scm/github/credential", func() {
	getRequest := func() *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/scm/github/credential", nil)
		req.Header.Set("Authorization", "Bearer "+testOCPToken)
		return req
	}

	// The whole point of keying on identity alone: a browser that has never
	// seen this credential is connected by its OpenShift token and nothing else.
	It("reports the GitHub account behind a stored credential", func() {
		h := Handlers{scmCredentials: testCredentialProvider()}
		w := httptest.NewRecorder()

		h.HandleGetSCMCredential(w, getRequest())

		Expect(w.Code).To(Equal(http.StatusOK))
		var resp map[string]string
		Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
		Expect(resp).To(HaveKeyWithValue("login", "tester"))
		Expect(resp).To(HaveLen(2)) // login, avatarUrl and nothing else
	})

	// The stored token answers requests on the backend; handing it back to the
	// browser would undo the entire reason it lives in a Secret.
	It("never returns the token itself", func() {
		h := Handlers{scmCredentials: testCredentialProvider()}
		w := httptest.NewRecorder()

		h.HandleGetSCMCredential(w, getRequest())

		Expect(w.Body.String()).NotTo(ContainSubstring("test-pat"))
	})

	// A caller with nothing stored is not an authentication failure: they are
	// known, they have simply never connected. The frontend reads 404 as "show
	// the connect prompt" rather than "something went wrong".
	It("returns 404 when the user has no stored credential", func() {
		h := Handlers{scmCredentials: make(credentials.MapProvider)}
		w := httptest.NewRecorder()

		h.HandleGetSCMCredential(w, getRequest())

		Expect(w.Code).To(Equal(http.StatusNotFound))
	})

	It("returns 401 when the OpenShift token is rejected", func() {
		h := Handlers{scmCredentials: failingCredentialProvider(fmt.Errorf("%w: token rejected by the API server", identity.ErrUnauthenticated))}
		w := httptest.NewRecorder()

		h.HandleGetSCMCredential(w, getRequest())

		Expect(w.Code).To(Equal(http.StatusUnauthorized))
	})

	// Not 404: the frontend reads that as "there is nothing stored" and shows
	// the connect prompt. Being unable to read the credential is not the same
	// as there not being one.
	It("returns 503 when the identity cannot be looked up at all", func() {
		h := Handlers{scmCredentials: failingCredentialProvider(errors.New("dial tcp: connection refused"))}
		w := httptest.NewRecorder()

		h.HandleGetSCMCredential(w, getRequest())

		Expect(w.Code).To(Equal(http.StatusServiceUnavailable))
	})
})

var _ = Describe("DELETE /api/v1/scm/github/credential", func() {
	deleteRequest := func() *http.Request {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/scm/github/credential", nil)
		authenticate(req)
		return req
	}

	It("deletes the credential of the user it belongs to", func() {
		cp := testCredentialProvider()
		h := Handlers{scmCredentials: cp}
		w := httptest.NewRecorder()

		h.HandleDeleteSCMCredential(w, deleteRequest())

		Expect(w.Code).To(Equal(http.StatusNoContent))
		Expect(cp).To(BeEmpty())
	})

	// Revocation follows the identity. Reaching the backend as one user must
	// not be a way to disconnect somebody else's GitHub account.
	It("leaves another user's credential alone", func() {
		cp := credentials.MapProvider{
			testOCPToken: credentials.Credential{
				Owner: "tester",
				Token: "test-pat",
				Type:  credentials.CredentialTypePAT,
			},
			"other-user-token": credentials.Credential{
				Owner: "other-user",
				Token: "other-user-test-pat",
				Type:  credentials.CredentialTypePAT,
			},
		}
		h := Handlers{scmCredentials: cp}
		w := httptest.NewRecorder()

		h.HandleDeleteSCMCredential(w, deleteRequest())

		Expect(w.Code).To(Equal(http.StatusNoContent))
		Expect(cp).To(HaveLen(1))
	})

	It("returns 204 without touching the store when the user cannot be resolved", func() {
		h := Handlers{scmCredentials: failingCredentialProvider(errors.New("token rejected by the API server"))}
		w := httptest.NewRecorder()

		h.HandleDeleteSCMCredential(w, deleteRequest())

		Expect(w.Code).To(Equal(http.StatusNoContent))
	})
})

func credentialProviderForOwner(owner string) credentials.MapProvider {
	return credentials.MapProvider{
		testOCPToken: credentials.Credential{
			Owner: owner,
			Token: "test-pat",
			Type:  credentials.CredentialTypePAT,
		},
	}

}

func failingCredentialProvider(cause error) credentials.Provider {
	return credentials.ProviderStub{
		SaveCredentialsFn: func(ctx context.Context, ocpToken string, cred credentials.Credential) error {
			return cause
		},
		GetCredentialFn: func(ctx context.Context, ocpToken string) (credentials.Credential, error) {
			return credentials.Credential{}, cause
		},
		DeleteCredentialFn: func(ctx context.Context, ocpToken string) error {
			return cause
		},
	}
}

func saveTokenRequest(pat string) *http.Request {
	body, err := json.Marshal(map[string]string{"pat": pat})
	Expect(err).NotTo(HaveOccurred())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/scm/github/credential", bytes.NewBuffer(body))
	return authenticate(req)
}

func saveTokenRequestEx(ocpToken, scmToken string) *http.Request {
	body, err := json.Marshal(map[string]string{"pat": scmToken})
	Expect(err).NotTo(HaveOccurred())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/scm/github/credential", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+ocpToken)
	return req
}
