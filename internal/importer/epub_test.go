package importer

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// createTestEpub creates a minimal valid EPUB file for testing.
func createTestEpub(t *testing.T, dir, title, author, series string, seriesIndex float64) string {
	t.Helper()
	path := filepath.Join(dir, "test.epub")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)

	// mimetype (must be first, uncompressed)
	mw, _ := w.Create("mimetype")
	mw.Write([]byte("application/epub+zip"))

	// container.xml
	cw, _ := w.Create("META-INF/container.xml")
	cw.Write([]byte(`<?xml version="1.0"?>
<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0">
  <rootfiles>
    <rootfile full-path="content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`))

	// content.opf
	seriesMeta := ""
	if series != "" {
		seriesMeta = `<meta name="calibre:series" content="` + series + `"/>
    <meta name="calibre:series_index" content="` + formatFloat(seriesIndex) + `"/>`
	}

	ow, _ := w.Create("content.opf")
	ow.Write([]byte(`<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>` + title + `</dc:title>
    <dc:creator>` + author + `</dc:creator>
    <dc:language>en</dc:language>
    <dc:publisher>Test Publisher</dc:publisher>
    <dc:description>A test book description.</dc:description>
    <dc:date>2020-01-15</dc:date>
    ` + seriesMeta + `
  </metadata>
  <manifest>
    <item id="ch1" href="chapter1.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="ch1"/>
  </spine>
</package>`))

	// chapter1.xhtml
	ch, _ := w.Create("chapter1.xhtml")
	ch.Write([]byte(`<html><body><h1>Chapter 1</h1><p>Hello world.</p></body></html>`))

	w.Close()
	f.Close()
	return path
}

func formatFloat(v float64) string {
	if v == float64(int(v)) {
		return fmt.Sprintf("%d", int(v))
	}
	return fmt.Sprintf("%.1f", v)
}

func TestExtractEpubMetadata(t *testing.T) {
	dir := t.TempDir()
	epubPath := createTestEpub(t, dir, "The Way of Kings", "Brandon Sanderson", "The Stormlight Archive", 1)

	meta, err := ExtractEpubMetadata(epubPath)
	if err != nil {
		t.Fatalf("ExtractEpubMetadata: %v", err)
	}

	if meta.Title != "The Way of Kings" {
		t.Errorf("Title = %q", meta.Title)
	}
	if len(meta.Authors) != 1 || meta.Authors[0] != "Brandon Sanderson" {
		t.Errorf("Authors = %v", meta.Authors)
	}
	if meta.Series != "The Stormlight Archive" {
		t.Errorf("Series = %q", meta.Series)
	}
	if meta.SeriesIndex != 1 {
		t.Errorf("SeriesIndex = %f", meta.SeriesIndex)
	}
	if meta.Language != "en" {
		t.Errorf("Language = %q", meta.Language)
	}
	if meta.Publisher != "Test Publisher" {
		t.Errorf("Publisher = %q", meta.Publisher)
	}
	if meta.Description == "" {
		t.Error("Description should not be empty")
	}
	if meta.PublishedYear != 2020 {
		t.Errorf("PublishedYear = %d", meta.PublishedYear)
	}
	if meta.Format != "epub" {
		t.Errorf("Format = %q", meta.Format)
	}
}

func TestExtractEpubChapter(t *testing.T) {
	dir := t.TempDir()
	epubPath := createTestEpub(t, dir, "Test Book", "Test Author", "", 0)

	html, _, chapters, err := ExtractEpubChapter(epubPath, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(chapters) != 1 {
		t.Errorf("chapters = %d, want 1", len(chapters))
	}
	if html == "" {
		t.Error("chapter HTML should not be empty")
	}
}
