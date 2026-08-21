// See stream-service/internal/auth/jwt.go's package comment: a deliberate
// per-service duplicate, not a shared package.
package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type Role string

type Claims struct {
	UserID      string `json:"sub"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        Role   `json:"role"`
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

// RequireAuthWS: see stream-service/chat-service's identical helper --
// the notification WebSocket has the same "browser can't set a header on
// a WS upgrade" constraint, so it takes the token as a query param too.
func RequireAuthWS(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.URL.Query().Get("token")
			if token == "" {
				if header := r.Header.Get("Authorization"); header != "" {
					token, _ = strings.CutPrefix(header, "Bearer ")
				}
			}
			claims, err := ParseAccessToken(secret, token)
			if err != nil {
				http.Error(w, `{"error":"missing or invalid token (query param or bearer header)"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), claimsCtxKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsCtxKey).(*Claims)
	return claims, ok
}
