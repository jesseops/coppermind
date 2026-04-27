package store

import (
	"github.com/jesseops/coppermind/internal/domain"
)

// Store is the main interface for all database operations.
type Store interface {
	Closer
	LibraryStore
	UserStore
	AuthorStore
	SeriesStore
	WorkStore
	EditionStore
	TrackStore
	ReadingStore
	RatingStore
	ShelfStore
	TagStore
}

// ImportStore is the subset needed by the import/matching pipeline.
type ImportStore interface {
	AuthorStore
	SeriesStore
	WorkStore
	EditionStore
	TrackStore
}

type Closer interface {
	Close() error
}

type LibraryStore interface {
	CreateLibrary(name string) (*domain.Library, error)
	GetLibrary(id int64) (*domain.Library, error)
	ListLibraries() ([]domain.Library, error)
}

type UserStore interface {
	CreateUser(username, displayName, passwordHash, role string) (*domain.User, error)
	GetUser(id int64) (*domain.User, error)
	GetUserByUsername(username string) (*domain.User, error)
	ListUsers() ([]domain.User, error)
	UpdateUser(id int64, updates UserUpdate) error
	DeleteUser(id int64) error
	CountUsers() (int, error)
}

type AuthorStore interface {
	CreateAuthor(name, sortName string) (*domain.Author, error)
	GetAuthor(id int64) (*domain.Author, error)
	FindAuthorBySortName(sortName string) (*domain.Author, error)
	ListAuthors(libraryID int64) ([]AuthorWithCount, error)
	LinkWorkAuthor(workID, authorID int64, role string) error
	UnlinkWorkAuthor(workID, authorID int64, role string) error
	GetWorkAuthors(workID int64) ([]domain.WorkAuthor, error)
}

type SeriesStore interface {
	CreateSeries(name, description string) (*domain.Series, error)
	GetSeries(id int64) (*domain.Series, error)
	FindSeriesByName(name string) (*domain.Series, error)
	ListSeries(libraryID int64) ([]SeriesWithCount, error)
}

type WorkStore interface {
	CreateWork(w *domain.Work) error
	GetWork(id int64) (*domain.Work, error)
	UpdateWork(id int64, updates WorkUpdate) error
	DeleteWork(id int64) error
	MergeWorks(targetID int64, sourceIDs []int64) error
	ListWorks(filter WorkFilter) ([]domain.Work, int, error)
	FindWorkByTitleAndAuthor(libraryID int64, sortTitle, authorSortName string) (*domain.Work, error)
}

type EditionStore interface {
	CreateEdition(e *domain.Edition) error
	GetEdition(id int64) (*domain.Edition, error)
	ListEditions(workID int64) ([]domain.Edition, error)
	UpdateEdition(id int64, updates EditionUpdate) error
	DeleteEdition(id int64) error
	FindEditionByHash(hash string) (*domain.Edition, error)
}

type TrackStore interface {
	CreateTrack(t *domain.Track) error
	GetTrack(id int64) (*domain.Track, error)
	ListTracks(editionID int64) ([]domain.Track, error)
	ReplaceTracksForEdition(editionID int64, tracks []domain.Track) error
}

type ReadingStore interface {
	GetReadingState(userID, editionID int64) (*domain.ReadingState, error)
	SaveReadingState(state *domain.ReadingState) error
	ListReadingStates(userID int64, status string) ([]domain.ReadingState, error)
}

type RatingStore interface {
	SaveRating(userID, workID int64, rating int, review string) error
	GetRating(userID, workID int64) (*domain.UserRating, error)
	ListRatings(workID int64) ([]domain.UserRating, error)
}

type ShelfStore interface {
	CreateShelf(userID int64, name, description string, isPublic bool) (*domain.Shelf, error)
	GetShelf(id int64) (*domain.Shelf, error)
	ListShelves(userID int64) ([]domain.Shelf, error)
	DeleteShelf(id int64) error
	AddToShelf(shelfID, workID int64) error
	RemoveFromShelf(shelfID, workID int64) error
	ListShelfWorks(shelfID int64) ([]domain.Work, error)
}

type TagStore interface {
	AddTag(workID int64, tag string) error
	RemoveTag(workID int64, tag string) error
	ListTags(workID int64) ([]string, error)
	ListAllTags(libraryID int64) ([]TagWithCount, error)
}

// ── Filter / Update types ───────────────────────────────────────────

// WorkFilter specifies criteria for listing works.
type WorkFilter struct {
	LibraryID     int64
	Query         string // text search across title, author, series
	Type          string // "ebook", "audiobook", or "" for all
	SeriesID      int64
	AuthorID      int64
	ShelfID       int64
	SortBy        string // "title", "author", "created_at", "updated_at", "series", "year"
	SortOrder     string // "asc" or "desc"
	Limit         int
	Offset        int
	IncludeHidden bool   // if false (default), hidden works are excluded
	OnlyHidden    bool   // if true, only return hidden works
	MissingCover  bool   // if true, only return works without covers
	MissingAuthor bool   // if true, only return works with no authors
	MissingDesc   bool   // if true, only return works with no description
	Format        string // edition format: "epub", "mobi", "pdf", etc.
}

// WorkUpdate specifies partial updates to a work.
type WorkUpdate struct {
	Title          *string
	Description    *string
	SeriesID       *int64
	SeriesIndex    *float64
	Language       *string
	FirstPublished *int
	CoverPath      *string
	Hidden         *bool
}

// EditionUpdate specifies partial updates to an edition.
type EditionUpdate struct {
	Format          *string
	ISBN            *string
	Publisher       *string
	PublishedYear   *int
	Narrator        *string
	DurationSeconds *int
	FilePath        *string
	FileHash        *string
	FileSize        *int64
	CoverPath       *string
	Notes           *string
	Status          *string
}

// UserUpdate specifies partial updates to a user.
type UserUpdate struct {
	DisplayName  *string
	PasswordHash *string
	Role         *string
	KindleEmail  *string
}

// ── Aggregate types ─────────────────────────────────────────────────

// AuthorWithCount pairs an author with the number of works.
type AuthorWithCount struct {
	domain.Author
	WorkCount int `db:"work_count" json:"work_count"`
}

// SeriesWithCount pairs a series with the number of works.
type SeriesWithCount struct {
	domain.Series
	WorkCount int `db:"work_count" json:"work_count"`
}

// TagWithCount pairs a tag string with a count.
type TagWithCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}
