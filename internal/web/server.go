package web

import (
	"compress/gzip"
	"context"
	cryptoRand "crypto/rand"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"github.com/jesseops/coppermind/internal/auth"
	"github.com/jesseops/coppermind/internal/config"
	"github.com/jesseops/coppermind/internal/importer"
	"github.com/jesseops/coppermind/internal/store"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

// Server is the main HTTP server.
type Server struct {
	store       store.Store
	importer    *importer.Importer
	config      *config.Config
	pages       map[string]*template.Template // per-page templates (each includes base)
	secret      []byte
	router      chi.Router
	sends       *sendStore    // in-memory key store for send-to-ereader
	loginLimiter *rateLimiter // IP-based login rate limiter
}

// NewServer creates a new web server.
func NewServer(s store.Store, cfg *config.Config) (*Server, error) {
	srv := &Server{
		store:        s,
		importer:     importer.NewImporter(s, cfg.DataDir),
		config:       cfg,
		secret:       cfg.SessionSecretBytes(),
		sends:        newSendStore(),
		loginLimiter: newRateLimiter(5, 60*time.Second),
	}

	// Parse templates — each page gets its own clone of the base template
	// so that "title" and "content" block definitions don't collide.
	funcs := template.FuncMap{
		"urlquery":  url.QueryEscape,
		"lower":     strings.ToLower,
		"title":     cases.Title(language.English).String,
		"hasPrefix": strings.HasPrefix,
		"csrfToken": func() string { return "" },
		"currentUser": func() any { return nil },
		"formatDate": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Format("Jan 2, 2006")
		},
		"formatBytes": func(b int64) string {
			const (
				KB = 1024
				MB = KB * 1024
				GB = MB * 1024
			)
			switch {
			case b >= GB:
				return fmt.Sprintf("%.1f GB", float64(b)/float64(GB))
			case b >= MB:
				return fmt.Sprintf("%.1f MB", float64(b)/float64(MB))
			case b >= KB:
				return fmt.Sprintf("%.0f KB", float64(b)/float64(KB))
			default:
				return fmt.Sprintf("%d B", b)
			}
		},
		"formatDuration": func(seconds int) string {
			h := seconds / 3600
			m := (seconds % 3600) / 60
			if h > 0 {
				return fmt.Sprintf("%dh %dm", h, m)
			}
			return fmt.Sprintf("%dm", m)
		},
		"coverInitials": coverInitials,
		"subtract": func(a, b int) int { return a - b },
		"add":      func(a, b int) int { return a + b },
		"multiply": func(a float64, b int) float64 { return a * float64(b) },
		"dict": func(values ...any) map[string]any {
			m := make(map[string]any)
			for i := 0; i+1 < len(values); i += 2 {
				key, _ := values[i].(string)
				m[key] = values[i+1]
			}
			return m
		},
	}

	// Parse base template first.
	baseContent, err := fs.ReadFile(templatesFS, "templates/base.html")
	if err != nil {
		return nil, fmt.Errorf("read base template: %w", err)
	}
	baseTpl, err := template.New("base.html").Funcs(funcs).Parse(string(baseContent))
	if err != nil {
		return nil, fmt.Errorf("parse base template: %w", err)
	}

	// For each page template, clone the base and parse the page into it.
	pages := make(map[string]*template.Template)
	entries, err := fs.ReadDir(templatesFS, "templates")
	if err != nil {
		return nil, fmt.Errorf("read templates dir: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "base.html" {
			continue
		}
		pageContent, err := fs.ReadFile(templatesFS, "templates/"+name)
		if err != nil {
			return nil, fmt.Errorf("read template %s: %w", name, err)
		}
		pageTpl, err := template.Must(baseTpl.Clone()).Parse(string(pageContent))
		if err != nil {
			return nil, fmt.Errorf("parse template %s: %w", name, err)
		}
		pages[name] = pageTpl
	}
	srv.pages = pages

	// Build router.
	srv.router = srv.buildRouter()

	return srv, nil
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// Run starts the HTTP server with graceful shutdown.
func (s *Server) Run() error {
	// Warn if session secret was auto-generated (won't survive restarts).
	if os.Getenv("COPPERMIND_SESSION_SECRET") == "" {
		slog.Warn("no COPPERMIND_SESSION_SECRET set — sessions will not survive restarts")
	}

	addr := s.config.Addr()
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           s,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       60 * time.Second,
	}

	slog.Info("server starting", "addr", addr, "url", "http://"+addr, "db", s.config.DBPath)

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		slog.Info("shutdown received")
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutCtx); err != nil {
			return err
		}
		<-errCh
		slog.Info("shutdown complete")
		return nil
	}
}

func (s *Server) buildRouter() chi.Router {
	r := chi.NewRouter()

	// Middleware stack.
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger)
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)
	r.Use(gzipMiddleware)
	r.Use(auth.OptionalAuth(s.store, s.secret))
	r.Use(s.guestGateMiddleware)
	r.Use(s.setupCheckMiddleware)

	// Health check (no auth, no CSRF).
	r.Get("/health", s.handleHealth)

	// Static files (no CSRF needed).
	staticRoot, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticRoot))))

	// ── Routes exempt from CSRF (e-reader API, JSON API, OPDS) ──────
	r.Post("/receive/generate", s.handleReceiveGenerate)
	r.Get("/receive/status/{key}", s.handleReceiveStatus)
	r.Get("/receive/download/{key}/{filename}", s.handleReceiveDownload)
	s.registerAPIRoutes(r)
	s.registerOPDSRoutes(r)

	// ── All other routes with CSRF protection ───────────────────────
	r.Group(func(r chi.Router) {
		r.Use(auth.CSRFProtect)

		// Public routes.
		r.Get("/", s.handleHome)
		r.Get("/works/{id}", s.handleWorkDetail)
		r.Get("/authors", s.handleAuthors)
		r.Get("/authors/{id}", s.handleAuthorDetail)
		r.Get("/series", s.handleSeriesList)
		r.Get("/series/{id}", s.handleSeriesDetail)
		r.Get("/covers/{id}", s.handleCover)
		r.Get("/covers/edition/{id}", s.handleEditionCover)
		r.Get("/download/{id}", s.handleDownload)
		r.Get("/receive", s.handleReceivePage)

		// Auth routes.
		r.Get("/login", s.handleLoginForm)
		r.Post("/login", s.handleLoginSubmit)
		r.Get("/logout", s.handleLogout)
		r.Get("/setup", s.handleSetupForm)
		r.Post("/setup", s.handleSetupSubmit)

		// Reader/player.
		r.Get("/read/{id}", s.handleReader)
		r.Get("/listen/{id}", s.handlePlayer)
		r.Get("/epub-asset/{id}/*", s.handleEpubAsset)
		r.Get("/audio/{id}", s.handleAudioTrack)

		// Send with code (uses CSRF since it's from the main UI).
		r.Post("/receive/send/{edition_id}", s.handleSendWithCode)

		// User routes (require auth).
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth(s.store, s.secret))
			r.Get("/me", s.handleProfile)
			r.Get("/me/reading", s.handleCurrentlyReading)
			r.Post("/api/v1/me/reading/{id}", s.handleSaveReadingState)
		})

		// User routes (shelves, ratings, profile).
		s.registerUserRoutes(r)

		// Admin routes.
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAdmin(s.store, s.secret))
			r.Get("/admin/import", s.handleAdminImportForm)
			r.Post("/admin/import", s.handleAdminImportSubmit)
			r.Get("/admin/works/{id}/edit", s.handleAdminEditForm)
			r.Post("/admin/works/{id}", s.handleAdminEditSubmit)
			r.Post("/admin/works/{id}/delete", s.handleAdminDeleteWork)
			r.Post("/admin/editions/{id}", s.handleAdminUpdateEdition)
			r.Get("/admin/works/{id}/covers/search", s.handleAdminCoverSearch)
			r.Post("/admin/works/{id}/covers/apply", s.handleAdminCoverApply)
			r.Get("/admin/users", s.handleAdminUsers)
			r.Post("/admin/users", s.handleAdminCreateUser)
			r.Get("/admin/duplicates", s.handleAdminDuplicates)
		})
	})

	return r
}

// setupCheckMiddleware redirects to /setup if no users exist.
func (s *Server) setupCheckMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip for setup and static routes.
		if strings.HasPrefix(r.URL.Path, "/setup") || strings.HasPrefix(r.URL.Path, "/static") || r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		count, _ := s.store.CountUsers()
		if count == 0 {
			http.Redirect(w, r, "/setup", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// guestGateMiddleware enforces authentication when AllowGuests is false.
// Skips auth-related paths so users can still log in.
func (s *Server) guestGateMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always allow if guests are permitted.
		if s.config.AllowGuests {
			next.ServeHTTP(w, r)
			return
		}

		// Skip paths that must remain accessible.
		path := r.URL.Path
		if path == "/login" || path == "/logout" ||
			strings.HasPrefix(path, "/setup") ||
			strings.HasPrefix(path, "/static/") ||
			strings.HasPrefix(path, "/receive/") ||
			path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		// If user is authenticated (via session or Basic Auth), allow.
		user := auth.UserFromContext(r.Context())
		if user != nil {
			next.ServeHTTP(w, r)
			return
		}

		// For API/OPDS requests, return 401 with WWW-Authenticate.
		if strings.HasPrefix(path, "/api/") {
			w.Header().Set("WWW-Authenticate", `Basic realm="Coppermind"`)
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}

		// For browser requests, redirect to login.
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
}

// ── Template rendering ──────────────────────────────────────────────

type templateData map[string]any

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data templateData) {
	if data == nil {
		data = templateData{}
	}
	data["CSRFToken"] = auth.CSRFToken(r)
	data["CurrentUser"] = auth.UserFromContext(r.Context())
	data["Config"] = s.config
	data["Nonce"] = generateNonce()

	tpl, ok := s.pages[name]
	if !ok {
		slog.Error("template not found", "name", name)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Set CSP header with nonce.
	nonce := data["Nonce"].(string)
	csp := fmt.Sprintf(
		"default-src 'self'; script-src 'self' 'nonce-%s'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; img-src 'self' https://covers.openlibrary.org data:; connect-src 'self'",
		nonce,
	)
	w.Header().Set("Content-Security-Policy", csp)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tpl.ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("render template", "name", name, "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (s *Server) renderPartial(w http.ResponseWriter, r *http.Request, name string, data templateData) {
	if data == nil {
		data = templateData{}
	}
	data["CSRFToken"] = auth.CSRFToken(r)
	data["CurrentUser"] = auth.UserFromContext(r.Context())

	// For partials, look for the named block in home.html (which defines works_grid).
	tpl, ok := s.pages["home.html"]
	if !ok {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tpl.ExecuteTemplate(w, name, data)
}

// ── Middleware ───────────────────────────────────────────────────────

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Debug("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration", time.Since(start),
		)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		gz, _ := gzip.NewWriterLevel(w, gzip.DefaultCompression)
		defer gz.Close()
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")
		next.ServeHTTP(gzipResponseWriter{Writer: gz, ResponseWriter: w}, r)
	})
}

// ── Helpers ─────────────────────────────────────────────────────────

func coverInitials(title string) string {
	words := strings.Fields(title)
	if len(words) == 0 {
		return "?"
	}
	if len(words) == 1 {
		r := []rune(words[0])
		if len(r) > 0 {
			return strings.ToUpper(string(r[0:1]))
		}
		return "?"
	}
	r1 := []rune(words[0])
	r2 := []rune(words[1])
	return strings.ToUpper(string(r1[0:1]) + string(r2[0:1]))
}

// generateNonce creates a random base64 nonce for CSP.
func generateNonce() string {
	b := make([]byte, 16)
	io.ReadFull(cryptoRand.Reader, b)
	return base64.StdEncoding.EncodeToString(b)
}

// handleHealth is a simple liveness check.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.store.(*store.SQLiteStore).DB().Ping(); err != nil {
		http.Error(w, "unhealthy", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("ok"))
}
