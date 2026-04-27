package web

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/jesseops/coppermind/internal/auth"
	"github.com/jesseops/coppermind/internal/domain"
	"github.com/jesseops/coppermind/internal/store"
	"log/slog"
)

// CatalogueGroup represents a group of works (by author or series).
type CatalogueGroup struct {
	ID    int64
	Name  string
	Works []domain.Work
}

func (s *Server) handleCatalogue(w http.ResponseWriter, r *http.Request) {
	libs, err := s.store.ListLibraries()
	if err != nil {
		slog.Error("list libraries", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if len(libs) == 0 {
		s.render(w, r, "catalogue.html", templateData{
			"Works": nil, "Total": 0, "View": "grid",
		})
		return
	}

	// Parse query params.
	query := r.URL.Query().Get("q")
	sortBy := r.URL.Query().Get("sort")
	filterType := r.URL.Query().Get("type")
	groupBy := r.URL.Query().Get("group")
	view := r.URL.Query().Get("view")
	authorID, _ := strconv.ParseInt(r.URL.Query().Get("author"), 10, 64)
	seriesID, _ := strconv.ParseInt(r.URL.Query().Get("series"), 10, 64)
	shelfID, _ := strconv.ParseInt(r.URL.Query().Get("shelf"), 10, 64)

	if view == "" {
		view = "grid"
	}
	if sortBy == "" {
		sortBy = "recent"
	}

	// Map sort param to store sort.
	storeSortBy := ""
	switch sortBy {
	case "title":
		storeSortBy = "title"
	case "author":
		storeSortBy = "title" // sort by title within author groups
	case "year":
		storeSortBy = "year"
	case "series":
		storeSortBy = "series"
	default:
		storeSortBy = "" // default = created_at DESC
	}

	filter := store.WorkFilter{
		LibraryID: libs[0].ID,
		Query:     query,
		SortBy:    storeSortBy,
		Type:      filterType,
		AuthorID:  authorID,
		SeriesID:  seriesID,
		ShelfID:   shelfID,
	}

	works, total, err := s.store.ListWorks(filter)
	if err != nil {
		slog.Error("list works", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Build groups if requested.
	var groups []CatalogueGroup
	if groupBy == "author" {
		groups = groupWorksByAuthor(works)
	} else if groupBy == "series" {
		groups = groupWorksBySeries(works)
	}

	// Load user shelves for card actions.
	var shelves []domain.Shelf
	user := auth.UserFromContext(r.Context())
	if user != nil {
		shelves, _ = s.store.ListShelves(user.ID)
	}

	// Load first edition per work for action links.
	editionMap := make(map[int64]*domain.Edition)
	for _, work := range works {
		editions, _ := s.store.ListEditions(work.ID)
		if len(editions) > 0 {
			ed := editions[0]
			editionMap[work.ID] = &ed
		}
	}

	// Context for filtering display.
	var filterAuthorName, filterSeriesName, filterShelfName string
	if authorID > 0 {
		if a, err := s.store.GetAuthor(authorID); err == nil {
			filterAuthorName = a.Name
		}
	}
	if seriesID > 0 {
		if ser, err := s.store.GetSeries(seriesID); err == nil {
			filterSeriesName = ser.Name
		}
	}
	if shelfID > 0 {
		if sh, err := s.store.GetShelf(shelfID); err == nil {
			filterShelfName = sh.Name
		}
	}

	data := templateData{
		"Works":            works,
		"Total":            total,
		"Groups":           groups,
		"Query":            query,
		"Sort":             sortBy,
		"Type":             filterType,
		"Group":            groupBy,
		"View":             view,
		"AuthorID":         authorID,
		"SeriesID":         seriesID,
		"ShelfID":          shelfID,
		"FilterAuthorName": filterAuthorName,
		"FilterSeriesName": filterSeriesName,
		"FilterShelfName":  filterShelfName,
		"Shelves":          shelves,
		"EditionMap":       editionMap,
	}

	if r.Header.Get("HX-Request") == "true" {
		s.renderPartial(w, r, "catalogue_results", data)
		return
	}
	s.render(w, r, "catalogue.html", data)
}

func groupWorksByAuthor(works []domain.Work) []CatalogueGroup {
	m := make(map[int64]*CatalogueGroup)
	var order []int64
	for _, w := range works {
		authorID := int64(0)
		authorName := "Unknown Author"
		if len(w.Authors) > 0 {
			authorID = w.Authors[0].AuthorID
			authorName = w.Authors[0].AuthorName
		}
		if _, ok := m[authorID]; !ok {
			m[authorID] = &CatalogueGroup{ID: authorID, Name: authorName}
			order = append(order, authorID)
		}
		m[authorID].Works = append(m[authorID].Works, w)
	}
	// Sort groups alphabetically.
	sort.Slice(order, func(i, j int) bool {
		return m[order[i]].Name < m[order[j]].Name
	})
	groups := make([]CatalogueGroup, 0, len(order))
	for _, id := range order {
		groups = append(groups, *m[id])
	}
	return groups
}

func groupWorksBySeries(works []domain.Work) []CatalogueGroup {
	m := make(map[int64]*CatalogueGroup)
	var order []int64
	var noSeries []domain.Work
	for _, w := range works {
		if w.SeriesID == 0 {
			noSeries = append(noSeries, w)
			continue
		}
		if _, ok := m[w.SeriesID]; !ok {
			name := w.SeriesName
			if name == "" {
				name = "Unnamed Series"
			}
			m[w.SeriesID] = &CatalogueGroup{ID: w.SeriesID, Name: name}
			order = append(order, w.SeriesID)
		}
		m[w.SeriesID].Works = append(m[w.SeriesID].Works, w)
	}
	sort.Slice(order, func(i, j int) bool {
		return m[order[i]].Name < m[order[j]].Name
	})
	groups := make([]CatalogueGroup, 0, len(order)+1)
	for _, id := range order {
		groups = append(groups, *m[id])
	}
	if len(noSeries) > 0 {
		groups = append(groups, CatalogueGroup{Name: "Standalone", Works: noSeries})
	}
	return groups
}
