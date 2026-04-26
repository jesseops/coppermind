package web

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jesseops/coppermind/internal/auth"
	"github.com/jesseops/coppermind/internal/domain"
	"github.com/jesseops/coppermind/internal/store"
)

func (s *Server) registerAPIRoutes(r chi.Router) {
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/works", s.apiListWorks)
		r.Get("/works/{id}", s.apiGetWork)
		r.Get("/editions/{id}", s.apiGetEdition)
		r.Get("/authors", s.apiListAuthors)
		r.Get("/series", s.apiListSeries)

		// Authenticated endpoints.
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth(s.store, s.secret))
			r.Get("/me/reading", s.apiListReading)
			// POST reading state already registered in main routes.
		})
	})
}

func (s *Server) apiListWorks(w http.ResponseWriter, r *http.Request) {
	libs, _ := s.store.ListLibraries()
	if len(libs) == 0 {
		jsonResp(w, http.StatusOK, map[string]any{"works": []any{}, "total": 0})
		return
	}

	query := r.URL.Query().Get("q")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := 50
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= 200 {
		limit = l
	}

	works, total, _ := s.store.ListWorks(store.WorkFilter{
		LibraryID: libs[0].ID,
		Query:     query,
		Type:      r.URL.Query().Get("type"),
		SortBy:    r.URL.Query().Get("sort"),
		SortOrder: r.URL.Query().Get("order"),
		Limit:     limit,
		Offset:    (page - 1) * limit,
	})

	jsonResp(w, http.StatusOK, map[string]any{
		"works": works,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (s *Server) apiGetWork(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	work, err := s.store.GetWork(id)
	if err != nil {
		jsonResp(w, http.StatusNotFound, map[string]string{"error": "work not found"})
		return
	}
	editions, _ := s.store.ListEditions(id)
	tags, _ := s.store.ListTags(id)
	jsonResp(w, http.StatusOK, map[string]any{
		"work":     work,
		"editions": editions,
		"tags":     tags,
	})
}

func (s *Server) apiGetEdition(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	edition, err := s.store.GetEdition(id)
	if err != nil {
		jsonResp(w, http.StatusNotFound, map[string]string{"error": "edition not found"})
		return
	}
	jsonResp(w, http.StatusOK, edition)
}

func (s *Server) apiListAuthors(w http.ResponseWriter, r *http.Request) {
	libs, _ := s.store.ListLibraries()
	if len(libs) == 0 {
		jsonResp(w, http.StatusOK, map[string]any{"authors": []any{}})
		return
	}
	authors, _ := s.store.ListAuthors(libs[0].ID)
	jsonResp(w, http.StatusOK, map[string]any{"authors": authors})
}

func (s *Server) apiListSeries(w http.ResponseWriter, r *http.Request) {
	libs, _ := s.store.ListLibraries()
	if len(libs) == 0 {
		jsonResp(w, http.StatusOK, map[string]any{"series": []any{}})
		return
	}
	series, _ := s.store.ListSeries(libs[0].ID)
	jsonResp(w, http.StatusOK, map[string]any{"series": series})
}

func (s *Server) apiListReading(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		jsonResp(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	states, _ := s.store.ListReadingStates(user.ID, domain.ReadingStatusReading)
	jsonResp(w, http.StatusOK, map[string]any{"reading": states})
}

func jsonResp(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
