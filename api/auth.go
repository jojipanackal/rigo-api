package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jojipanackal/rigo-api/middlewares"
	"github.com/jojipanackal/rigo-api/models"
)

type AuthHandler struct {
	AuthModel *models.AuthModel
	UserModel *models.UserModel
}

// POST /api/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.UserModel.GetByEmail(body.Email)
	if err != nil || !h.UserModel.VerifyPassword(user, body.Password) {
		WriteError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	token, err := h.AuthModel.CreateToken(user.Id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not create token")
		return
	}
	if err = h.AuthModel.CreateAuth(user.Id, token); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not store session")
		return
	}
	setAuthCookies(w, token)

	WriteJSON(w, http.StatusOK, map[string]any{
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
		"expires_at":    time.Unix(token.AtExpires, 0),
		"user":          user,
	})
}

// POST /api/auth/signup
func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Name == "" || body.Email == "" || len(body.Password) < 8 {
		WriteError(w, http.StatusBadRequest, "name, email, and password (min 8 chars) are required")
		return
	}
	if h.UserModel.EmailExists(body.Email) {
		WriteError(w, http.StatusConflict, "email is already registered")
		return
	}

	userId, err := h.UserModel.Create(body.Name, body.Email, body.Password)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not create user")
		return
	}

	token, err := h.AuthModel.CreateToken(userId)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not create token")
		return
	}
	h.AuthModel.CreateAuth(userId, token)
	setAuthCookies(w, token)

	user, _ := h.UserModel.GetById(userId)
	WriteJSON(w, http.StatusCreated, map[string]any{
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
		"expires_at":    time.Unix(token.AtExpires, 0),
		"user":          user,
	})
}

// POST /api/auth/logout  (requires Bearer token)
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var revoked bool

	if metadata, err := h.AuthModel.ExtractTokenMetadata(r); err == nil {
		h.AuthModel.DeleteAuth(metadata.AccessUUID)
		revoked = true
	}

	if refreshToken := h.AuthModel.ExtractRefreshToken(r); refreshToken != "" {
		if refreshMeta, err := h.AuthModel.ValidateRefreshToken(refreshToken); err == nil {
			h.AuthModel.DeleteAuth(refreshMeta.RefreshUUID)
			revoked = true
		}
	}

	clearAuthCookies(w)

	if !revoked {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// POST /api/auth/refresh  — body: {"refresh_token": "..."}
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RefreshToken == "" {
		WriteError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	// Validate refresh token and extract user_id
	refreshMeta, err := h.AuthModel.ValidateRefreshToken(body.RefreshToken)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}
	h.AuthModel.DeleteAuth(refreshMeta.RefreshUUID)
	userId := refreshMeta.UserID

	// Issue new access token
	token, err := h.AuthModel.CreateToken(userId)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not create token")
		return
	}
	h.AuthModel.CreateAuth(userId, token)
	setAuthCookies(w, token)

	WriteJSON(w, http.StatusOK, map[string]any{
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
		"expires_at":    time.Unix(token.AtExpires, 0),
	})
}

// GET /api/auth/me  — returns current user from token
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userId := middlewares.GetUserID(r.Context())
	user, err := h.UserModel.GetById(userId)
	if err != nil {
		WriteError(w, http.StatusNotFound, "user not found")
		return
	}
	WriteJSON(w, http.StatusOK, user)
}

func setAuthCookies(w http.ResponseWriter, token *models.Token) {
	setCookie := func(name, value string, expires time.Time) {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    value,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   false,
			Expires:  expires,
			MaxAge:   int(time.Until(expires).Seconds()),
		})
	}

	setCookie("access_token", token.AccessToken, time.Unix(token.AtExpires, 0))
	setCookie("refresh_token", token.RefreshToken, time.Unix(token.RtExpires, 0))
}

func clearAuthCookies(w http.ResponseWriter) {
	expireCookie := func(name string) {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   false,
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
		})
	}

	expireCookie("access_token")
	expireCookie("refresh_token")
}
