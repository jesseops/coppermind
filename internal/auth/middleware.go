package auth

import (
	"context"
	"net/http"

	"github.com/jesseops/coppermind/internal/domain"
	"github.com/jesseops/coppermind/internal/store"
)

type contextKey string

const userContextKey contextKey = "user"

// UserFromContext returns the authenticated user from the request context, or nil.
func UserFromContext(ctx context.Context) *domain.User {
	u, _ := ctx.Value(userContextKey).(*domain.User)
	return u
}

// SetUserInContext returns a new context with the user set.
func SetUserInContext(ctx context.Context, user *domain.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// OptionalAuth is middleware that loads the user from the session cookie if present.
// Also supports HTTP Basic Auth for API/OPDS clients.
// Requests continue even without authentication.
func OptionalAuth(s store.Store, secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try session cookie first.
			cookie, err := r.Cookie(CookieName)
			if err == nil && cookie.Value != "" {
				userID, err := ValidateSession(cookie.Value, secret)
				if err == nil {
					user, err := s.GetUser(userID)
					if err == nil {
						ctx := SetUserInContext(r.Context(), user)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
				}
			}

			// Try HTTP Basic Auth (for API/OPDS clients).
			if username, password, ok := r.BasicAuth(); ok {
				user, err := s.GetUserByUsername(username)
				if err == nil {
					if CheckPassword(user.PasswordHash, password) == nil {
						ctx := SetUserInContext(r.Context(), user)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth is middleware that requires an authenticated user.
// Returns 401 if not authenticated.
func RequireAuth(s store.Store, secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil {
				// Try to load from cookie (in case OptionalAuth wasn't in the chain).
				cookie, err := r.Cookie(CookieName)
				if err != nil || cookie.Value == "" {
					http.Redirect(w, r, "/login", http.StatusSeeOther)
					return
				}
				userID, err := ValidateSession(cookie.Value, secret)
				if err != nil {
					http.Redirect(w, r, "/login", http.StatusSeeOther)
					return
				}
				user, err = s.GetUser(userID)
				if err != nil {
					http.Redirect(w, r, "/login", http.StatusSeeOther)
					return
				}
				r = r.WithContext(SetUserInContext(r.Context(), user))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin is middleware that requires an admin user.
func RequireAdmin(s store.Store, secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil {
				cookie, err := r.Cookie(CookieName)
				if err != nil || cookie.Value == "" {
					http.Redirect(w, r, "/login", http.StatusSeeOther)
					return
				}
				userID, err := ValidateSession(cookie.Value, secret)
				if err != nil {
					http.Redirect(w, r, "/login", http.StatusSeeOther)
					return
				}
				user, err = s.GetUser(userID)
				if err != nil {
					http.Redirect(w, r, "/login", http.StatusSeeOther)
					return
				}
				r = r.WithContext(SetUserInContext(r.Context(), user))
			}
			if !user.IsAdmin() {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
