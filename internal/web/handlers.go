package web

import (
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jesseops/coppermind/internal/auth"
	"github.com/jesseops/coppermind/internal/domain"
	"github.com/jesseops/coppermind/internal/importer"
	"github.com/jesseops/coppermind/internal/store"
)

// safePath validates that a file path is within the server's data directory.
// Returns the cleaned absolute path, or an error if it escapes the data dir.
func (s *Server) safePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path")
	}

	// Resolve both to absolute paths for reliable comparison.
	absData, err := filepath.Abs(s.config.DataDir)
	if err != nil {
		return "", fmt.Errorf("resolve data dir: %w", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	if !strings.HasPrefix(absPath, absData+string(filepath.Separator)) && absPath != absData {
		return "", fmt.Errorf("path %q is outside data directory", path)
	}
	return absPath, nil
}

// ── Public handlers ─────────────────────────────────────────────────

func (s *Server) handleWorkDetail(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	work, err := s.store.GetWork(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	editions, err := s.store.ListEditions(id)
	if err != nil {
		slog.Error("list editions", "work_id", id, "err", err)
	}
	tags, _ := s.store.ListTags(id)

	// Collect editions that have their own cover (for admin cover picker).
	var editionsWithCovers []domain.Edition
	for _, ed := range editions {
		if ed.HasCover() {
			editionsWithCovers = append(editionsWithCovers, ed)
		}
	}

	s.render(w, r, "work.html", templateData{
		"Work":               work,
		"Editions":           editions,
		"Tags":               tags,
		"EditionsWithCovers": editionsWithCovers,
	})
}

func (s *Server) handleCover(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	work, err := s.store.GetWork(id)
	if err != nil || work.CoverPath == "" {
		http.NotFound(w, r)
		return
	}
	safe, err := s.safePath(work.CoverPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// Validate the file is actually an image before serving.
	f, err := os.Open(safe)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	header := make([]byte, 12)
	n, _ := f.Read(header)
	f.Close()
	if n < 4 || !isImageHeader(header[:n]) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, safe)
}

func (s *Server) handleEditionCover(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	edition, err := s.store.GetEdition(id)
	if err != nil || edition.CoverPath == "" {
		http.NotFound(w, r)
		return
	}
	safe, err := s.safePath(edition.CoverPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, safe)
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	edition, err := s.store.GetEdition(id)
	if err != nil || edition.FilePath == "" {
		http.NotFound(w, r)
		return
	}
	safe, err := s.safePath(edition.FilePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(safe)))
	http.ServeFile(w, r, safe)
}

// ── Auth handlers ───────────────────────────────────────────────────

func (s *Server) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "login.html", nil)
}

func (s *Server) handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	// Rate limit login attempts by IP.
	ip := clientIP(r)
	if !s.loginLimiter.allow(ip) {
		w.Header().Set("Retry-After", "60")
		http.Error(w, "Too many login attempts. Please try again later.", http.StatusTooManyRequests)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")
	rememberMe := r.FormValue("remember_me") == "on"

	user, err := s.store.GetUserByUsername(username)
	if err != nil {
		s.render(w, r, "login.html", templateData{"Error": "Invalid credentials"})
		return
	}
	if err := auth.CheckPassword(user.PasswordHash, password); err != nil {
		s.render(w, r, "login.html", templateData{"Error": "Invalid credentials"})
		return
	}

	token, expiry := auth.CreateSession(user.ID, rememberMe, s.secret)
	auth.SetSessionCookie(w, r, token, expiry)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	auth.ClearSessionCookie(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) handleSetupForm(w http.ResponseWriter, r *http.Request) {
	count, _ := s.store.CountUsers()
	if count > 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, r, "setup.html", nil)
}

func (s *Server) handleSetupSubmit(w http.ResponseWriter, r *http.Request) {
	count, _ := s.store.CountUsers()
	if count > 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")
	libraryName := r.FormValue("library_name")

	if username == "" || password == "" {
		s.render(w, r, "setup.html", templateData{"Error": "Username and password are required"})
		return
	}
	if libraryName == "" {
		libraryName = "My Library"
	}

	hash, _ := auth.HashPassword(password)
	user, err := s.store.CreateUser(username, username, hash, domain.RoleAdmin)
	if err != nil {
		s.render(w, r, "setup.html", templateData{"Error": err.Error()})
		return
	}
	s.store.CreateLibrary(libraryName)

	token, expiry := auth.CreateSession(user.ID, false, s.secret)
	auth.SetSessionCookie(w, r, token, expiry)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ── Reader/Player stubs ─────────────────────────────────────────────

func (s *Server) handleReader(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	edition, err := s.store.GetEdition(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// PDF: redirect to download (no in-browser reader).
	if edition.Format == "pdf" {
		http.Redirect(w, r, fmt.Sprintf("/download/%d", id), http.StatusSeeOther)
		return
	}

	safe, err := s.safePath(edition.FilePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	chapter, _ := strconv.Atoi(r.URL.Query().Get("chapter"))

	var chapterHTML string
	var chapterPath string
	var chapters []importer.ReaderChapter

	switch edition.Format {
	case "mobi":
		// MOBI: extract full text as a single "chapter".
		text, err := importer.ExtractMobiTextPreview(safe, 0) // 0 = no limit
		if err != nil {
			http.Error(w, "Failed to read MOBI", http.StatusInternalServerError)
			return
		}
		// Convert plain text to HTML paragraphs.
		var sb strings.Builder
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				sb.WriteString("<p>")
				sb.WriteString(template.HTMLEscapeString(line))
				sb.WriteString("</p>\n")
			}
		}
		chapterHTML = sb.String()
		chapters = []importer.ReaderChapter{{Index: 0, Title: edition.Format}}
		chapter = 0
	default:
		// EPUB and other formats.
		var err error
		chapterHTML, chapterPath, chapters, err = importer.ExtractEpubChapter(safe, chapter)
		if err != nil {
			http.Error(w, "Failed to read chapter", http.StatusInternalServerError)
			return
		}
		// Rewrite relative asset URLs in chapter HTML to point to /epub-asset/{id}/...
		chapterHTML = rewriteEpubAssetURLs(chapterHTML, id, chapterPath)
	}

	// Auto-set reading state when opening.
	user := auth.UserFromContext(r.Context())
	if user != nil {
		totalChapters := len(chapters)
		progress := 0.0
		if totalChapters > 0 {
			progress = float64(chapter) / float64(totalChapters)
		}
		s.store.SaveReadingState(&domain.ReadingState{
			UserID:       user.ID,
			EditionID:    id,
			Status:       domain.ReadingStatusReading,
			Progress:     progress,
			ChapterIndex: chapter,
		})
	}

	work, _ := s.store.GetWork(edition.WorkID)
	s.render(w, r, "reader.html", templateData{
		"Work":          work,
		"Edition":       edition,
		"ChapterHTML":   template.HTML(chapterHTML),
		"ChapterPath":   chapterPath,
		"Chapters":      chapters,
		"CurrentCh":     chapter,
		"TotalChapters": len(chapters),
	})
}

func (s *Server) handlePlayer(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	edition, err := s.store.GetEdition(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	tracks, err := s.store.ListTracks(id)
	if err != nil {
		slog.Error("list tracks", "edition_id", id, "err", err)
	}

	// Auto-set reading state when opening player.
	user := auth.UserFromContext(r.Context())
	if user != nil {
		s.store.SaveReadingState(&domain.ReadingState{
			UserID:    user.ID,
			EditionID: id,
			Status:    domain.ReadingStatusReading,
		})
	}

	work, _ := s.store.GetWork(edition.WorkID)
	s.render(w, r, "player.html", templateData{
		"Work":    work,
		"Edition": edition,
		"Tracks":  tracks,
	})
}

func (s *Server) handleEpubAsset(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	edition, err := s.store.GetEdition(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	safe, err := s.safePath(edition.FilePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	assetPath := chi.URLParam(r, "*")
	data, err := importer.ExtractEpubAsset(safe, assetPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	// Set Content-Type based on file extension.
	switch strings.ToLower(path.Ext(assetPath)) {
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".gif":
		w.Header().Set("Content-Type", "image/gif")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".webp":
		w.Header().Set("Content-Type", "image/webp")
	case ".woff", ".woff2":
		w.Header().Set("Content-Type", "font/woff2")
	case ".ttf":
		w.Header().Set("Content-Type", "font/ttf")
	case ".otf":
		w.Header().Set("Content-Type", "font/otf")
	case ".xhtml", ".html", ".htm":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case ".xml":
		w.Header().Set("Content-Type", "application/xml")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Write(data)
}

func (s *Server) handleAudioTrack(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	track, err := s.store.GetTrack(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	safe, err := s.safePath(track.FilePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, safe)
}

// ── User handlers ───────────────────────────────────────────────────

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	s.render(w, r, "profile.html", templateData{"User": user})
}

func (s *Server) handleCurrentlyReading(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	states, err := s.store.ListReadingStates(user.ID, domain.ReadingStatusReading)
	if err != nil {
		slog.Error("list reading states", "user_id", user.ID, "err", err)
	}
	s.render(w, r, "reading.html", templateData{"States": states})
}

func (s *Server) handleSaveReadingState(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	editionID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	progress, _ := strconv.ParseFloat(r.FormValue("progress"), 64)
	chapterIndex, _ := strconv.Atoi(r.FormValue("chapter_index"))

	status := r.FormValue("status")
	if status == "" {
		status = domain.ReadingStatusReading
	}

	s.store.SaveReadingState(&domain.ReadingState{
		UserID:       user.ID,
		EditionID:    editionID,
		Status:       status,
		Progress:     progress,
		ChapterIndex: chapterIndex,
	})
	w.WriteHeader(http.StatusNoContent)
}

// ── Admin handlers ──────────────────────────────────────────────────

func (s *Server) handleAdminImportForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "admin_import.html", nil)
}

func (s *Server) handleAdminImportSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(100 << 20) // 100MB
	files := r.MultipartForm.File["files"]
	libs, _ := s.store.ListLibraries()
	if len(libs) == 0 {
		s.render(w, r, "admin_import.html", templateData{"Error": "No library found"})
		return
	}

	var results []string
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			results = append(results, fmt.Sprintf("Error opening %s: %v", fh.Filename, err))
			continue
		}
		// Write to temp file.
		tmpFile, _ := os.CreateTemp("", "coppermind-import-*-"+fh.Filename)
		io.Copy(tmpFile, f)
		tmpFile.Close()
		f.Close()

		result, err := s.importer.Import(importer.ImportInput{
			SourcePath: tmpFile.Name(),
			LibraryID:  libs[0].ID,
		})
		os.Remove(tmpFile.Name())

		if err != nil {
			results = append(results, fmt.Sprintf("Error: %s - %v", fh.Filename, err))
		} else if result.Skipped {
			results = append(results, fmt.Sprintf("Skipped: %s - %s", fh.Filename, result.SkipReason))
		} else {
			results = append(results, fmt.Sprintf("Imported: %s → %s", fh.Filename, result.Work.Title))
		}
	}
	s.render(w, r, "admin_import.html", templateData{"Results": results})
}

func (s *Server) handleAdminEditForm(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	work, err := s.store.GetWork(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	editions, err := s.store.ListEditions(id)
	if err != nil {
		slog.Error("list editions for edit", "work_id", id, "err", err)
	}
	tags, _ := s.store.ListTags(id)
	s.render(w, r, "admin_edit.html", templateData{
		"Work":     work,
		"Editions": editions,
		"Tags":     tags,
	})
}

func (s *Server) handleAdminEditSubmit(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	title := r.FormValue("title")
	desc := r.FormValue("description")

	update := store.WorkUpdate{}
	if title != "" {
		update.Title = &title
	}
	if desc != "" {
		update.Description = &desc
	}
	if err := s.store.UpdateWork(id, update); err != nil {
		slog.Error("update work", "id", id, "err", err)
	}
	http.Redirect(w, r, fmt.Sprintf("/works/%d", id), http.StatusSeeOther)
}

func (s *Server) handleAdminDeleteWork(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := s.store.DeleteWork(id); err != nil {
		slog.Error("delete work", "id", id, "err", err)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleAdminReparse(w http.ResponseWriter, r *http.Request) {
	workID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	work, err := s.store.GetWork(workID)
	if err != nil {
		fmt.Fprintf(w, `<span class="text-error text-sm">Work not found</span>`)
		return
	}

	editions, err := s.store.ListEditions(workID)
	if err != nil || len(editions) == 0 {
		fmt.Fprintf(w, `<span class="text-error text-sm">No editions found</span>`)
		return
	}

	ed := editions[0]
	if ed.FilePath == "" {
		fmt.Fprintf(w, `<span class="text-error text-sm">No file path for edition</span>`)
		return
	}

	ext := strings.ToLower(filepath.Ext(ed.FilePath))
	var meta *importer.Extracted
	switch ext {
	case ".epub":
		meta, err = importer.ExtractEpubMetadata(ed.FilePath)
	case ".mobi":
		meta, err = importer.ExtractMobiMetadata(ed.FilePath)
	default:
		fmt.Fprintf(w, `<span class="text-error text-sm">Unsupported format: %s</span>`, ext)
		return
	}
	if err != nil {
		fmt.Fprintf(w, `<span class="text-error text-sm">Parse error: %s</span>`, template.HTMLEscapeString(err.Error()))
		return
	}

	var changes []string

	// Update title.
	if meta.Title != "" && meta.Title != work.Title {
		changes = append(changes, fmt.Sprintf("title: %q → %q", work.Title, meta.Title))
		s.store.UpdateWork(workID, store.WorkUpdate{Title: &meta.Title})
	}

	// Update authors — compare by sort name to avoid format-only differences.
	if len(meta.Authors) > 0 && reparseAuthorsChanged(work, meta.Authors) {
		oldNames := make([]string, len(work.Authors))
		for i, a := range work.Authors {
			oldNames[i] = a.AuthorName
		}
		changes = append(changes, fmt.Sprintf("authors: %q → %q",
			strings.Join(oldNames, ", "), strings.Join(meta.Authors, ", ")))
		// Unlink old authors, link new.
		oldAuthors, _ := s.store.GetWorkAuthors(workID)
		for _, oa := range oldAuthors {
			s.store.UnlinkWorkAuthor(workID, oa.AuthorID, oa.Role)
		}
		for _, name := range meta.Authors {
			sortName := domain.GenerateSortName(name)
			existing, err := s.store.FindAuthorBySortName(sortName)
			if err == nil && existing != nil {
				s.store.LinkWorkAuthor(workID, existing.ID, domain.RoleAuthorOf)
			} else {
				created, err := s.store.CreateAuthor(name, sortName)
				if err == nil {
					s.store.LinkWorkAuthor(workID, created.ID, domain.RoleAuthorOf)
				}
			}
		}
	}

	// Update description if empty.
	if meta.Description != "" && work.Description == "" {
		changes = append(changes, "added description")
		s.store.UpdateWork(workID, store.WorkUpdate{Description: &meta.Description})
	}

	// Update language if empty.
	if meta.Language != "" && work.Language == "" {
		changes = append(changes, "added language")
		s.store.UpdateWork(workID, store.WorkUpdate{Language: &meta.Language})
	}

	if len(changes) == 0 {
		fmt.Fprintf(w, `<span class="text-text-muted text-sm">✓ No changes needed — metadata already matches file.</span>`)
		return
	}

	w.Header().Set("HX-Trigger", "metadataUpdated")
	fmt.Fprintf(w, `<span class="text-sm">✓ Updated: %s</span>`, template.HTMLEscapeString(strings.Join(changes, "; ")))
}

// reparseAuthorsChanged compares authors by sort name to avoid flagging
// format-only differences like "First Last" vs "Last, First".
func reparseAuthorsChanged(work *domain.Work, newAuthors []string) bool {
	oldSet := make(map[string]bool)
	for _, a := range work.Authors {
		oldSet[domain.GenerateSortName(a.AuthorName)] = true
	}
	newSet := make(map[string]bool)
	for _, name := range newAuthors {
		newSet[domain.GenerateSortName(name)] = true
	}
	if len(oldSet) != len(newSet) {
		return true
	}
	for k := range newSet {
		if !oldSet[k] {
			return true
		}
	}
	return false
}

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers()
	if err != nil {
		slog.Error("list users", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	s.render(w, r, "admin_users.html", templateData{"Users": users})
}

func (s *Server) handleAdminCreateUser(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	role := r.FormValue("role")
	if role == "" {
		role = domain.RoleViewer
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		slog.Error("hash password", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if _, err := s.store.CreateUser(username, username, hash, role); err != nil {
		slog.Error("create user", "username", username, "err", err)
	}
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (s *Server) handleAdminDuplicates(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "admin_duplicates.html", nil)
}

func (s *Server) handleAdminLibrarian(w http.ResponseWriter, r *http.Request) {
	libs, err := s.store.ListLibraries()
	if err != nil || len(libs) == 0 {
		http.Error(w, "No libraries", http.StatusInternalServerError)
		return
	}

	issue := r.URL.Query().Get("issue")
	q := r.URL.Query().Get("q")
	format := r.URL.Query().Get("format")

	showHidden := r.URL.Query().Get("show_hidden") == "1"

	filter := store.WorkFilter{
		LibraryID: libs[0].ID,
		Query:     q,
		Format:    format,
		SortBy:    "title",
	}

	switch issue {
	case "no-cover":
		filter.MissingCover = true
		filter.IncludeHidden = showHidden
	case "no-author":
		filter.MissingAuthor = true
		filter.IncludeHidden = showHidden
	case "no-description":
		filter.MissingDesc = true
		filter.IncludeHidden = showHidden
	case "hidden":
		filter.OnlyHidden = true
	default:
		filter.IncludeHidden = showHidden
	}

	works, total, _ := s.store.ListWorks(filter)

	// If "all" mode (no specific issue), filter to works that have at least one issue.
	if issue == "" {
		var filtered []domain.Work
		for _, w := range works {
			if !w.HasCover() || len(w.Authors) == 0 || w.Description == "" {
				filtered = append(filtered, w)
			}
		}
		works = filtered
		total = len(filtered)
	}

	// Count per issue for the sidebar (respecting format filter).
	allWorks, _, _ := s.store.ListWorks(store.WorkFilter{LibraryID: libs[0].ID, IncludeHidden: true, Format: format})
	counts := map[string]int{}
	for _, w := range allWorks {
		if !w.HasCover() {
			counts["no-cover"]++
		}
		if len(w.Authors) == 0 {
			counts["no-author"]++
		}
		if w.Description == "" {
			counts["no-description"]++
		}
		if w.Hidden {
			counts["hidden"]++
		}
	}

	data := templateData{
		"Works":      works,
		"Total":      total,
		"Issue":      issue,
		"Query":      q,
		"Format":     format,
		"ShowHidden": showHidden,
		"Counts":     counts,
	}

	s.render(w, r, "admin_librarian.html", data)
}

func (s *Server) handleAdminToggleHide(w http.ResponseWriter, r *http.Request) {
	workID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	work, err := s.store.GetWork(workID)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	newHidden := !work.Hidden
	s.store.UpdateWork(workID, store.WorkUpdate{Hidden: &newHidden})

	// If HTMX request, trigger a page refresh.
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Refresh", "true")
		return
	}
	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func (s *Server) handleAdminLibrarianBulk(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	action := r.FormValue("action")
	ids := r.Form["work_id"]

	if len(ids) == 0 {
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	var workIDs []int64
	for _, raw := range ids {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err == nil && id > 0 {
			workIDs = append(workIDs, id)
		}
	}

	switch action {
	case "reparse":
		for _, workID := range workIDs {
			s.reparseWork(workID)
		}
	case "hide":
		hidden := true
		for _, workID := range workIDs {
			s.store.UpdateWork(workID, store.WorkUpdate{Hidden: &hidden})
		}
	case "unhide":
		hidden := false
		for _, workID := range workIDs {
			s.store.UpdateWork(workID, store.WorkUpdate{Hidden: &hidden})
		}
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Refresh", "true")
		return
	}
	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

// reparseWork re-extracts metadata from the first edition's source file
// and updates the work's title, authors, description, and language.
func (s *Server) reparseWork(workID int64) {
	work, err := s.store.GetWork(workID)
	if err != nil {
		return
	}
	editions, err := s.store.ListEditions(workID)
	if err != nil || len(editions) == 0 {
		return
	}
	ed := editions[0]
	if ed.FilePath == "" {
		return
	}

	ext := strings.ToLower(filepath.Ext(ed.FilePath))
	var meta *importer.Extracted
	switch ext {
	case ".epub":
		meta, err = importer.ExtractEpubMetadata(ed.FilePath)
	case ".mobi":
		meta, err = importer.ExtractMobiMetadata(ed.FilePath)
	default:
		return
	}
	if err != nil || meta == nil {
		return
	}

	if meta.Title != "" && meta.Title != work.Title {
		s.store.UpdateWork(workID, store.WorkUpdate{Title: &meta.Title})
	}
	if len(meta.Authors) > 0 && reparseAuthorsChanged(work, meta.Authors) {
		oldAuthors, _ := s.store.GetWorkAuthors(workID)
		for _, oa := range oldAuthors {
			s.store.UnlinkWorkAuthor(workID, oa.AuthorID, oa.Role)
		}
		for _, name := range meta.Authors {
			sortName := domain.GenerateSortName(name)
			existing, _ := s.store.FindAuthorBySortName(sortName)
			if existing != nil {
				s.store.LinkWorkAuthor(workID, existing.ID, domain.RoleAuthorOf)
			} else {
				created, err := s.store.CreateAuthor(name, sortName)
				if err == nil {
					s.store.LinkWorkAuthor(workID, created.ID, domain.RoleAuthorOf)
				}
			}
		}
	}
	if meta.Description != "" && work.Description == "" {
		s.store.UpdateWork(workID, store.WorkUpdate{Description: &meta.Description})
	}
	if meta.Language != "" && work.Language == "" {
		s.store.UpdateWork(workID, store.WorkUpdate{Language: &meta.Language})
	}
}

// rewriteEpubAssetURLs rewrites relative src/href attributes in EPUB chapter
// HTML to point to the /epub-asset/{editionID}/... endpoint.
func rewriteEpubAssetURLs(html string, editionID int64, chapterPath string) string {
	base := path.Dir(chapterPath)
	prefix := fmt.Sprintf("/epub-asset/%d/", editionID)

	rewriteURL := func(url string) string {
		url = strings.TrimSpace(url)
		if url == "" || strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") ||
			strings.HasPrefix(url, "data:") || strings.HasPrefix(url, "/") || strings.HasPrefix(url, "#") {
			return url
		}
		// Strip fragment.
		frag := ""
		if i := strings.Index(url, "#"); i >= 0 {
			frag = url[i:]
			url = url[:i]
		}
		resolved := path.Clean(path.Join(base, url))
		return prefix + resolved + frag
	}

	// Rewrite all src="..." attributes (images, etc.).
	result := rewriteAttrValues(html, `src="`, rewriteURL)
	result = rewriteAttrValues(result, `src='`, rewriteURL)
	// Rewrite href="..." only on <link> and <image> tags (CSS, SVG refs), not <a> tags.
	result = rewriteLinkHrefs(result, rewriteURL)
	return result
}

// rewriteAttrValues finds all occurrences of prefix + value + quote and rewrites value.
func rewriteAttrValues(html, prefix string, fn func(string) string) string {
	quote := prefix[len(prefix)-1:]
	var b strings.Builder
	i := 0
	for i < len(html) {
		idx := strings.Index(html[i:], prefix)
		if idx == -1 {
			b.WriteString(html[i:])
			break
		}
		b.WriteString(html[i : i+idx])
		b.WriteString(prefix[:len(prefix)-1]) // write attr= without quote
		valStart := i + idx + len(prefix)
		valEnd := strings.Index(html[valStart:], quote)
		if valEnd == -1 {
			b.WriteString(html[i+idx+len(prefix)-1:])
			break
		}
		value := html[valStart : valStart+valEnd]
		b.WriteString(quote)
		b.WriteString(fn(value))
		b.WriteString(quote)
		i = valStart + valEnd + 1
	}
	return b.String()
}

// isImageHeader checks magic bytes to verify data starts with a known image format.
func isImageHeader(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	if data[0] == 0xFF && data[1] == 0xD8 {
		return true // JPEG
	}
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return true // PNG
	}
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 {
		return true // GIF
	}
	if len(data) >= 12 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
		data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
		return true // WebP
	}
	return false
}

// rewriteLinkHrefs rewrites href attributes only on <link> elements (for CSS).
func rewriteLinkHrefs(html string, fn func(string) string) string {
	var b strings.Builder
	lower := strings.ToLower(html)
	i := 0
	for i < len(html) {
		// Find next <link
		idx := strings.Index(lower[i:], "<link")
		if idx == -1 {
			b.WriteString(html[i:])
			break
		}
		tagStart := i + idx
		tagEnd := strings.Index(html[tagStart:], ">")
		if tagEnd == -1 {
			b.WriteString(html[i:])
			break
		}
		tagEnd += tagStart + 1
		// Rewrite href inside this tag only.
		tag := html[tagStart:tagEnd]
		tag = rewriteAttrValues(tag, `href="`, fn)
		tag = rewriteAttrValues(tag, `href='`, fn)
		b.WriteString(html[i:tagStart])
		b.WriteString(tag)
		i = tagEnd
	}
	return b.String()
}
