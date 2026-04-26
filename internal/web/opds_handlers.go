package web

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jesseops/coppermind/internal/domain"
	"github.com/jesseops/coppermind/internal/opds"
	"github.com/jesseops/coppermind/internal/store"
)

func (s *Server) registerOPDSRoutes(r chi.Router) {
	r.Route("/api/v1/opds", func(r chi.Router) {
		r.Get("/", s.opdsRoot)
		r.Get("/new", s.opdsNew)
		r.Get("/authors", s.opdsAuthors)
		r.Get("/authors/{id}", s.opdsAuthor)
		r.Get("/series", s.opdsSeries)
		r.Get("/series/{id}", s.opdsSeriesDetail)
		r.Get("/search", s.opdsSearch)
	})
}

func (s *Server) baseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

func (s *Server) opdsRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/atom+xml;charset=utf-8")
	w.Write(opds.RootCatalog(s.baseURL(r)))
}

func (s *Server) opdsNew(w http.ResponseWriter, r *http.Request) {
	base := s.baseURL(r)
	libs, _ := s.store.ListLibraries()
	if len(libs) == 0 {
		w.Header().Set("Content-Type", "application/atom+xml;charset=utf-8")
		w.Write(opds.AcquisitionFeed(base, "New Acquisitions", base+"/api/v1/opds/new", nil, nil))
		return
	}
	works, _, _ := s.store.ListWorks(store.WorkFilter{LibraryID: libs[0].ID, Limit: 50})
	edMap := s.buildEditionMap(works)
	w.Header().Set("Content-Type", "application/atom+xml;charset=utf-8")
	w.Write(opds.AcquisitionFeed(base, "New Acquisitions", base+"/api/v1/opds/new", works, edMap))
}

func (s *Server) opdsAuthors(w http.ResponseWriter, r *http.Request) {
	base := s.baseURL(r)
	libs, _ := s.store.ListLibraries()
	if len(libs) == 0 {
		w.Header().Set("Content-Type", "application/atom+xml;charset=utf-8")
		w.Write(opds.NavigationFeed(base, "Authors", base+"/api/v1/opds/authors", nil))
		return
	}
	authors, _ := s.store.ListAuthors(libs[0].ID)
	entries := make([]opds.Entry, 0, len(authors))
	for _, a := range authors {
		entries = append(entries, opds.Entry{
			Title:   a.Name,
			ID:      fmt.Sprintf("%s/api/v1/opds/authors/%d", base, a.ID),
			Updated: a.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
			Content: &opds.Content{Type: "text", Body: fmt.Sprintf("%d works", a.WorkCount)},
			Links:   []opds.Link{{Href: fmt.Sprintf("%s/api/v1/opds/authors/%d", base, a.ID), Type: "application/atom+xml;profile=opds-catalog;kind=acquisition"}},
		})
	}
	w.Header().Set("Content-Type", "application/atom+xml;charset=utf-8")
	w.Write(opds.NavigationFeed(base, "Authors", base+"/api/v1/opds/authors", entries))
}

func (s *Server) opdsAuthor(w http.ResponseWriter, r *http.Request) {
	base := s.baseURL(r)
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
	edMap := s.buildEditionMap(works)
	w.Header().Set("Content-Type", "application/atom+xml;charset=utf-8")
	w.Write(opds.AcquisitionFeed(base, author.Name, fmt.Sprintf("%s/api/v1/opds/authors/%d", base, id), works, edMap))
}

func (s *Server) opdsSeries(w http.ResponseWriter, r *http.Request) {
	base := s.baseURL(r)
	libs, _ := s.store.ListLibraries()
	if len(libs) == 0 {
		w.Header().Set("Content-Type", "application/atom+xml;charset=utf-8")
		w.Write(opds.NavigationFeed(base, "Series", base+"/api/v1/opds/series", nil))
		return
	}
	series, _ := s.store.ListSeries(libs[0].ID)
	entries := make([]opds.Entry, 0, len(series))
	for _, se := range series {
		entries = append(entries, opds.Entry{
			Title:   se.Name,
			ID:      fmt.Sprintf("%s/api/v1/opds/series/%d", base, se.ID),
			Updated: se.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
			Content: &opds.Content{Type: "text", Body: fmt.Sprintf("%d works", se.WorkCount)},
			Links:   []opds.Link{{Href: fmt.Sprintf("%s/api/v1/opds/series/%d", base, se.ID), Type: "application/atom+xml;profile=opds-catalog;kind=acquisition"}},
		})
	}
	w.Header().Set("Content-Type", "application/atom+xml;charset=utf-8")
	w.Write(opds.NavigationFeed(base, "Series", base+"/api/v1/opds/series", entries))
}

func (s *Server) opdsSeriesDetail(w http.ResponseWriter, r *http.Request) {
	base := s.baseURL(r)
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
	edMap := s.buildEditionMap(works)
	w.Header().Set("Content-Type", "application/atom+xml;charset=utf-8")
	w.Write(opds.AcquisitionFeed(base, ser.Name, fmt.Sprintf("%s/api/v1/opds/series/%d", base, id), works, edMap))
}

func (s *Server) opdsSearch(w http.ResponseWriter, r *http.Request) {
	base := s.baseURL(r)
	query := r.URL.Query().Get("q")
	libs, _ := s.store.ListLibraries()
	var works []domain.Work
	if len(libs) > 0 && query != "" {
		works, _, _ = s.store.ListWorks(store.WorkFilter{LibraryID: libs[0].ID, Query: query, Limit: 50})
	}
	edMap := s.buildEditionMap(works)
	w.Header().Set("Content-Type", "application/atom+xml;charset=utf-8")
	w.Write(opds.AcquisitionFeed(base, "Search: "+query, base+"/api/v1/opds/search?q="+query, works, edMap))
}

func (s *Server) buildEditionMap(works []domain.Work) map[int64][]domain.Edition {
	edMap := make(map[int64][]domain.Edition)
	for _, w := range works {
		editions, _ := s.store.ListEditions(w.ID)
		edMap[w.ID] = editions
	}
	return edMap
}
