package importer

// Extracted holds metadata extracted from a book file.
type Extracted struct {
	Title         string
	Authors       []string
	Series        string
	SeriesIndex   float64
	ISBN          string
	PublishedYear int
	Language      string
	Publisher     string
	Description   string
	Subjects      []string // genre/category tags
	Format        string   // "epub", "mobi", "mp3", "m4b", "pdf"
	CoverData     []byte
	CoverExt      string   // ".jpg", ".png", etc.
}
