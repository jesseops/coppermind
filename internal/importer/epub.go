package importer

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"path"
	"strconv"
	"strings"
)

// ── XML types ───────────────────────────────────────────────────────

type containerXML struct {
	Rootfiles []containerRootfile `xml:"rootfiles>rootfile"`
}

type containerRootfile struct {
	FullPath string `xml:"full-path,attr"`
}

type opfPackage struct {
	Metadata opfMetadata `xml:"metadata"`
	Manifest opfManifest `xml:"manifest"`
	Spine    opfSpine    `xml:"spine"`
}

type opfMetadata struct {
	Titles       []string        `xml:"title"`
	Creators     []string        `xml:"creator"`
	Subjects     []string        `xml:"subject"`
	Identifiers  []opfIdentifier `xml:"identifier"`
	Dates        []string        `xml:"date"`
	Languages    []string        `xml:"language"`
	Publishers   []string        `xml:"publisher"`
	Descriptions []string        `xml:"description"`
	Meta         []opfMeta       `xml:"meta"`
}

type opfMeta struct {
	Name     string `xml:"name,attr"`
	Property string `xml:"property,attr"`
	Content  string `xml:"content,attr"`
	Value    string `xml:",chardata"`
}

type opfIdentifier struct {
	ID     string `xml:"id,attr"`
	Scheme string `xml:"scheme,attr"`
	Value  string `xml:",chardata"`
}

type opfManifest struct {
	Items []opfItem `xml:"item"`
}

type opfItem struct {
	ID         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	MediaType  string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`
}

type opfSpine struct {
	Itemrefs []opfItemref `xml:"itemref"`
}

type opfItemref struct {
	IDRef string `xml:"idref,attr"`
}

// ── EPUB metadata extraction ────────────────────────────────────────

// ExtractEpubMetadata extracts metadata and cover from an EPUB file.
func ExtractEpubMetadata(filePath string) (*Extracted, error) {
	archive, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("open epub: %w", err)
	}
	defer archive.Close()

	opf, opfPath, err := parseOPF(&archive.Reader)
	if err != nil {
		return nil, err
	}

	ext := &Extracted{
		Title:         firstNonEmpty(opf.Metadata.Titles),
		Authors:       nonEmptyStrings(opf.Metadata.Creators),
		Series:        extractSeries(opf.Metadata.Meta),
		SeriesIndex:   extractSeriesIndex(opf.Metadata.Meta),
		PublishedYear: extractYear(opf.Metadata.Dates),
		ISBN:          extractISBNFromOPF(opf.Metadata),
		Language:      firstNonEmpty(opf.Metadata.Languages),
		Publisher:     firstNonEmpty(opf.Metadata.Publishers),
		Description:   cleanDescription(firstNonEmpty(opf.Metadata.Descriptions)),
		Subjects:      nonEmptyStrings(opf.Metadata.Subjects),
		Format:        "epub",
	}

	// Extract cover.
	coverID := findCoverID(opf.Metadata.Meta)
	coverItem := findCoverItem(opf.Manifest.Items, coverID)
	if coverItem != nil {
		coverPath := resolveEpubPath(opfPath, coverItem.Href)
		data, err := readZipFile(&archive.Reader, coverPath)
		if err == nil && len(data) > 0 {
			ext.CoverData = data
			ext.CoverExt = coverExt(coverItem.Href, coverItem.MediaType)
		}
	}

	return ext, nil
}

// ── EPUB reader support ─────────────────────────────────────────────

// ReaderChapter describes a chapter in an EPUB for the reader.
type ReaderChapter struct {
	Index int
	Title string
}

// ExtractEpubChapter returns chapter HTML, the chapter file path within the EPUB,
// and the full chapter list.
func ExtractEpubChapter(filePath string, index int) (string, string, []ReaderChapter, error) {
	archive, err := zip.OpenReader(filePath)
	if err != nil {
		return "", "", nil, err
	}
	defer archive.Close()

	opf, opfPath, err := parseOPF(&archive.Reader)
	if err != nil {
		return "", "", nil, err
	}

	manifest := map[string]opfItem{}
	for _, item := range opf.Manifest.Items {
		manifest[item.ID] = item
	}

	var chapters []ReaderChapter
	var spineItems []opfItem
	for _, itemref := range opf.Spine.Itemrefs {
		item, ok := manifest[itemref.IDRef]
		if !ok || !isTextItem(item.MediaType) {
			continue
		}
		chapters = append(chapters, ReaderChapter{
			Index: len(spineItems),
			Title: chapterTitle(item.Href),
		})
		spineItems = append(spineItems, item)
	}

	if len(spineItems) == 0 {
		return "", "", chapters, nil
	}
	if index < 0 || index >= len(spineItems) {
		index = 0
	}

	chapterPath := resolveEpubPath(opfPath, spineItems[index].Href)
	data, err := readZipFile(&archive.Reader, chapterPath)
	if err != nil {
		return "", "", chapters, err
	}
	return string(data), chapterPath, chapters, nil
}

// ExtractEpubChapterText returns plain text for an EPUB chapter.
func ExtractEpubChapterText(filePath string, index int) (string, []ReaderChapter, error) {
	chapterHTML, _, chapters, err := ExtractEpubChapter(filePath, index)
	if err != nil {
		return "", nil, err
	}
	text := html.UnescapeString(stripHTMLTagsWithBreaks(chapterHTML))
	text = normalizeWhitespacePreserveLines(text)
	return text, chapters, nil
}

// ExtractEpubAsset returns raw bytes for an asset within an EPUB.
func ExtractEpubAsset(filePath string, assetPath string) ([]byte, error) {
	archive, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, err
	}
	defer archive.Close()
	return readZipFile(&archive.Reader, assetPath)
}

// ExtractTextPreview returns a plain-text preview of a book's content.
func ExtractTextPreview(filePath string, maxChars int) (string, error) {
	return extractEpubTextPreview(filePath, maxChars)
}

// ── Internal helpers ────────────────────────────────────────────────

func parseOPF(archive *zip.Reader) (*opfPackage, string, error) {
	containerData, err := readZipFile(archive, "META-INF/container.xml")
	if err != nil {
		return nil, "", fmt.Errorf("read container.xml: %w", err)
	}

	var container containerXML
	if err := xml.Unmarshal(containerData, &container); err != nil {
		return nil, "", fmt.Errorf("parse container.xml: %w", err)
	}
	if len(container.Rootfiles) == 0 {
		return nil, "", fmt.Errorf("epub missing rootfile")
	}

	opfPath := container.Rootfiles[0].FullPath
	opfData, err := readZipFile(archive, opfPath)
	if err != nil {
		return nil, "", fmt.Errorf("read OPF: %w", err)
	}

	var opf opfPackage
	if err := xml.Unmarshal(opfData, &opf); err != nil {
		return nil, "", fmt.Errorf("parse OPF: %w", err)
	}

	return &opf, opfPath, nil
}

func readZipFile(archive *zip.Reader, name string) ([]byte, error) {
	for _, file := range archive.File {
		if file.Name == name {
			reader, err := file.Open()
			if err != nil {
				return nil, err
			}
			defer reader.Close()
			return io.ReadAll(reader)
		}
	}
	return nil, fmt.Errorf("missing file in epub: %s", name)
}

func resolveEpubPath(opfPath, href string) string {
	base := path.Dir(opfPath)
	if base == "." {
		return href
	}
	return path.Join(base, href)
}

func firstNonEmpty(values []string) string {
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func nonEmptyStrings(values []string) []string {
	var result []string
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func extractSeries(values []opfMeta) string {
	for _, meta := range values {
		if meta.Name == "calibre:series" && meta.Content != "" {
			return strings.TrimSpace(meta.Content)
		}
		if meta.Property == "belongs-to-collection" {
			if s := strings.TrimSpace(meta.Value); s != "" {
				return s
			}
		}
	}
	return ""
}

func extractSeriesIndex(values []opfMeta) float64 {
	for _, meta := range values {
		if meta.Name == "calibre:series_index" && meta.Content != "" {
			if v, err := strconv.ParseFloat(strings.TrimSpace(meta.Content), 64); err == nil {
				return v
			}
		}
		if meta.Property == "group-position" {
			if v, err := strconv.ParseFloat(strings.TrimSpace(meta.Value), 64); err == nil {
				return v
			}
		}
	}
	return 0
}

func extractYear(values []string) int {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		for i := 0; i+4 <= len(trimmed); i++ {
			seg := trimmed[i : i+4]
			if isDigits(seg) {
				year, _ := strconv.Atoi(seg)
				if year >= 1000 && year <= 9999 {
					return year
				}
			}
		}
	}
	return 0
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

func extractISBNFromOPF(metadata opfMetadata) string {
	var candidates []string
	for _, ident := range metadata.Identifiers {
		if ident.Value != "" {
			candidates = append(candidates, ident.Value)
		}
	}
	for _, meta := range metadata.Meta {
		if meta.Name == "isbn" && meta.Content != "" {
			candidates = append(candidates, meta.Content)
		}
		if meta.Value != "" {
			candidates = append(candidates, meta.Value)
		}
	}
	return extractISBNFromCandidates(candidates)
}

func extractISBNFromCandidates(candidates []string) string {
	for _, c := range candidates {
		if isbn := normalizeISBN(c); isbn != "" {
			return isbn
		}
	}
	return ""
}

func normalizeISBN(value string) string {
	clean := strings.ToUpper(value)
	var b strings.Builder
	for _, r := range clean {
		if (r >= '0' && r <= '9') || r == 'X' {
			b.WriteRune(r)
		}
	}
	compact := b.String()
	if len(compact) == 13 && isValidISBN13(compact) {
		return compact
	}
	if len(compact) == 10 && isValidISBN10(compact) {
		return compact
	}
	return ""
}

func isValidISBN10(value string) bool {
	if len(value) != 10 {
		return false
	}
	sum := 0
	for i := 0; i < 9; i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
		sum += (10 - i) * int(value[i]-'0')
	}
	check := value[9]
	checkVal := 0
	if check == 'X' {
		checkVal = 10
	} else if check >= '0' && check <= '9' {
		checkVal = int(check - '0')
	} else {
		return false
	}
	sum += checkVal
	return sum%11 == 0
}

func isValidISBN13(value string) bool {
	if len(value) != 13 {
		return false
	}
	sum := 0
	for i := 0; i < 12; i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
		d := int(value[i] - '0')
		if i%2 == 0 {
			sum += d
		} else {
			sum += d * 3
		}
	}
	check := (10 - (sum % 10)) % 10
	return int(value[12]-'0') == check
}

func findCoverID(meta []opfMeta) string {
	for _, entry := range meta {
		if entry.Name == "cover" && entry.Content != "" {
			return strings.TrimSpace(entry.Content)
		}
	}
	return ""
}

func findCoverItem(items []opfItem, coverID string) *opfItem {
	for i := range items {
		if coverID != "" && items[i].ID == coverID {
			return &items[i]
		}
		if strings.Contains(items[i].Properties, "cover-image") {
			return &items[i]
		}
	}
	return nil
}

func coverExt(href, mediaType string) string {
	switch strings.ToLower(mediaType) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	}
	ext := strings.ToLower(path.Ext(href))
	if ext != "" {
		return ext
	}
	return ".jpg"
}

func isTextItem(mediaType string) bool {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "application/xhtml+xml", "text/html":
		return true
	}
	return false
}

func chapterTitle(href string) string {
	name := strings.TrimSuffix(path.Base(href), path.Ext(href))
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	words := strings.Fields(name)
	if len(words) == 0 {
		return "Chapter"
	}
	for i, word := range words {
		runes := []rune(word)
		runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

func cleanDescription(desc string) string {
	if desc == "" {
		return ""
	}
	// Strip HTML tags if present.
	text := html.UnescapeString(stripHTMLTagsWithBreaks(desc))
	return normalizeWhitespacePreserveLines(text)
}

// ── Text processing ─────────────────────────────────────────────────

func stripHTMLTagsWithBreaks(value string) string {
	var b strings.Builder
	inTag := false
	var tagName strings.Builder
	for i := 0; i < len(value); i++ {
		ch := value[i]
		switch ch {
		case '<':
			inTag = true
			tagName.Reset()
		case '>':
			inTag = false
			name := strings.ToLower(strings.Trim(tagName.String(), " /"))
			if name != "" && (isBlockTag(name) || name == "br") {
				b.WriteString("\n")
			}
		default:
			if inTag {
				if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '/' {
					tagName.WriteByte(ch)
				}
			} else {
				b.WriteByte(ch)
			}
		}
	}
	return b.String()
}

func isBlockTag(name string) bool {
	switch name {
	case "p", "div", "section", "article", "li", "ul", "ol",
		"h1", "h2", "h3", "h4", "h5", "h6":
		return true
	}
	return false
}

func normalizeWhitespacePreserveLines(value string) string {
	lines := strings.Split(value, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
			continue
		}
		out = append(out, strings.Join(strings.Fields(trimmed), " "))
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func extractEpubTextPreview(filePath string, maxChars int) (string, error) {
	archive, err := zip.OpenReader(filePath)
	if err != nil {
		return "", err
	}
	defer archive.Close()

	opf, opfPath, err := parseOPF(&archive.Reader)
	if err != nil {
		return "", err
	}

	manifest := map[string]opfItem{}
	for _, item := range opf.Manifest.Items {
		manifest[item.ID] = item
	}

	var builder strings.Builder
	for _, itemref := range opf.Spine.Itemrefs {
		item, ok := manifest[itemref.IDRef]
		if !ok || !isTextItem(item.MediaType) {
			continue
		}
		textPath := resolveEpubPath(opfPath, item.Href)
		data, err := readZipFile(&archive.Reader, textPath)
		if err != nil {
			continue
		}
		chunk := html.UnescapeString(stripHTMLTagsWithBreaks(string(data)))
		chunk = normalizeWhitespacePreserveLines(chunk)
		if chunk == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		remaining := maxChars - builder.Len()
		if remaining <= 0 {
			break
		}
		if len(chunk) > remaining {
			builder.WriteString(chunk[:remaining])
			break
		}
		builder.WriteString(chunk)
	}
	return builder.String(), nil
}
