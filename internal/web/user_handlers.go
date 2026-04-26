package web

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jesseops/coppermind/internal/auth"
	"github.com/jesseops/coppermind/internal/sender"
	"github.com/jesseops/coppermind/internal/store"
)

// ── Shelves ─────────────────────────────────────────────────────────

func (s *Server) handleShelves(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	shelves, _ := s.store.ListShelves(user.ID)
	s.render(w, r, "shelves.html", templateData{"Shelves": shelves})
}

func (s *Server) handleCreateShelf(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	name := r.FormValue("name")
	desc := r.FormValue("description")
	isPublic := r.FormValue("is_public") == "on"

	if name == "" {
		s.render(w, r, "shelves.html", templateData{"Error": "Name is required"})
		return
	}
	s.store.CreateShelf(user.ID, name, desc, isPublic)
	http.Redirect(w, r, "/me/shelves", http.StatusSeeOther)
}

func (s *Server) handleShelfDetail(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	shelf, err := s.store.GetShelf(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	works, _ := s.store.ListShelfWorks(id)
	s.render(w, r, "shelf.html", templateData{
		"Shelf": shelf,
		"Works": works,
	})
}

func (s *Server) handleAddToShelf(w http.ResponseWriter, r *http.Request) {
	shelfID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	workID, _ := strconv.ParseInt(r.FormValue("work_id"), 10, 64)
	s.store.AddToShelf(shelfID, workID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRemoveFromShelf(w http.ResponseWriter, r *http.Request) {
	shelfID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	workID, _ := strconv.ParseInt(r.FormValue("work_id"), 10, 64)
	s.store.RemoveFromShelf(shelfID, workID)
	w.WriteHeader(http.StatusNoContent)
}

// ── Ratings ─────────────────────────────────────────────────────────

func (s *Server) handleRate(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	workID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	rating, _ := strconv.Atoi(r.FormValue("rating"))
	review := r.FormValue("review")

	if rating < 1 || rating > 5 {
		http.Error(w, "Rating must be 1-5", http.StatusBadRequest)
		return
	}

	s.store.SaveRating(user.ID, workID, rating, review)

	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/works/%d", workID), http.StatusSeeOther)
}

// ── Send to Kindle ──────────────────────────────────────────────────

func (s *Server) handleSendToKindle(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	editionID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	edition, err := s.store.GetEdition(editionID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	kindleEmail := user.KindleEmail
	if kindleEmail == "" {
		http.Error(w, "No Kindle email configured in your profile", http.StatusBadRequest)
		return
	}

	if !s.config.HasSMTP() {
		http.Error(w, "SMTP not configured on this server", http.StatusBadRequest)
		return
	}

	err = sender.SendToKindle(sender.Config{
		Host:     s.config.SMTPHost,
		Port:     s.config.SMTPPort,
		Username: s.config.SMTPUser,
		Password: s.config.SMTPPass,
		From:     s.config.SMTPFrom,
	}, kindleEmail, edition.FilePath)

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to send: %v", err), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Write([]byte("Sent!"))
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/works/%d", edition.WorkID), http.StatusSeeOther)
}

// ── Profile ─────────────────────────────────────────────────────────

func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	displayName := r.FormValue("display_name")
	kindleEmail := r.FormValue("kindle_email")
	newPassword := r.FormValue("new_password")

	updates := store.UserUpdate{}
	if displayName != "" {
		updates.DisplayName = &displayName
	}
	updates.KindleEmail = &kindleEmail

	if newPassword != "" {
		hash, _ := auth.HashPassword(newPassword)
		updates.PasswordHash = &hash
	}

	s.store.UpdateUser(user.ID, updates)
	http.Redirect(w, r, "/me", http.StatusSeeOther)
}

// registerUserRoutes adds shelf/rating/profile routes.
func (s *Server) registerUserRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth(s.store, s.secret))
		r.Get("/me/shelves", s.handleShelves)
		r.Post("/me/shelves", s.handleCreateShelf)
		r.Post("/me/rate/{id}", s.handleRate)
		r.Post("/me", s.handleUpdateProfile)
		r.Post("/send/{id}", s.handleSendToKindle)
	})
	r.Get("/shelves/{id}", s.handleShelfDetail)
	r.Post("/shelves/{id}/add", s.handleAddToShelf)
	r.Post("/shelves/{id}/remove", s.handleRemoveFromShelf)
}
