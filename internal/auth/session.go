package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// CookieName is the session cookie name.
	CookieName = "coppermind_session"

	// DefaultExpiry is the default session duration.
	DefaultExpiry = 24 * time.Hour

	// RememberMeExpiry is the extended session duration.
	RememberMeExpiry = 30 * 24 * time.Hour
)

// CreateSession creates a signed session token and returns the cookie value and expiry.
// Format: base64(userID|expiryUnix|signature)
func CreateSession(userID int64, rememberMe bool, secret []byte) (string, time.Time) {
	expiry := time.Now().Add(DefaultExpiry)
	if rememberMe {
		expiry = time.Now().Add(RememberMeExpiry)
	}

	payload := fmt.Sprintf("%d|%d", userID, expiry.Unix())
	sig := sign(payload, secret)
	token := payload + "|" + sig

	return base64.URLEncoding.EncodeToString([]byte(token)), expiry
}

// ValidateSession validates a session token and returns the user ID.
func ValidateSession(cookieValue string, secret []byte) (int64, error) {
	raw, err := base64.URLEncoding.DecodeString(cookieValue)
	if err != nil {
		return 0, fmt.Errorf("invalid session encoding")
	}

	parts := strings.SplitN(string(raw), "|", 3)
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid session format")
	}

	userID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid user ID in session")
	}

	expiryUnix, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid expiry in session")
	}

	// Check signature.
	payload := parts[0] + "|" + parts[1]
	expectedSig := sign(payload, secret)
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return 0, fmt.Errorf("invalid session signature")
	}

	// Check expiry.
	if time.Now().Unix() > expiryUnix {
		return 0, fmt.Errorf("session expired")
	}

	return userID, nil
}

// SetSessionCookie sets the session cookie on the response.
func SetSessionCookie(w http.ResponseWriter, r *http.Request, value string, expiry time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		Expires:  expiry,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})
}

func sign(payload string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return base64.URLEncoding.EncodeToString(mac.Sum(nil))
}
