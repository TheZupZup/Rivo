package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TheZupZup/Rivo/services/api/internal/auth"
)

// TokenStore resolves API tokens to the user that holds them.
type TokenStore struct {
	pool *pgxpool.Pool
}

func NewTokenStore(pool *pgxpool.Pool) TokenStore {
	return TokenStore{pool: pool}
}

// IdentityForTokenHash looks a token up by digest. Expired and revoked tokens are
// filtered in the query rather than reported separately, so every unusable token
// produces the same ErrUnknownToken.
func (store TokenStore) IdentityForTokenHash(ctx context.Context, tokenHash []byte) (auth.Identity, error) {
	const query = `
		SELECT api_tokens.id, users.id, users.handle
		FROM api_tokens
		JOIN users ON users.id = api_tokens.user_id
		WHERE api_tokens.token_hash = $1
		  AND api_tokens.revoked_at IS NULL
		  AND (api_tokens.expires_at IS NULL OR api_tokens.expires_at > NOW())`

	var identity auth.Identity
	err := store.pool.QueryRow(ctx, query, tokenHash).Scan(&identity.TokenID, &identity.UserID, &identity.Handle)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.Identity{}, auth.ErrUnknownToken
		}

		return auth.Identity{}, fmt.Errorf("query api token: %w", err)
	}

	return identity, nil
}
