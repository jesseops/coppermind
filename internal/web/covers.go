package web

import (
	"encoding/json"
	"fmt"
	html "html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jesseops/coppermind/internal/auth"
	"github.com/jesseops/coppermind/internal/store"
)

// allowedCoverHosts is the whitelist of domains we'll fetch cover images from.
var allowedCoverHosts = []string{
	"covers.openlibrary.org",
}

// isAllowedCoverURL validates a URL against the cover host whitelist.
func isAllowedCoverURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if u.Scheme != "https" {
		return false
	}
	for _, h := range allowedCoverHosts {
		if u.Host == h {
			return true
		}
	}
	return false
}

// CoverSearchResult represents a single cover art candidate.
type CoverSearchResult struct {
	Source    string `json:"source"`
	Title     string `json:"title"`
	Author    string `json:"author"`
	ISBN      string `json:"isbn"`
	Year      int    `json:"year,omitempty"`
	CoverURL  string `json:"cover_url"`
	ThumbURL  string `json:"thumb_url"`
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// searchOpenLibrary queries Open Library for cover art.
func searchOpenLibrary(query, author, isbn string) ([]CoverSearchResult, error) {
	var results []CoverSearchResult

	// If we have an ISBN, try direct lookup first.
	if isbn != "" {
		r, err := fetchOpenLibraryByISBN(isbn)
		if err == nil && r != nil {
			results = append(results, *r)
		}
	}

	// Also do a text search.
	params := url.Values{}
	q := query
	if author != "" {
		q += " " + author
	}
	params.Set("q", q)
	params.Set("fields", "title,author_name,isbn,first_publish_year,cover_i")
	params.Set("limit", "8")

	resp, err := httpClient.Get("https://openlibrary.org/search.json?" + params.Encode())
	if err != nil {
		return results, fmt.Errorf("open library search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return results, fmt.Errorf("open library returned %d", resp.StatusCode)
	}

	var data struct {
		Docs []struct {
			Title      string   `json:"title"`
			AuthorName []string `json:"author_name"`
			ISBN       []string `json:"isbn"`
			CoverI     int      `json:"cover_i"`
			FirstYear  int      `json:"first_publish_year"`
		} `json:"docs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return results, fmt.Errorf("decode: %w", err)
	}

	seen := make(map[int]bool)

	for _, doc := range data.Docs {
		if doc.CoverI == 0 || seen[doc.CoverI] {
			continue
		}
		seen[doc.CoverI] = true

		authorStr := ""
		if len(doc.AuthorName) > 0 {
			authorStr = strings.Join(doc.AuthorName, ", ")
		}
		isbnStr := ""
		if len(doc.ISBN) > 0 {
			isbnStr = doc.ISBN[0]
		}

		results = append(results, CoverSearchResult{
			Source:   "Open Library",
			Title:    doc.Title,
			Author:   authorStr,
			ISBN:     isbnStr,
			Year:     doc.FirstYear,
			CoverURL: fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-L.jpg", doc.CoverI),
			ThumbURL: fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-M.jpg", doc.CoverI),
		})
	}

	return results, nil
}

func fetchOpenLibraryByISBN(isbn string) (*CoverSearchResult, error) {
	isbn = strings.ReplaceAll(isbn, "-", "")
	resp, err := httpClient.Get("https://openlibrary.org/isbn/" + isbn + ".json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ISBN lookup returned %d", resp.StatusCode)
	}

	var data struct {
		Title  string `json:"title"`
		Covers []int  `json:"covers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	if len(data.Covers) == 0 {
		return nil, nil
	}

	return &CoverSearchResult{
		Source:   "Open Library (ISBN)",
		Title:    data.Title,
		ISBN:     isbn,
		CoverURL: fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-L.jpg", data.Covers[0]),
		ThumbURL: fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-M.jpg", data.Covers[0]),
	}, nil
}

// downloadCover downloads a cover image URL and saves it to the data dir.
// Only fetches from whitelisted domains (SSRF protection).
func (s *Server) downloadCover(coverURL string, workID int64) (string, error) {
	if !isAllowedCoverURL(coverURL) {
		return "", fmt.Errorf("URL not in allowed cover sources: %s", coverURL)
	}

	resp, err := httpClient.Get(coverURL)
	if err != nil {
		return "", fmt.Errorf("download cover: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("cover download returned %d", resp.StatusCode)
	}

	// Determine extension from content type.
	ct := resp.Header.Get("Content-Type")
	ext := ".jpg"
	if strings.Contains(ct, "png") {
		ext = ".png"
	}

	coverDir := fmt.Sprintf("%s/covers", s.config.DataDir)
	if err := mkdirAll(coverDir); err != nil {
		return "", err
	}
	path := fmt.Sprintf("%s/%d%s", coverDir, workID, ext)

	f, err := createFile(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	n, err := io.Copy(f, resp.Body)
	if err != nil {
		return "", err
	}

	// Open Library returns a tiny 1x1 placeholder for missing covers.
	if n < 1000 {
		os.Remove(path)
		return "", fmt.Errorf("downloaded image too small (%d bytes) — likely a placeholder", n)
	}

	return path, nil
}

// ── Admin handlers for editions and covers ──────────────────────────

// handleAdminUpdateEdition handles inline edition field updates via HTMX.
// POST /admin/editions/{id}
func (s *Server) handleAdminUpdateEdition(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	edition, err := s.store.GetEdition(id)
	if err != nil {
		http.Error(w, "Edition not found", http.StatusNotFound)
		return
	}

	user := auth.UserFromContext(r.Context())
	if user == nil || !user.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	update := store.EditionUpdate{}
	changed := false

	if isbn := r.FormValue("isbn"); isbn != "" || r.FormValue("clear_isbn") == "1" {
		v := strings.TrimSpace(isbn)
		update.ISBN = &v
		changed = true
	}
	if pub := r.FormValue("publisher"); pub != "" || r.FormValue("clear_publisher") == "1" {
		v := strings.TrimSpace(pub)
		update.Publisher = &v
		changed = true
	}
	if notes := r.FormValue("notes"); r.Form.Has("notes") {
		v := strings.TrimSpace(notes)
		update.Notes = &v
		changed = true
	}

	if changed {
		if err := s.store.UpdateEdition(id, update); err != nil {
			slog.Error("update edition", "id", id, "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `<span class="error">Update failed</span>`)
			return
		}
	}

	// Re-fetch and return the updated metadata snippet.
	edition, _ = s.store.GetEdition(id)
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<span style="color:var(--color-success);">✓ Saved</span>`)
	_ = edition
}

// handleAdminCoverSearch searches for cover art online.
// GET /admin/works/{id}/covers/search
func (s *Server) handleAdminCoverSearch(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	work, err := s.store.GetWork(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		query = work.Title
	}
	author := work.PrimaryAuthor()

	// Also check editions for ISBNs.
	editions, _ := s.store.ListEditions(id)
	isbn := ""
	for _, ed := range editions {
		if ed.ISBN != "" {
			isbn = ed.ISBN
			break
		}
	}

	results, err := searchOpenLibrary(query, author, isbn)
	if err != nil {
		slog.Warn("cover search failed", "err", err)
	}

	w.Header().Set("Content-Type", "text/html")
	if len(results) == 0 {
		fmt.Fprintf(w, `<p style="color:var(--color-text-muted);padding:0.5rem;">No covers found. Try a different search query.</p>`)
		return
	}

	fmt.Fprintf(w, `<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(120px,1fr));gap:1rem;padding:0.5rem;">`)
	for _, res := range results {
		// Skip results with non-whitelisted URLs.
		if !isAllowedCoverURL(res.ThumbURL) || !isAllowedCoverURL(res.CoverURL) {
			continue
		}
		title := html.EscapeString(res.Title)
		fmt.Fprintf(w, `<div style="text-align:center;">`)
		fmt.Fprintf(w, `<img src="%s" alt="%s" style="max-width:120px;max-height:180px;border-radius:0.25rem;border:1px solid var(--color-border);cursor:pointer;"
			loading="lazy">`, res.ThumbURL, title)
		fmt.Fprintf(w, `<div style="font-size:0.75rem;margin-top:0.25rem;color:var(--color-text-muted);line-height:1.2;">%s`, title)
		if res.Year > 0 {
			fmt.Fprintf(w, ` (%d)`, res.Year)
		}
		fmt.Fprintf(w, `</div>`)
		fmt.Fprintf(w, `<form hx-post="/admin/works/%d/covers/apply" hx-target="#cover-status" hx-swap="innerHTML" style="margin-top:0.25rem;">`, id)
		fmt.Fprintf(w, `<input type="hidden" name="url" value="%s">`, html.EscapeString(res.CoverURL))
		fmt.Fprintf(w, `<button type="submit" class="btn btn-primary btn-sm">Use</button>`)
		fmt.Fprintf(w, `</form>`)
		fmt.Fprintf(w, `</div>`)
	}
	fmt.Fprintf(w, `</div>`)
}

// handleAdminCoverApply downloads and applies a cover to a work.
// POST /admin/works/{id}/covers/apply
func (s *Server) handleAdminCoverApply(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	coverURL := r.FormValue("url")
	editionID := r.FormValue("edition_id")

	var coverPath string
	var err error

	if editionID != "" {
		// Use cover from an existing edition.
		eid, _ := strconv.ParseInt(editionID, 10, 64)
		edition, e := s.store.GetEdition(eid)
		if e != nil || !edition.HasCover() {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `<span class="error">Edition cover not found</span>`)
			return
		}
		coverPath = edition.CoverPath
	} else if coverURL != "" {
		// Download from URL.
		coverPath, err = s.downloadCover(coverURL, id)
		if err != nil {
			slog.Error("download cover", "url", coverURL, "err", err)
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, `<span class="error">Failed to download cover: %s</span>`, err.Error())
			return
		}
	} else {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `<span class="error">No cover source specified</span>`)
		return
	}

	if err := s.store.UpdateWork(id, store.WorkUpdate{CoverPath: &coverPath}); err != nil {
		slog.Error("update work cover", "id", id, "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `<span class="error">Failed to save</span>`)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("HX-Trigger", "coverUpdated")
	fmt.Fprintf(w, `<span class="text-success">✓ Cover updated</span>`)
}

// File helpers — thin wrappers to enable testing.
var mkdirAll = func(path string) error {
	return os.MkdirAll(path, 0o755)
}
var createFile = func(path string) (io.WriteCloser, error) {
	return os.Create(path)
}
