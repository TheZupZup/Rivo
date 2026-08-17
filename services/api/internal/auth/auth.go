// Package auth resolves the actor behind a request.
//
// Rivo records who filed every report and who published every upload, because a
// moderation history with anonymous actors cannot be audited. v0.1 has no signup
// flow, so identity comes from tokens issued out of band. Only the SHA-256 digest
// of a token is stored, so a database leak does not hand out working credentials.
package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
)

var (
	ErrMissingCredentials = errors.New("authorization header is required")
	ErrMalformedHeader    = errors.New("authorization header must use the Bearer scheme")
	ErrUnknownToken       = errors.New("unknown, expired or revoked api token")
	ErrTokenLookup        = errors.New("api token lookup failed")
)

// Identity is the authenticated actor for a request.
type Identity struct {
	UserID  string
	Handle  string
	TokenID string
}

// TokenStore looks up an identity by the digest of a presented token. It must
// return ErrUnknownToken for tokens that are absent, expired or revoked, so that
// those cases are indistinguishable to a caller.
type TokenStore interface {
	IdentityForTokenHash(ctx context.Context, tokenHash []byte) (Identity, error)
}

// HashToken derives the value stored in api_tokens.token_hash.
func HashToken(token string) []byte {
	digest := sha256.Sum256([]byte(token))
	return digest[:]
}

type Authenticator struct {
	tokens TokenStore
}

func NewAuthenticator(tokens TokenStore) Authenticator {
	return Authenticator{tokens: tokens}
}

// Authenticate resolves the identity behind an Authorization header value.
func (authenticator Authenticator) Authenticate(ctx context.Context, authorizationHeader string) (Identity, error) {
	token, err := bearerToken(authorizationHeader)
	if err != nil {
		return Identity{}, err
	}

	identity, err := authenticator.tokens.IdentityForTokenHash(ctx, HashToken(token))
	if err != nil {
		if errors.Is(err, ErrUnknownToken) {
			return Identity{}, err
		}

		return Identity{}, errors.Join(ErrTokenLookup, err)
	}

	return identity, nil
}

func bearerToken(authorizationHeader string) (string, error) {
	if authorizationHeader == "" {
		return "", ErrMissingCredentials
	}

	scheme, value, found := strings.Cut(authorizationHeader, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", ErrMalformedHeader
	}

	token := strings.TrimSpace(value)
	if token == "" {
		return "", ErrMalformedHeader
	}

	return token, nil
}

type contextKey struct{}

// WithIdentity stores the authenticated actor on the request context.
func WithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, contextKey{}, identity)
}

// IdentityFrom reads the authenticated actor back. Handlers behind the
// authentication middleware can rely on ok being true.
func IdentityFrom(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(contextKey{}).(Identity)
	return identity, ok
}
