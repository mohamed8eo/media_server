package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"mediaserver/internal/database"
	"mediaserver/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/mattn/go-sqlite3"
)

type AuthHandler struct {
	db        database.Service
	validator *validator.Validate
	jwt       *JWT
}

func NewRouter(db database.Service) http.Handler {
	validate := validator.New()

	if err := validate.RegisterValidation(
		"strongpassword",
		validatePassword,
	); err != nil {
		panic(err)
	}

	h := &AuthHandler{
		db:        db,
		validator: validate,
		jwt:       NewJWT(os.Getenv("JWT_SECRET")),
	}

	r := chi.NewRouter()

	r.Post("/register", h.RegisterHandler)
	r.Post("/login", h.LoginHandler)
	r.Post("/refresh", h.RefreshHandler)
	r.Post("/reset-password", h.ResetPasswordHandler)
	r.Get("/logout", h.LogoutHandler)

	return r
}

func (h *AuthHandler) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest

	if err := decodeRequest(r, &req); err != nil {
		h.errorResponse(w, r, http.StatusBadRequest, "Bad Request")
		return
	}

	// Validate request.
	if err := h.validator.Struct(req); err != nil {
		h.errorResponse(w, r, http.StatusBadRequest, "Invalid email or password")
		return
	}

	// Hash password.
	hash, err := HashPassword(req.Password)
	if err != nil {
		h.errorResponse(
			w,
			r,
			http.StatusInternalServerError,
			"Failed to hash password",
		)
		return
	}

	// Create user.
	userID, err := h.db.CreateUser(req.Email, hash)
	if err != nil {
		var sqliteErr sqlite3.Error

		if errors.As(err, &sqliteErr) &&
			(sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique ||
				sqliteErr.Code == sqlite3.ErrConstraint) {

			h.errorResponse(
				w,
				r,
				http.StatusConflict,
				"Email is already registered",
			)
			return
		}

		h.errorResponse(
			w,
			r,
			http.StatusInternalServerError,
			"Failed to create user",
		)
		return
	}

	// Generate Access and Refresh Tokens.
	accessToken, _, err := h.jwt.GenerateAccessToken(userID)
	if err != nil {
		h.errorResponse(w, r, http.StatusInternalServerError, "Failed to generate access token")
		return
	}

	refreshToken, refreshExpiresAt, err := h.jwt.GenerateRefreshToken(userID)
	if err != nil {
		h.errorResponse(w, r, http.StatusInternalServerError, "Failed to generate refresh token")
		return
	}

	if _, err := h.db.StoreRefreshToken(userID, refreshToken, refreshExpiresAt); err != nil {
		h.errorResponse(w, r, http.StatusInternalServerError, "Failed to store refresh token")
		return
	}

	SetAuthCookie(w, r, "access_token", accessToken, int(AccessTokenDuration.Seconds()))
	SetAuthCookie(w, r, "refresh_token", refreshToken, int(RefreshTokenDuration.Seconds()))

	// HTMX response.
	if isHTMX(r) {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte(`
			<div class="p-3 bg-emerald-50 border border-emerald-200 text-emerald-700 rounded-xl text-xs font-medium flex items-center space-x-2">
				<svg class="w-4 h-4 text-emerald-500 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
				</svg>
				<span>Account created successfully! Redirecting...</span>
			</div>
		`))

		return
	}

	// JSON response.
	res := AuthResponse{
		UserID: userID.String(),
		// Token:        accessToken,
		// RefreshToken: refreshToken,
		Email:     req.Email,
		CreatedAt: time.Now(),
	}

	utils.RespondWithJSON(w, http.StatusCreated, res)
}

func (h *AuthHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req LogInRequest

	if err := decodeRequest(r, &req); err != nil {
		h.errorResponse(w, r, http.StatusBadRequest, "Bad Request")
		return
	}

	// Validate request.
	if err := h.validator.Struct(req); err != nil {
		h.errorResponse(w, r, http.StatusBadRequest, "Invalid email or password")
		return
	}

	// Find user.
	user, err := h.db.GetUserByEmail(req.Email)
	if err != nil {
		slog.Warn("auth.login.failed", "email", req.Email, "ip", r.RemoteAddr, "reason", "user_not_found")
		// Don't reveal whether the email exists.
		h.errorResponse(
			w,
			r,
			http.StatusUnauthorized,
			"Invalid email or password",
		)
		return
	}

	// Check password.
	if !CheckPassword(req.Password, user.PasswordHash) {
		slog.Warn("auth.login.failed", "email", req.Email, "user_id", user.ID, "ip", r.RemoteAddr, "reason", "invalid_password")
		h.errorResponse(
			w,
			r,
			http.StatusUnauthorized,
			"Invalid email or password",
		)
		return
	}

	slog.Info("auth.login.success", "email", user.Email, "user_id", user.ID, "ip", r.RemoteAddr)

	// Generate Access and Refresh Tokens.
	accessToken, _, err := h.jwt.GenerateAccessToken(user.ID)
	if err != nil {
		h.errorResponse(w, r, http.StatusInternalServerError, "Failed to generate access token")
		return
	}

	refreshToken, refreshExpiresAt, err := h.jwt.GenerateRefreshToken(user.ID)
	if err != nil {
		h.errorResponse(w, r, http.StatusInternalServerError, "Failed to generate refresh token")
		return
	}

	if _, err := h.db.StoreRefreshToken(user.ID, refreshToken, refreshExpiresAt); err != nil {
		h.errorResponse(w, r, http.StatusInternalServerError, "Failed to store refresh token")
		return
	}

	SetAuthCookie(w, r, "access_token", accessToken, int(AccessTokenDuration.Seconds()))
	SetAuthCookie(w, r, "refresh_token", refreshToken, int(RefreshTokenDuration.Seconds()))

	// HTMX response.
	if isHTMX(r) {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte(`
			<div class="p-3 bg-emerald-50 border border-emerald-200 text-emerald-700 rounded-xl text-xs font-medium flex items-center space-x-2">
				<svg class="w-4 h-4 text-emerald-500 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
				</svg>
				<span>Signed in successfully! Redirecting...</span>
			</div>
		`))

		return
	}

	// JSON response.
	res := AuthResponse{
		UserID: user.ID.String(),
		// Token:        accessToken,
		// RefreshToken: refreshToken,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}

	utils.RespondWithJSON(w, http.StatusOK, res)
}

func (h *AuthHandler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("refresh_token"); err == nil && cookie != nil && cookie.Value != "" {
		_ = h.db.RevokeRefreshToken(cookie.Value)
		slog.Info("auth.token.revoked", "ip", r.RemoteAddr)
	}

	SetAuthCookie(w, r, "access_token", "", -1)
	SetAuthCookie(w, r, "refresh_token", "", -1)

	if isHTMX(r) {
		w.Header().Set("HX-Redirect", "/sign-in")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<span>Logged out successfully</span>`))
		return
	}

	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
		return
	}

	http.Redirect(w, r, "/sign-in", http.StatusSeeOther)
}

func (h *AuthHandler) ResetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordRequest

	if err := decodeRequest(r, &req); err != nil {
		h.errorResponse(w, r, http.StatusBadRequest, "Bad Request")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.errorResponse(w, r, http.StatusBadRequest, "Invalid email or password requirements")
		return
	}

	user, err := h.db.GetUserByEmail(req.Email)
	if err != nil {
		h.errorResponse(w, r, http.StatusNotFound, "User not found with this email")
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		h.errorResponse(w, r, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	if err := h.db.UpdatePasswordByEmail(req.Email, hash); err != nil {
		h.errorResponse(w, r, http.StatusInternalServerError, "Failed to update password")
		return
	}

	_ = h.db.RevokeAllUserRefreshTokens(user.ID)

	slog.Info("auth.password.reset", "email", user.Email, "user_id", user.ID, "ip", r.RemoteAddr)

	if isHTMX(r) {
		w.Header().Set("HX-Redirect", "/sign-in")
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte(`
			<div class="p-3 bg-emerald-50 border border-emerald-200 text-emerald-700 rounded-xl text-xs font-medium flex items-center space-x-2">
				<svg class="w-4 h-4 text-emerald-500 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
				</svg>
				<span>Password reset successfully! Redirecting to sign in...</span>
			</div>
		`))

		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Password reset successfully"})
}

func decodeRequest(r *http.Request, dst any) error {
	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "application/json") {
		return json.NewDecoder(r.Body).Decode(dst)
	}

	if err := r.ParseForm(); err != nil {
		return err
	}

	switch v := dst.(type) {
	case *RegisterRequest:
		v.Email = r.FormValue("email")
		v.Password = r.FormValue("password")

	case *LogInRequest:
		v.Email = r.FormValue("email")
		v.Password = r.FormValue("password")

	case *RefreshRequest:
		v.RefreshToken = r.FormValue("refresh_token")

	case *ResetPasswordRequest:
		v.Email = r.FormValue("email")
		v.Password = r.FormValue("password")

	default:
		return errors.New("unsupported request type")
	}

	return nil
}

func (h *AuthHandler) errorResponse(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	message string,
) {
	if isHTMX(r) {
		w.WriteHeader(status)

		_, _ = w.Write([]byte(`
			<div class="p-3 bg-red-50 border border-red-200 text-red-700 rounded-xl text-xs font-medium flex items-center space-x-2">
				<svg class="w-4 h-4 text-red-500 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"/>
				</svg>
				<span>` + message + `</span>
			</div>
		`))

		return
	}

	utils.RespondWithError(w, status, message)
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func (h *AuthHandler) RefreshHandler(w http.ResponseWriter, r *http.Request) {
	if _, accessToken, ok := RefreshAccessFromCookie(w, r, h.db, h.jwt); ok {
		if isHTMX(r) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<span>Token refreshed successfully</span>`))
			return
		}
		utils.RespondWithJSON(w, http.StatusOK, map[string]string{
			"access_token": accessToken,
		})
		return
	}

	// Fallback: refresh token in JSON/form body (API clients without cookies).
	var req RefreshRequest
	if err := decodeRequest(r, &req); err != nil || req.RefreshToken == "" {
		h.errorResponse(w, r, http.StatusUnauthorized, "Missing refresh token")
		return
	}

	claims, err := h.jwt.ValidateToken(req.RefreshToken)
	if err != nil || claims.Type != "refresh" {
		h.errorResponse(w, r, http.StatusUnauthorized, "Invalid refresh token")
		return
	}

	storedToken, err := h.db.GetRefreshToken(req.RefreshToken)
	if err != nil || storedToken.Revoked || time.Now().After(storedToken.ExpiresAt) {
		h.errorResponse(w, r, http.StatusUnauthorized, "Refresh token is expired or revoked")
		return
	}

	accessToken, _, err := h.jwt.GenerateAccessToken(claims.UserID)
	if err != nil {
		h.errorResponse(w, r, http.StatusInternalServerError, "Failed to generate access token")
		return
	}

	SetAuthCookie(w, r, "access_token", accessToken, int(AccessTokenDuration.Seconds()))

	if isHTMX(r) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<span>Token refreshed successfully</span>`))
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{
		"access_token": accessToken,
	})
}
