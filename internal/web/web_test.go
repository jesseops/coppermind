package web

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jesseops/coppermind/internal/auth"
	"github.com/jesseops/coppermind/internal/config"
	"github.com/jesseops/coppermind/internal/domain"
	"github.com/jesseops/coppermind/internal/store"
)

// testEnv holds a test server and helper methods.
type testEnv struct {
	t      *testing.T
	srv    *Server
	ts     *httptest.Server
	store  *store.SQLiteStore
	client *http.Client
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		DataDir:       t.TempDir(),
		Host:          "127.0.0.1",
		Port:          0,
		SessionSecret: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		AllowGuests:   true,
	}
	srv, err := NewServer(s, cfg)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // don't follow redirects
		},
	}
	t.Cleanup(func() {
		ts.Close()
		s.Close()
	})
	return &testEnv{t: t, srv: srv, ts: ts, store: s, client: client}
}

func (e *testEnv) createUser(username, password, role string) *domain.User {
	e.t.Helper()
	hash, _ := auth.HashPassword(password)
	user, err := e.store.CreateUser(username, username, hash, role)
	if err != nil {
		e.t.Fatal(err)
	}
	return user
}

func (e *testEnv) createLibrary(name string) *domain.Library {
	e.t.Helper()
	lib, err := e.store.CreateLibrary(name)
	if err != nil {
		e.t.Fatal(err)
	}
	return lib
}

func (e *testEnv) login(username, password string) {
	e.t.Helper()
	// Get CSRF cookie.
	resp, _ := e.client.Get(e.ts.URL + "/login")
	resp.Body.Close()

	// Extract CSRF token from cookie.
	u, _ := url.Parse(e.ts.URL)
	var csrfToken string
	for _, c := range e.client.Jar.Cookies(u) {
		if c.Name == "coppermind_csrf" {
			csrfToken = c.Value
		}
	}

	// POST login.
	form := url.Values{
		"username":   {username},
		"password":   {password},
		"csrf_token": {csrfToken},
	}
	resp, err := e.client.PostForm(e.ts.URL+"/login", form)
	if err != nil {
		e.t.Fatal(err)
	}
	resp.Body.Close()
}

func (e *testEnv) csrfToken() string {
	u, _ := url.Parse(e.ts.URL)
	for _, c := range e.client.Jar.Cookies(u) {
		if c.Name == "coppermind_csrf" {
			return c.Value
		}
	}
	return ""
}

// ── Tests ───────────────────────────────────────────────────────────

func TestHealthEndpoint(t *testing.T) {
	env := newTestEnv(t)
	resp, err := env.client.Get(env.ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("health: got %d, want 200", resp.StatusCode)
	}
}

func TestLoginFlow(t *testing.T) {
	env := newTestEnv(t)
	env.createUser("alice", "secret123", domain.RoleAdmin)

	t.Run("valid credentials", func(t *testing.T) {
		env.login("alice", "secret123")
		// Should redirect to /
		resp, _ := env.client.Get(env.ts.URL + "/me")
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("got %d after login, want 200 on /me", resp.StatusCode)
		}
	})

	t.Run("invalid credentials", func(t *testing.T) {
		env2 := newTestEnv(t)
		env2.createUser("bob", "pass", domain.RoleViewer)
		// Get CSRF.
		resp, _ := env2.client.Get(env2.ts.URL + "/login")
		resp.Body.Close()
		token := env2.csrfToken()

		form := url.Values{
			"username":   {"bob"},
			"password":   {"wrong"},
			"csrf_token": {token},
		}
		resp, _ = env2.client.PostForm(env2.ts.URL+"/login", form)
		resp.Body.Close()
		// Should render login page again (200), not redirect.
		if resp.StatusCode != 200 {
			t.Fatalf("got %d, want 200 (re-rendered login)", resp.StatusCode)
		}
	})
}

func TestCSRFEnforcement(t *testing.T) {
	env := newTestEnv(t)
	env.createUser("admin", "pass", domain.RoleAdmin)
	env.createLibrary("Test")
	env.login("admin", "pass")

	t.Run("POST without CSRF token returns 403", func(t *testing.T) {
		req, _ := http.NewRequest("POST", env.ts.URL+"/admin/import", strings.NewReader(""))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := env.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 403 {
			t.Fatalf("got %d, want 403", resp.StatusCode)
		}
	})
}

func TestAdminRouteAccess(t *testing.T) {
	env := newTestEnv(t)
	env.createUser("viewer", "pass", domain.RoleViewer)
	env.createUser("admin", "adminpass", domain.RoleAdmin)
	env.createLibrary("Test")

	t.Run("viewer cannot access admin routes", func(t *testing.T) {
		env.login("viewer", "pass")
		resp, _ := env.client.Get(env.ts.URL + "/admin/users")
		resp.Body.Close()
		if resp.StatusCode != 403 {
			t.Fatalf("viewer got %d on /admin/users, want 403", resp.StatusCode)
		}
	})

	t.Run("admin can access admin routes", func(t *testing.T) {
		env2 := newTestEnv(t)
		env2.createUser("admin2", "pass", domain.RoleAdmin)
		env2.createLibrary("Lib")
		env2.login("admin2", "pass")
		resp, _ := env2.client.Get(env2.ts.URL + "/admin/users")
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("admin got %d on /admin/users, want 200", resp.StatusCode)
		}
	})
}

func TestGuestGateWhenDisabled(t *testing.T) {
	s, _ := store.NewMemoryStore()
	cfg := &config.Config{
		DataDir:       t.TempDir(),
		Host:          "127.0.0.1",
		Port:          0,
		SessionSecret: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		AllowGuests:   false,
	}
	srv, _ := NewServer(s, cfg)
	ts := httptest.NewServer(srv)
	defer ts.Close()
	defer s.Close()

	hash, _ := auth.HashPassword("pass")
	s.CreateUser("admin", "admin", hash, domain.RoleAdmin)
	s.CreateLibrary("Lib")

	t.Run("unauthenticated redirected to login", func(t *testing.T) {
		client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}}
		resp, _ := client.Get(ts.URL + "/")
		resp.Body.Close()
		if resp.StatusCode != 303 {
			t.Fatalf("got %d, want 303 redirect", resp.StatusCode)
		}
		loc := resp.Header.Get("Location")
		if loc != "/login" {
			t.Fatalf("redirect to %q, want /login", loc)
		}
	})

	t.Run("API returns 401 with WWW-Authenticate", func(t *testing.T) {
		client := &http.Client{}
		resp, _ := client.Get(ts.URL + "/api/v1/works")
		resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Fatalf("got %d, want 401", resp.StatusCode)
		}
		if !strings.Contains(resp.Header.Get("WWW-Authenticate"), "Basic") {
			t.Fatal("missing WWW-Authenticate header")
		}
	})

	t.Run("Basic Auth grants access", func(t *testing.T) {
		client := &http.Client{}
		req, _ := http.NewRequest("GET", ts.URL+"/api/v1/works", nil)
		req.SetBasicAuth("admin", "pass")
		resp, _ := client.Do(req)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("got %d with Basic Auth, want 200", resp.StatusCode)
		}
	})

	t.Run("health always accessible", func(t *testing.T) {
		client := &http.Client{}
		resp, _ := client.Get(ts.URL + "/health")
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("health got %d, want 200", resp.StatusCode)
		}
	})
}

func TestShelfOwnership(t *testing.T) {
	env := newTestEnv(t)
	alice := env.createUser("alice", "pass", domain.RoleViewer)
	env.createUser("bob", "pass", domain.RoleViewer)
	env.createLibrary("Lib")

	// Alice creates a private shelf.
	shelf, _ := env.store.CreateShelf(alice.ID, "My Shelf", "", false)

	t.Run("bob cannot add to alice shelf", func(t *testing.T) {
		env2 := newTestEnv(t)
		bob := env2.createUser("bob2", "pass", domain.RoleViewer)
		_ = bob
		env2.createLibrary("L")
		shelf2, _ := env2.store.CreateShelf(1, "Other", "", false) // belongs to user 1 (bob2)
		_ = shelf2
		env2.login("bob2", "pass")

		// Try to add to shelf that doesn't belong to bob2.
		alice2 := env2.createUser("alice2", "pass", domain.RoleViewer)
		aliceShelf, _ := env2.store.CreateShelf(alice2.ID, "Alice Shelf", "", false)

		token := env2.csrfToken()
		form := url.Values{
			"work_id":    {"1"},
			"csrf_token": {token},
		}
		resp, _ := env2.client.PostForm(env2.ts.URL+"/shelves/"+strings.TrimSpace(itoa(aliceShelf.ID))+"/add", form)
		resp.Body.Close()
		if resp.StatusCode != 403 {
			t.Fatalf("got %d, want 403", resp.StatusCode)
		}
	})

	_ = shelf // used above conceptually
}

func TestRateLimit(t *testing.T) {
	env := newTestEnv(t)
	env.createUser("user", "pass", domain.RoleViewer)

	// Get CSRF.
	resp, _ := env.client.Get(env.ts.URL + "/login")
	resp.Body.Close()
	token := env.csrfToken()

	// Make 5 failed attempts.
	for i := 0; i < 5; i++ {
		form := url.Values{
			"username":   {"user"},
			"password":   {"wrong"},
			"csrf_token": {token},
		}
		resp, _ = env.client.PostForm(env.ts.URL+"/login", form)
		resp.Body.Close()
	}

	// 6th attempt should be rate-limited.
	form := url.Values{
		"username":   {"user"},
		"password":   {"wrong"},
		"csrf_token": {token},
	}
	resp, _ = env.client.PostForm(env.ts.URL+"/login", form)
	resp.Body.Close()
	if resp.StatusCode != 429 {
		t.Fatalf("got %d, want 429 after rate limit", resp.StatusCode)
	}
}

func TestTemplateSmoke(t *testing.T) {
	env := newTestEnv(t)
	env.createUser("admin", "pass", domain.RoleAdmin)
	lib := env.createLibrary("Test")
	_ = lib
	env.login("admin", "pass")

	pages := []string{
		"/",
		"/authors",
		"/series",
		"/me",
		"/me/shelves",
		"/me/reading",
		"/admin/import",
		"/admin/users",
		"/admin/duplicates",
		"/receive",
	}
	for _, path := range pages {
		t.Run(path, func(t *testing.T) {
			resp, err := env.client.Get(env.ts.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Fatalf("%s: got %d, want 200", path, resp.StatusCode)
			}
		})
	}
}

func itoa(i int64) string {
	return fmt.Sprintf("%d", i)
}
