package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const (
	SessionCookieName = "zlog_session"
	SessionMaxAge     = 8 * time.Hour
)

func (m *SessionManager) SetSessionCookie(w http.ResponseWriter, username string) error {
	token, err := m.Sign(username)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   int(SessionMaxAge.Seconds()),
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (m *SessionManager) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

func (m *SessionManager) AuthenticatedUser(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return "", false
	}
	username, err := m.Verify(cookie.Value)
	if err != nil {
		return "", false
	}
	return username, true
}

func sha256digest(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}

func hexEncode(data []byte) string {
	return hex.EncodeToString(data)
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type apiResponse struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (m *SessionManager) LoginHandler(username, passwordHash string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if r.Method == http.MethodPost {
			contentType := r.Header.Get("Content-Type")
			if contentType == "application/json" {
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					writeJSON(w, http.StatusBadRequest, apiResponse{Error: "invalid request"})
					return
				}
			} else {
				r.ParseForm()
				req.Username = r.FormValue("username")
				req.Password = r.FormValue("password")
			}
		} else {
			// Show login page or reject
			writeJSON(w, http.StatusMethodNotAllowed, apiResponse{Error: "method not allowed"})
			return
		}

		if req.Username != username || !VerifyPassword(passwordHash, req.Password) {
			writeJSON(w, http.StatusUnauthorized, apiResponse{Error: "invalid credentials"})
			return
		}

		if err := m.SetSessionCookie(w, req.Username); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiResponse{Error: "session error"})
			return
		}

		if r.Header.Get("Accept") == "application/json" || r.Header.Get("Content-Type") == "application/json" {
			writeJSON(w, http.StatusOK, apiResponse{OK: true, Message: "logged in"})
		} else {
			http.Redirect(w, r, "/", http.StatusSeeOther)
		}
	}
}

func (m *SessionManager) LogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m.ClearSessionCookie(w)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

func (m *SessionManager) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := m.AuthenticatedUser(r); !ok {
			if isAPIRequest(r) {
				writeJSON(w, http.StatusUnauthorized, apiResponse{Error: "unauthorized"})
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func isAPIRequest(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/api/")
}
