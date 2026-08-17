package auth

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"testing"
)

type fakeTokenStore struct {
	knownHash []byte
	identity  Identity
	err       error
}

func (store fakeTokenStore) IdentityForTokenHash(_ context.Context, tokenHash []byte) (Identity, error) {
	if store.err != nil {
		return Identity{}, store.err
	}
	if !bytes.Equal(tokenHash, store.knownHash) {
		return Identity{}, ErrUnknownToken
	}

	return store.identity, nil
}

func TestHashTokenIsStableAndDoesNotStoreTheSecret(t *testing.T) {
	hash := HashToken("rivo_dev_token")

	if len(hash) != 32 {
		t.Fatalf("expected a 32 byte sha-256 digest, got %d bytes", len(hash))
	}
	if !bytes.Equal(hash, HashToken("rivo_dev_token")) {
		t.Fatal("expected hashing to be deterministic")
	}
	if bytes.Contains(hash, []byte("rivo_dev_token")) {
		t.Fatal("expected the digest not to contain the token")
	}
	// Guards the value the dev seed inserts; changing the algorithm must break here.
	if hex.EncodeToString(HashToken("a")) != "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb" {
		t.Fatal("expected sha-256 of the raw token bytes")
	}
}

func TestAuthenticateResolvesAKnownToken(t *testing.T) {
	expected := Identity{UserID: "user-1", Handle: "reporter", TokenID: "token-1"}
	authenticator := NewAuthenticator(fakeTokenStore{
		knownHash: HashToken("secret"),
		identity:  expected,
	})

	identity, err := authenticator.Authenticate(context.Background(), "Bearer secret")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if identity != expected {
		t.Fatalf("expected %+v, got %+v", expected, identity)
	}
}

func TestAuthenticateAcceptsAnyBearerCasing(t *testing.T) {
	authenticator := NewAuthenticator(fakeTokenStore{
		knownHash: HashToken("secret"),
		identity:  Identity{UserID: "user-1"},
	})

	if _, err := authenticator.Authenticate(context.Background(), "bearer secret"); err != nil {
		t.Fatalf("expected a lowercase scheme to be accepted, got %v", err)
	}
}

func TestAuthenticateRejectsBadCredentials(t *testing.T) {
	authenticator := NewAuthenticator(fakeTokenStore{
		knownHash: HashToken("secret"),
		identity:  Identity{UserID: "user-1"},
	})

	cases := map[string]struct {
		header   string
		expected error
	}{
		"empty header":     {header: "", expected: ErrMissingCredentials},
		"wrong scheme":     {header: "Basic secret", expected: ErrMalformedHeader},
		"no scheme":        {header: "secret", expected: ErrMalformedHeader},
		"empty token":      {header: "Bearer   ", expected: ErrMalformedHeader},
		"unknown token":    {header: "Bearer wrong", expected: ErrUnknownToken},
		"token with space": {header: "Bearer wrong secret", expected: ErrUnknownToken},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := authenticator.Authenticate(context.Background(), testCase.header); !errors.Is(err, testCase.expected) {
				t.Fatalf("expected %v, got %v", testCase.expected, err)
			}
		})
	}
}

func TestAuthenticateDistinguishesLookupFailureFromRejection(t *testing.T) {
	unavailable := errors.New("database unavailable")
	authenticator := NewAuthenticator(fakeTokenStore{err: unavailable})

	_, err := authenticator.Authenticate(context.Background(), "Bearer secret")

	if !errors.Is(err, ErrTokenLookup) {
		t.Fatalf("expected ErrTokenLookup, got %v", err)
	}
	if errors.Is(err, ErrUnknownToken) {
		t.Fatal("an outage must not be reported as an invalid token")
	}
}

func TestIdentityRoundTripsThroughContext(t *testing.T) {
	expected := Identity{UserID: "user-1", Handle: "reporter"}

	identity, ok := IdentityFrom(WithIdentity(context.Background(), expected))
	if !ok {
		t.Fatal("expected an identity on the context")
	}
	if identity != expected {
		t.Fatalf("expected %+v, got %+v", expected, identity)
	}

	if _, ok := IdentityFrom(context.Background()); ok {
		t.Fatal("expected no identity on a bare context")
	}
}
