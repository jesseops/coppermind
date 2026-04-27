package web

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jesseops/coppermind/internal/auth"
	"github.com/jesseops/coppermind/internal/domain"
	"github.com/jesseops/coppermind/internal/importer"
	"github.com/jesseops/coppermind/internal/store"
)

// ── Public handlers ─────────────────────────────────────────────────

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	libs, _ := s.store.ListLibraries()
	if len(libs) == 0 {
		s.render(w, r, "home.html", templateData{"Works": nil, "Total": 0})
		return
	}

	query := r.URL.Query().Get("q")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := 50
	offset := (page - 1) * limit

	works, total, _ := s.store.ListWorks(store.WorkFilter{
		LibraryID: libs[0].ID,
		Query:     query,
		SortBy:    r.URL.Query().Get("sort"),
		SortOrder: r.URL.Query().Get("order"),
		Type:      r.URL.Query().Get("type"),
		Limit:     limit,
		Offset:    offset,
	})

	totalPages := (total + limit - 1) / limit

	data := templateData{
		"Works":      works,
		"Total":      total,
		"Query":      query,
		"Page":       page,
		"TotalPages": totalPages,
		"HasPrev":    page > 1,
		"HasNext":    page < totalPages,
		"PrevPage":   page - 1,
		"NextPage":   page + 1,
	}

	if r.Header.Get("HX-Request") == "true" {
		s.renderPartial(w, r, "works_grid", data)
		return
	}
	s.render(w, r, "home.html", data)
}

func (s *Server) handleWorkDetail(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	work, err := s.store.GetWork(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	editions, _ := s.store.ListEditions(id)
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

func (s *Server) handleAuthors(w http.ResponseWriter, r *http.Request) {
	libs, _ := s.store.ListLibraries()
	if len(libs) == 0 {
		s.render(w, r, "authors.html", templateData{"Authors": nil})
		return
	}
	authors, _ := s.store.ListAuthors(libs[0].ID)
	s.render(w, r, "authors.html", templateData{"Authors": authors})
}

func (s *Server) handleAuthorDetail(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	author, err := s.store.GetAuthor(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	libs, _ := s.store.ListLibraries()
	var works []domain.Work
	if len(libs) > 0 {
		works, _, _ = s.store.ListWorks(store.WorkFilter{LibraryID: libs[0].ID, AuthorID: id})
	}
	s.render(w, r, "author_detail.html", templateData{
		"Author": author,
		"Works":  works,
	})
}

func (s *Server) handleSeriesList(w http.ResponseWriter, r *http.Request) {
	libs, _ := s.store.ListLibraries()
	if len(libs) == 0 {
		s.render(w, r, "series_list.html", templateData{"Series": nil})
		return
	}
	series, _ := s.store.ListSeries(libs[0].ID)
	s.render(w, r, "series_list.html", templateData{"Series": series})
}

func (s *Server) handleSeriesDetail(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	ser, err := s.store.GetSeries(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	libs, _ := s.store.ListLibraries()
	var works []domain.Work
	if len(libs) > 0 {
		works, _, _ = s.store.ListWorks(store.WorkFilter{LibraryID: libs[0].ID, SeriesID: id, SortBy: "series"})
	}
	s.render(w, r, "series_detail.html", templateData{
		"Series": ser,
		"Works":  works,
	})
}

func (s *Server) handleCover(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	work, err := s.store.GetWork(id)
	if err != nil || work.CoverPath == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, work.CoverPath)
}

func (s *Server) handleEditionCover(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	edition, err := s.store.GetEdition(id)
	if err != nil || edition.CoverPath == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, edition.CoverPath)
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	edition, err := s.store.GetEdition(id)
	if err != nil || edition.FilePath == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(edition.FilePath)))
	http.ServeFile(w, r, edition.FilePath)
}

// ── Auth handlers ───────────────────────────────────────────────────

func (s *Server) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "login.html", nil)
}

func (s *Server) handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
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
	chapter, _ := strconv.Atoi(r.URL.Query().Get("chapter"))

	chapterHTML, chapterPath, chapters, err := importer.ExtractEpubChapter(edition.FilePath, chapter)
	if err != nil {
		http.Error(w, "Failed to read chapter", http.StatusInternalServerError)
		return
	}

	work, _ := s.store.GetWork(edition.WorkID)
	s.render(w, r, "reader.html", templateData{
		"Work":        work,
		"Edition":     edition,
		"ChapterHTML": template.HTML(chapterHTML),
		"ChapterPath": chapterPath,
		"Chapters":    chapters,
		"CurrentCh":   chapter,
	})
}

func (s *Server) handlePlayer(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	edition, err := s.store.GetEdition(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	tracks, _ := s.store.ListTracks(id)
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
	assetPath := chi.URLParam(r, "*")
	data, err := importer.ExtractEpubAsset(edition.FilePath, assetPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(data)
}

func (s *Server) handleAudioTrack(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	track, err := s.store.GetTrack(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, track.FilePath)
}

// ── User handlers ───────────────────────────────────────────────────

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	s.render(w, r, "profile.html", templateData{"User": user})
}

func (s *Server) handleCurrentlyReading(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	states, _ := s.store.ListReadingStates(user.ID, domain.ReadingStatusReading)
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

	s.store.SaveReadingState(&domain.ReadingState{
		UserID:       user.ID,
		EditionID:    editionID,
		Status:       domain.ReadingStatusReading,
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
	editions, _ := s.store.ListEditions(id)
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
	s.store.UpdateWork(id, update)
	http.Redirect(w, r, fmt.Sprintf("/works/%d", id), http.StatusSeeOther)
}

func (s *Server) handleAdminDeleteWork(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	s.store.DeleteWork(id)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	users, _ := s.store.ListUsers()
	s.render(w, r, "admin_users.html", templateData{"Users": users})
}

func (s *Server) handleAdminCreateUser(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	role := r.FormValue("role")
	if role == "" {
		role = domain.RoleViewer
	}

	hash, _ := auth.HashPassword(password)
	s.store.CreateUser(username, username, hash, role)
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (s *Server) handleAdminDuplicates(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "admin_duplicates.html", nil)
}
