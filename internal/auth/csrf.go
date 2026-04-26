package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const (
	csrfCookieName = "coppermind_csrf"
	csrfHeaderName = "X-CSRF-Token"
	csrfFormField  = "csrf_token"
)

// CSRFProtect is middleware that implements the double-submit cookie pattern.
func CSRFProtect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Ensure CSRF cookie exists.
		cookie, err := r.Cookie(csrfCookieName)
		if err != nil || cookie.Value == "" {
			token := generateCSRFToken()
			http.SetCookie(w, &http.Cookie{
				Name:     csrfCookieName,
				Value:    token,
				Path:     "/",
				HttpOnly: false, // JS needs to read this
				Secure:   r.TLS != nil,
				SameSite: http.SameSiteStrictMode,
			})
			cookie = &http.Cookie{Value: token}
		}

		// For safe methods, just continue.
		switch r.Method {
		case "GET", "HEAD", "OPTIONS":
			next.ServeHTTP(w, r)
			return
		}

		// For state-changing methods, validate the token.
		token := r.Header.Get(csrfHeaderName)
		if token == "" {
			token = r.FormValue(csrfFormField)
		}

		if token == "" || token != cookie.Value {
			http.Error(w, "CSRF token mismatch", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// CSRFToken returns the current CSRF token from the request cookies.
func CSRFToken(r *http.Request) string {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func generateCSRFToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
