package opds

import (
	"encoding/xml"
	"fmt"
	"time"

	"github.com/jesseops/coppermind/internal/domain"
)

// ── OPDS Atom types ─────────────────────────────────────────────────

type Feed struct {
	XMLName xml.Name `xml:"feed"`
	XMLNS   string   `xml:"xmlns,attr"`
	ID      string   `xml:"id"`
	Title   string   `xml:"title"`
	Updated string   `xml:"updated"`
	Author  *Author  `xml:"author,omitempty"`
	Links   []Link   `xml:"link"`
	Entries []Entry  `xml:"entry"`
}

type Author struct {
	Name string `xml:"name"`
}

type Link struct {
	Rel      string `xml:"rel,attr,omitempty"`
	Href     string `xml:"href,attr"`
	Type     string `xml:"type,attr,omitempty"`
	Title    string `xml:"title,attr,omitempty"`
}

type Entry struct {
	Title   string  `xml:"title"`
	ID      string  `xml:"id"`
	Updated string  `xml:"updated"`
	Content *Content `xml:"content,omitempty"`
	Authors []EntryAuthor `xml:"author,omitempty"`
	Links   []Link  `xml:"link"`
}

type Content struct {
	Type string `xml:"type,attr"`
	Body string `xml:",chardata"`
}

type EntryAuthor struct {
	Name string `xml:"name"`
}

const (
	nsAtom = "http://www.w3.org/2005/Atom"
	typeNavigation = "application/atom+xml;profile=opds-catalog;kind=navigation"
	typeAcquisition = "application/atom+xml;profile=opds-catalog;kind=acquisition"
)

// ── Feed generators ─────────────────────────────────────────────────

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// RootCatalog generates the OPDS root navigation feed.
func RootCatalog(baseURL string) []byte {
	feed := Feed{
		XMLNS:   nsAtom,
		ID:      baseURL + "/api/v1/opds",
		Title:   "Coppermind Library",
		Updated: now(),
		Links: []Link{
			{Rel: "self", Href: baseURL + "/api/v1/opds", Type: typeNavigation},
			{Rel: "start", Href: baseURL + "/api/v1/opds", Type: typeNavigation},
			{Rel: "search", Href: baseURL + "/api/v1/opds/search?q={searchTerms}", Type: "application/opensearchdescription+xml"},
		},
		Entries: []Entry{
			{
				Title:   "New Acquisitions",
				ID:      baseURL + "/api/v1/opds/new",
				Updated: now(),
				Content: &Content{Type: "text", Body: "Recently added books"},
				Links:   []Link{{Href: baseURL + "/api/v1/opds/new", Type: typeAcquisition}},
			},
			{
				Title:   "By Author",
				ID:      baseURL + "/api/v1/opds/authors",
				Updated: now(),
				Content: &Content{Type: "text", Body: "Browse by author"},
				Links:   []Link{{Href: baseURL + "/api/v1/opds/authors", Type: typeNavigation}},
			},
			{
				Title:   "By Series",
				ID:      baseURL + "/api/v1/opds/series",
				Updated: now(),
				Content: &Content{Type: "text", Body: "Browse by series"},
				Links:   []Link{{Href: baseURL + "/api/v1/opds/series", Type: typeNavigation}},
			},
		},
	}
	data, _ := xml.MarshalIndent(feed, "", "  ")
	return append([]byte(xml.Header), data...)
}

// AcquisitionFeed generates an OPDS acquisition feed from works and their editions.
func AcquisitionFeed(baseURL, title, id string, works []domain.Work, editions map[int64][]domain.Edition) []byte {
	entries := make([]Entry, 0, len(works))
	for _, w := range works {
		entry := Entry{
			Title:   w.Title,
			ID:      fmt.Sprintf("%s/api/v1/opds/works/%d", baseURL, w.ID),
			Updated: w.UpdatedAt.UTC().Format(time.RFC3339),
		}
		if w.Description != "" {
			entry.Content = &Content{Type: "text", Body: w.Description}
		}
		for _, a := range w.Authors {
			entry.Authors = append(entry.Authors, EntryAuthor{Name: a.AuthorName})
		}
		if w.HasCover() {
			entry.Links = append(entry.Links, Link{
				Rel:  "http://opds-spec.org/image",
				Href: fmt.Sprintf("%s/covers/%d", baseURL, w.ID),
				Type: "image/jpeg",
			})
		}
		for _, e := range editions[w.ID] {
			mimeType := formatToMIME(e.Format)
			entry.Links = append(entry.Links, Link{
				Rel:  "http://opds-spec.org/acquisition",
				Href: fmt.Sprintf("%s/download/%d", baseURL, e.ID),
				Type: mimeType,
			})
		}
		entries = append(entries, entry)
	}

	feed := Feed{
		XMLNS:   nsAtom,
		ID:      id,
		Title:   title,
		Updated: now(),
		Links: []Link{
			{Rel: "self", Href: id, Type: typeAcquisition},
			{Rel: "start", Href: baseURL + "/api/v1/opds", Type: typeNavigation},
		},
		Entries: entries,
	}
	data, _ := xml.MarshalIndent(feed, "", "  ")
	return append([]byte(xml.Header), data...)
}

// NavigationFeed generates an OPDS navigation feed (e.g., author list).
func NavigationFeed(baseURL, title, id string, entries []Entry) []byte {
	feed := Feed{
		XMLNS:   nsAtom,
		ID:      id,
		Title:   title,
		Updated: now(),
		Links: []Link{
			{Rel: "self", Href: id, Type: typeNavigation},
			{Rel: "start", Href: baseURL + "/api/v1/opds", Type: typeNavigation},
		},
		Entries: entries,
	}
	data, _ := xml.MarshalIndent(feed, "", "  ")
	return append([]byte(xml.Header), data...)
}

func formatToMIME(format string) string {
	switch format {
	case "epub":
		return "application/epub+zip"
	case "mobi":
		return "application/x-mobipocket-ebook"
	case "pdf":
		return "application/pdf"
	case "m4b":
		return "audio/mp4"
	case "mp3":
		return "audio/mpeg"
	default:
		return "application/octet-stream"
	}
}
