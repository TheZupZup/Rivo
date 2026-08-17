package httpapi

import (
	"errors"
	"log"
	"net/http"

	"github.com/TheZupZup/Rivo/services/api/internal/auth"
	"github.com/TheZupZup/Rivo/services/api/internal/httpapi/handlers"
)

// requireIdentity rejects a request that does not carry a usable API token and
// attaches the resolved actor to the request context for the handler behind it.
//
// Every failure mode other than an infrastructure error answers 401 with the same
// message, so a caller cannot use the response to distinguish "no such token" from
// "revoked token" and enumerate valid ones.
func requireIdentity(authenticator auth.Authenticator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		identity, err := authenticator.Authenticate(request.Context(), request.Header.Get("Authorization"))
		if err != nil {
			if errors.Is(err, auth.ErrTokenLookup) {
				log.Printf("authentication lookup failed: %v", err)
				handlers.WriteError(w, http.StatusServiceUnavailable, "authentication is temporarily unavailable")
				return
			}

			w.Header().Set("WWW-Authenticate", `Bearer realm="rivo"`)
			handlers.WriteError(w, http.StatusUnauthorized, "a valid API token is required")
			return
		}

		next.ServeHTTP(w, request.WithContext(auth.WithIdentity(request.Context(), identity)))
	})
}
