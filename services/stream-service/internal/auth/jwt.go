// Package auth verifies JWTs issued by auth-service. It deliberately does
// not import anything from auth-service -- each service owns its own copy
// of the tiny bit of verification logic it needs, so the two can be built,
// tested, and deployed independently. The contract between them is just
// "same JWT_SECRET, same claim shape", not a shared Go package.
package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type Role string

const (
	RoleViewer  Role = "viewer"
	RoleCreator Role = "creator"
)

type Claims struct {
	UserID string `json:"sub"`
	Email  string `json:"email"`
	Role   Role   `json:"role"`
	jwt.RegisteredClaims
}

var ErrInvalidToken = errors.New("invalid or expired token")

func ParseAccessToken(secret, tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

type ctxKey string

const claimsCtxKey ctxKey = "claims"

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

func RequireRole(role Role) func(http.Handler) http.Handler {
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

// OptionalAuth attaches claims to the context if a valid bearer token is
// present, but never rejects the request -- for routes like the WebRTC
// signaling endpoint where a viewer connects unauthenticated but a
// broadcaster connects with a token.
func OptionalAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if token, ok := strings.CutPrefix(header, "Bearer "); ok && token != "" {
				if claims, err := ParseAccessToken(secret, token); err == nil {
					r = r.WithContext(context.WithValue(r.Context(), claimsCtxKey, claims))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
