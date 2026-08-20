package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"auth-service/internal/auth"
	"auth-service/internal/config"
	"auth-service/internal/db"
	"auth-service/internal/model"
)

type Handler struct {
	DB          *db.DB
	Cfg         config.Config
	GoogleOAuth *oauth2.Config
	// oauthStates guards against CSRF on the OAuth callback: a real
	// deployment with multiple replicas should move this to Redis instead
	// of an in-memory map, same reasoning as fraud-detection's Redis-backed
	// idempotency guards -- a single process's memory doesn't survive a
	// pod restart or fan out across replicas.
	oauthStates map[string]time.Time
}

func New(database *db.DB, cfg config.Config) *Handler {
	h := &Handler{DB: database, Cfg: cfg, oauthStates: map[string]time.Time{}}
	if cfg.GoogleClientID != "" {
		h.GoogleOAuth = auth.GoogleOAuthConfig(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL)
	}
	return h
}

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type signupRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"` // "viewer" (default) or "creator"
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || len(req.Password) < 8 || req.DisplayName == "" {
		writeError(w, http.StatusBadRequest, "email, display_name, and a password of at least 8 characters are required")
		return
	}
	role := model.RoleViewer
	if req.Role == string(model.RoleCreator) {
		role = model.RoleCreator
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not hash password")
		return
	}

	user, err := h.DB.CreateUser(r.Context(), req.Email, &hash, req.DisplayName, role)
	if errors.Is(err, db.ErrConflict) {
		writeError(w, http.StatusConflict, "an account with that email already exists")
		return
	}
	if err != nil {
		slog.Error("signup: create user", "err", err)
		writeError(w, http.StatusInternalServerError, "could not create account")
		return
	}

	h.issueTokens(w, r, user)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	user, err := h.DB.GetUserByEmail(r.Context(), req.Email)
	if errors.Is(err, db.ErrNotFound) || user.PasswordHash == nil || !auth.CheckPassword(*user.PasswordHash, req.Password) {
		// same error for "no such user" and "wrong password" -- don't leak
		// which one it was
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if err != nil {
		slog.Error("login: get user", "err", err)
		writeError(w, http.StatusInternalServerError, "login failed")
		return
	}

	h.issueTokens(w, r, user)
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	userID, err := h.DB.ValidRefreshToken(r.Context(), auth.HashToken(req.RefreshToken))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}
	user, err := h.DB.GetUserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	// rotate: revoke the presented refresh token and issue a new pair, so a
	// leaked refresh token has a single-use window rather than living for
	// its full 30-day TTL
	_ = h.DB.RevokeRefreshToken(r.Context(), auth.HashToken(req.RefreshToken))
	h.issueTokens(w, r, user)
}

// CreatorOnlyPing exists only to prove RequireRole works end to end; the
// real creator-dashboard routes land in stream-service / commerce-service.
func (h *Handler) CreatorOnlyPing(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	writeJSON(w, http.StatusOK, map[string]string{"message": "welcome, creator " + claims.Email})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	user, err := h.DB.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) issueTokens(w http.ResponseWriter, r *http.Request, user *model.User) {
	access, err := auth.GenerateAccessToken(h.Cfg.JWTSecret, h.Cfg.AccessTokenTTL, user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not issue token")
		return
	}
	refreshPlain, refreshHash, err := auth.GenerateRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not issue token")
		return
	}
	if err := h.DB.StoreRefreshToken(r.Context(), user.ID, refreshHash, time.Now().Add(h.Cfg.RefreshTokenTTL)); err != nil {
		writeError(w, http.StatusInternalServerError, "could not issue token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  access,
		"refresh_token": refreshPlain,
		"expires_in":    int(h.Cfg.AccessTokenTTL.Seconds()),
		"user":          user,
	})
}

// --- Google OAuth ---

func (h *Handler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	if h.GoogleOAuth == nil {
		writeError(w, http.StatusNotImplemented, "Google OAuth is not configured on this server (GOOGLE_CLIENT_ID unset)")
		return
	}
	state := randomState()
	h.oauthStates[state] = time.Now().Add(10 * time.Minute)
	http.Redirect(w, r, h.GoogleOAuth.AuthCodeURL(state, oauth2.AccessTypeOnline), http.StatusFound)
}

type googleUserInfo struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (h *Handler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	if h.GoogleOAuth == nil {
		writeError(w, http.StatusNotImplemented, "Google OAuth is not configured on this server")
		return
	}
	state := r.URL.Query().Get("state")
	if exp, ok := h.oauthStates[state]; !ok || time.Now().After(exp) {
		writeError(w, http.StatusBadRequest, "invalid or expired oauth state")
		return
	}
	delete(h.oauthStates, state)

	code := r.URL.Query().Get("code")
	token, err := h.GoogleOAuth.Exchange(r.Context(), code)
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not exchange oauth code")
		return
	}

	resp, err := h.GoogleOAuth.Client(r.Context(), token).Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not fetch google profile")
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not read google profile")
		return
	}
	var info googleUserInfo
	if err := json.Unmarshal(body, &info); err != nil || info.Email == "" {
		writeError(w, http.StatusBadGateway, "malformed google profile response")
		return
	}

	user, err := h.DB.GetOrCreateUserByOAuth(r.Context(), "google", info.ID, strings.ToLower(info.Email), info.Name)
	if err != nil {
		slog.Error("oauth: get or create user", "err", err)
		writeError(w, http.StatusInternalServerError, "could not sign in with google")
		return
	}
	h.issueTokens(w, r, user)
}

func randomState() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return base64.RawURLEncoding.EncodeToString(buf)
}

// --- helpers ---

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
