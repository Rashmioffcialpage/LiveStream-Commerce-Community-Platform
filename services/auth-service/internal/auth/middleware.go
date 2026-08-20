package auth

import (
	"context"
	"net/http"
	"strings"

	"auth-service/internal/model"
)

type ctxKey string

const claimsCtxKey ctxKey = "claims"

// RequireAuth validates the bearer JWT and attaches its claims to the
// request context; downstream handlers read them via ClaimsFromContext
// instead of re-parsing the token.
func RequireAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			token, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || token == "" {
				http.Error(w, `{"error":"missing bearer token"}`, http.StatusUnauthorized)
				return
			}
			claims, err := ParseAccessToken(secret, token)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), claimsCtxKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole wraps a handler that has already passed RequireAuth and
// rejects requests whose token role doesn't match -- e.g. viewer-only
// tokens hitting a creator-dashboard route.
func RequireRole(role model.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok || claims.Role != role {
				http.Error(w, `{"error":"forbidden: requires `+string(role)+` role"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsCtxKey).(*Claims)
	return claims, ok
}
