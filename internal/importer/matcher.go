package importer

import (
	"github.com/jesseops/coppermind/internal/domain"
	"github.com/jesseops/coppermind/internal/store"
)

// MatchResult contains the results of matching extracted metadata against the store.
type MatchResult struct {
	Work               *domain.Work
	Authors            []domain.Author
	Series             *domain.Series
	IsNewWork          bool
	IsDuplicate        bool
	DuplicateEditionID int64
}

// Matcher matches extracted metadata to existing or new entities in the store.
type Matcher struct {
	store store.Store
}

// NewMatcher creates a new Matcher.
func NewMatcher(s store.Store) *Matcher {
	return &Matcher{store: s}
}

// Match takes extracted metadata and finds or creates the corresponding Work, Authors, and Series.
func (m *Matcher) Match(meta *Extracted, libraryID int64, fileHash string) (*MatchResult, error) {
	result := &MatchResult{}

	// 1. Check for duplicate by file hash.
	if fileHash != "" {
		existing, err := m.store.FindEditionByHash(fileHash)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			result.IsDuplicate = true
			result.DuplicateEditionID = existing.ID
			return result, nil
		}
	}

	// 2. Match or create authors.
	var authors []domain.Author
	for _, name := range meta.Authors {
		sortName := domain.GenerateSortName(name)
		existing, err := m.store.FindAuthorBySortName(sortName)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			authors = append(authors, *existing)
		} else {
			created, err := m.store.CreateAuthor(name, sortName)
			if err != nil {
				return nil, err
			}
			authors = append(authors, *created)
		}
	}
	result.Authors = authors

	// 3. Match or create series.
	if meta.Series != "" {
		existing, err := m.store.FindSeriesByName(meta.Series)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			result.Series = existing
		} else {
			created, err := m.store.CreateSeries(meta.Series, "")
			if err != nil {
				return nil, err
			}
			result.Series = created
		}
	}

	// 4. Match or create work.
	sortTitle := domain.GenerateSortTitle(meta.Title)
	var primaryAuthorSortName string
	if len(authors) > 0 {
		primaryAuthorSortName = authors[0].SortName
	}

	if meta.Title != "" && primaryAuthorSortName != "" {
		existing, err := m.store.FindWorkByTitleAndAuthor(libraryID, sortTitle, primaryAuthorSortName)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			result.Work = existing
			result.IsNewWork = false
			return result, nil
		}
	}

	// Create new work.
	work := &domain.Work{
		LibraryID:      libraryID,
		Title:          meta.Title,
		SortTitle:      sortTitle,
		Description:    meta.Description,
		Language:        meta.Language,
		FirstPublished: meta.PublishedYear,
	}
	if result.Series != nil {
		work.SeriesID = result.Series.ID
		work.SeriesIndex = meta.SeriesIndex
	}
	if err := m.store.CreateWork(work); err != nil {
		return nil, err
	}

	// Link authors to work.
	for _, a := range authors {
		if err := m.store.LinkWorkAuthor(work.ID, a.ID, domain.RoleAuthorOf); err != nil {
			return nil, err
		}
	}

	result.Work = work
	result.IsNewWork = true
	return result, nil
}
