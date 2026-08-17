package httpapi

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TheZupZup/Rivo/services/api/internal/auth"
)

type fakeTokenStore struct {
	knownHash []byte
	identity  auth.Identity
	err       error
}

func (store fakeTokenStore) IdentityForTokenHash(_ context.Context, tokenHash []byte) (auth.Identity, error) {
	if store.err != nil {
		return auth.Identity{}, store.err
	}
	if !bytes.Equal(tokenHash, store.knownHash) {
		return auth.Identity{}, auth.ErrUnknownToken
	}

	return store.identity, nil
}

func testAuthenticator() auth.Authenticator {
	return auth.NewAuthenticator(fakeTokenStore{
		knownHash: auth.HashToken("dev-token"),
		identity:  auth.Identity{UserID: "user-1", Handle: "reporter", TokenID: "token-1"},
	})
}

func TestRequireIdentityPassesTheActorToTheHandler(t *testing.T) {
	var seen auth.Identity
	handler := requireIdentity(testAuthenticator(), http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		seen, _ = auth.IdentityFrom(request.Context())
	}))

	request := httptest.NewRequest(http.MethodPost, "/api/reports", nil)
	request.Header.Set("Authorization", "Bearer dev-token")
	handler.ServeHTTP(httptest.NewRecorder(), request)

	if seen.UserID != "user-1" {
		t.Fatalf("expected the authenticated user on the context, got %+v", seen)
	}
}

func TestRequireIdentityRejectsMissingAndUnknownTokensIdentically(t *testing.T) {
	handler := requireIdentity(testAuthenticator(), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("handler must not run for an unauthenticated request")
	}))

	for name, header := range map[string]string{
		"no header":     "",
		"unknown token": "Bearer nope",
		"wrong scheme":  "Basic dev-token",
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/reports", nil)
			if header != "" {
				request.Header.Set("Authorization", header)
			}
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", response.Code)
			}
			// Identical bodies: a caller must not be able to tell "no such token"
			// from "revoked token" and enumerate valid ones.
			if body := response.Body.String(); body != "{\"error\":\"a valid API token is required\"}\n" {
				t.Fatalf("unexpected body %q", body)
			}
			if response.Header().Get("WWW-Authenticate") == "" {
				t.Fatal("expected a WWW-Authenticate challenge")
			}
		})
	}
}

func TestRequireIdentityReportsAnOutageAsUnavailable(t *testing.T) {
	authenticator := auth.NewAuthenticator(fakeTokenStore{err: errors.New("database unavailable")})
	handler := requireIdentity(authenticator, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler must not run when authentication cannot be decided")
	}))

	request := httptest.NewRequest(http.MethodPost, "/api/reports", nil)
	request.Header.Set("Authorization", "Bearer dev-token")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	// A database outage is not a credential problem; answering 401 would send
	// clients off to re-issue tokens that were never invalid.
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", response.Code)
	}
}
