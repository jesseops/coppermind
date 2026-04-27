package web

import (
	"encoding/json"
	"fmt"
	html "html"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jesseops/coppermind/internal/store"
)

// MetadataResult represents a search result from Open Library.
type MetadataResult struct {
	Title       string   `json:"title"`
	Authors     []string `json:"authors"`
	Year        int      `json:"first_publish_year"`
	ISBN        []string `json:"isbn"`
	Publishers  []string `json:"publishers"`
	Languages   []string `json:"languages"`
	Subjects    []string `json:"subjects"`
	Pages       int      `json:"pages"`
	CoverID     int      `json:"cover_id"`
	Description string   `json:"description"`
}

// searchMetadata queries Open Library for book metadata.
func searchMetadata(query, author string) ([]MetadataResult, error) {
	params := url.Values{}
	q := query
	if author != "" {
		q += " " + author
	}
	params.Set("q", q)
	params.Set("fields", "title,author_name,isbn,first_publish_year,cover_i,publisher,language,number_of_pages_median,subject")
	params.Set("limit", "8")

	resp, err := httpClient.Get("https://openlibrary.org/search.json?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("open library search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("open library returned %d", resp.StatusCode)
	}

	var data struct {
		Docs []struct {
			Title     string   `json:"title"`
			Authors   []string `json:"author_name"`
			ISBN      []string `json:"isbn"`
			Year      int      `json:"first_publish_year"`
			CoverI    int      `json:"cover_i"`
			Publisher []string `json:"publisher"`
			Language  []string `json:"language"`
			Pages     int      `json:"number_of_pages_median"`
			Subject   []string `json:"subject"`
		} `json:"docs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []MetadataResult
	for _, doc := range data.Docs {
		r := MetadataResult{
			Title:   doc.Title,
			Authors: doc.Authors,
			Year:    doc.Year,
			CoverID: doc.CoverI,
			Pages:   doc.Pages,
		}
		// Keep first 3 of each list.
		if len(doc.ISBN) > 3 {
			r.ISBN = doc.ISBN[:3]
		} else {
			r.ISBN = doc.ISBN
		}
		if len(doc.Publisher) > 3 {
			r.Publishers = doc.Publisher[:3]
		} else {
			r.Publishers = doc.Publisher
		}
		if len(doc.Language) > 3 {
			r.Languages = doc.Language[:3]
		} else {
			r.Languages = doc.Language
		}
		// Subjects: first 5, skip very long ones.
		for _, s := range doc.Subject {
			if len(s) < 50 && len(r.Subjects) < 5 {
				r.Subjects = append(r.Subjects, s)
			}
		}
		results = append(results, r)
	}
	return results, nil
}

// handleAdminMetadataSearch searches Open Library for metadata.
// GET /admin/works/{id}/metadata/search
func (s *Server) handleAdminMetadataSearch(w http.ResponseWriter, r *http.Request) {
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

	results, err := searchMetadata(query, author)
	if err != nil {
		slog.Warn("metadata search failed", "err", err)
	}

	w.Header().Set("Content-Type", "text/html")
	if len(results) == 0 {
		fmt.Fprintf(w, `<p class="text-text-muted p-2">No results found. Try a different search query.</p>`)
		return
	}

	for i, res := range results {
		title := html.EscapeString(res.Title)
		authors := html.EscapeString(strings.Join(res.Authors, ", "))

		fmt.Fprintf(w, `<div class="border border-border rounded-md p-3 mb-2 flex gap-3 items-start">`)

		// Cover thumbnail
		if res.CoverID > 0 {
			fmt.Fprintf(w, `<img src="https://covers.openlibrary.org/b/id/%d-S.jpg" alt="" class="w-12 h-16 object-cover rounded shrink-0" loading="lazy">`, res.CoverID)
		} else {
			fmt.Fprintf(w, `<div class="w-12 h-16 bg-surface rounded shrink-0 flex items-center justify-center text-text-muted text-xs">?</div>`)
		}

		fmt.Fprintf(w, `<div class="flex-1 min-w-0">`)
		fmt.Fprintf(w, `<div class="font-semibold text-sm">%s</div>`, title)
		if authors != "" {
			fmt.Fprintf(w, `<div class="text-xs text-text-muted">%s</div>`, authors)
		}

		// Details line
		var details []string
		if res.Year > 0 {
			details = append(details, fmt.Sprintf("%d", res.Year))
		}
		if len(res.Publishers) > 0 {
			details = append(details, html.EscapeString(res.Publishers[0]))
		}
		if res.Pages > 0 {
			details = append(details, fmt.Sprintf("%d pages", res.Pages))
		}
		if len(res.Languages) > 0 {
			details = append(details, html.EscapeString(strings.Join(res.Languages, "/")))
		}
		if len(details) > 0 {
			fmt.Fprintf(w, `<div class="text-xs text-text-muted mt-0.5">%s</div>`, strings.Join(details, " · "))
		}

		// ISBN
		if len(res.ISBN) > 0 {
			fmt.Fprintf(w, `<div class="text-xs text-text-muted mt-0.5">ISBN: %s</div>`, html.EscapeString(strings.Join(res.ISBN, ", ")))
		}

		// Subjects
		if len(res.Subjects) > 0 {
			fmt.Fprintf(w, `<div class="flex gap-1 flex-wrap mt-1">`)
			for _, subj := range res.Subjects {
				fmt.Fprintf(w, `<span class="badge bg-surface text-xs">%s</span>`, html.EscapeString(subj))
			}
			fmt.Fprintf(w, `</div>`)
		}

		fmt.Fprintf(w, `</div>`) // flex-1

		// Apply button
		fmt.Fprintf(w, `<form hx-post="/admin/works/%d/metadata/apply" hx-target="#metadata-status" hx-swap="innerHTML" class="shrink-0">`, id)
		fmt.Fprintf(w, `<input type="hidden" name="idx" value="%d">`, i)
		// Encode the metadata as hidden fields.
		fmt.Fprintf(w, `<input type="hidden" name="title" value="%s">`, html.EscapeString(res.Title))
		if len(res.Authors) > 0 {
			fmt.Fprintf(w, `<input type="hidden" name="author" value="%s">`, html.EscapeString(res.Authors[0]))
		}
		if res.Year > 0 {
			fmt.Fprintf(w, `<input type="hidden" name="year" value="%d">`, res.Year)
		}
		if len(res.Languages) > 0 {
			fmt.Fprintf(w, `<input type="hidden" name="language" value="%s">`, html.EscapeString(res.Languages[0]))
		}
		if len(res.ISBN) > 0 {
			fmt.Fprintf(w, `<input type="hidden" name="isbn" value="%s">`, html.EscapeString(res.ISBN[0]))
		}
		if len(res.Publishers) > 0 {
			fmt.Fprintf(w, `<input type="hidden" name="publisher" value="%s">`, html.EscapeString(res.Publishers[0]))
		}
		for _, subj := range res.Subjects {
			fmt.Fprintf(w, `<input type="hidden" name="subjects" value="%s">`, html.EscapeString(subj))
		}
		if res.CoverID > 0 {
			fmt.Fprintf(w, `<input type="hidden" name="cover_url" value="https://covers.openlibrary.org/b/id/%d-L.jpg">`, res.CoverID)
		}
		fmt.Fprintf(w, `<button type="submit" class="btn btn-primary btn-sm whitespace-nowrap">Apply</button>`)
		fmt.Fprintf(w, `</form>`)

		fmt.Fprintf(w, `</div>`) // border card
	}
}

// handleAdminMetadataApply applies metadata from Open Library to a work.
// POST /admin/works/{id}/metadata/apply
func (s *Server) handleAdminMetadataApply(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	work, err := s.store.GetWork(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Build work update from form fields.
	update := store.WorkUpdate{}
	var applied []string

	if title := strings.TrimSpace(r.FormValue("title")); title != "" && title != work.Title {
		update.Title = &title
		applied = append(applied, "title")
	}
	if yearStr := r.FormValue("year"); yearStr != "" {
		if year, err := strconv.Atoi(yearStr); err == nil && year > 0 && work.FirstPublished == 0 {
			update.FirstPublished = &year
			applied = append(applied, "year")
		}
	}
	if lang := strings.TrimSpace(r.FormValue("language")); lang != "" && work.Language == "" {
		update.Language = &lang
		applied = append(applied, "language")
	}

	// Apply cover if work doesn't have one.
	coverURL := r.FormValue("cover_url")
	if coverURL != "" && !work.HasCover() {
		path, err := s.downloadCover(coverURL, id)
		if err != nil {
			slog.Warn("metadata cover download failed", "err", err)
		} else {
			update.CoverPath = &path
			applied = append(applied, "cover")
		}
	}

	// Update work.
	if err := s.store.UpdateWork(id, update); err != nil {
		slog.Error("apply metadata", "work_id", id, "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `<span class="error">Failed to apply metadata</span>`)
		return
	}

	// Add subjects as tags.
	if subjects, ok := r.Form["subjects"]; ok {
		for _, subj := range subjects {
			subj = strings.TrimSpace(subj)
			if subj != "" {
				s.store.AddTag(id, strings.ToLower(subj))
			}
		}
		if len(subjects) > 0 {
			applied = append(applied, fmt.Sprintf("%d tags", len(subjects)))
		}
	}

	// Update edition ISBN/publisher if provided and edition doesn't have them.
	isbn := strings.TrimSpace(r.FormValue("isbn"))
	publisher := strings.TrimSpace(r.FormValue("publisher"))
	if isbn != "" || publisher != "" {
		editions, _ := s.store.ListEditions(id)
		for _, ed := range editions {
			eu := store.EditionUpdate{}
			changed := false
			if isbn != "" && ed.ISBN == "" {
				eu.ISBN = &isbn
				changed = true
			}
			if publisher != "" && ed.Publisher == "" {
				eu.Publisher = &publisher
				changed = true
			}
			if changed {
				s.store.UpdateEdition(ed.ID, eu)
			}
		}
		if isbn != "" {
			applied = append(applied, "isbn")
		}
		if publisher != "" {
			applied = append(applied, "publisher")
		}
	}

	w.Header().Set("Content-Type", "text/html")
	if len(applied) == 0 {
		fmt.Fprintf(w, `<span class="text-text-muted">No new metadata to apply (fields already populated)</span>`)
	} else {
		w.Header().Set("HX-Trigger", "metadataUpdated")
		fmt.Fprintf(w, `<span class="text-success">✓ Applied: %s — <a href="/works/%d">refresh to see</a></span>`, strings.Join(applied, ", "), id)
	}
}
