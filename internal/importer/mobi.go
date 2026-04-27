package importer

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// EXTH record type constants per MobileRead wiki spec.
const (
	exthAuthor         = 100 // dc:Creator — can appear multiple times
	exthPublisher      = 101 // dc:Publisher
	exthDescription    = 103 // dc:Description
	exthISBN           = 104 // dc:Identifier scheme='ISBN'
	exthSubject        = 105 // dc:Subject — can appear multiple times
	exthPublishingDate = 106 // dc:Date (publication date)
	exthContributor    = 108 // dc:Contributor
	exthCoverOffset    = 201 // offset from first image record
	exthThumbOffset    = 202 // offset from first image record
	exthUpdatedTitle   = 503 // overrides PalmDB/MOBI full name
	exthLanguage       = 524 // dc:language
)

// ExtractMobiMetadata extracts metadata and cover from a MOBI file.
func ExtractMobiMetadata(filePath string) (*Extracted, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read mobi: %w", err)
	}

	record0, err := mobiRecord0(data)
	if err != nil {
		return nil, fmt.Errorf("mobi record0: %w", err)
	}

	records, allByType, err := parseEXTH(record0)
	if err != nil {
		return nil, fmt.Errorf("parse EXTH: %w", err)
	}

	ext := &Extracted{
		Format: "mobi",
	}

	// ── Title: EXTH 503 > MOBI full name > PalmDB name ──
	ext.Title = records[exthUpdatedTitle]
	if ext.Title == "" {
		ext.Title = mobiFullName(record0)
	}

	// ── Authors: all EXTH 100 records, split on ; and & ──
	for _, raw := range allByType[exthAuthor] {
		for _, a := range strings.FieldsFunc(raw, func(r rune) bool { return r == ';' || r == '&' }) {
			a = strings.TrimSpace(a)
			if a != "" {
				ext.Authors = append(ext.Authors, a)
			}
		}
	}

	// Old Calibre versions sometimes swap EXTH 100 (author) and 503 (title).
	primaryAuthor := ""
	if len(ext.Authors) > 0 {
		primaryAuthor = ext.Authors[0]
	}
	ext.Title, primaryAuthor = fixMobiAuthorTitleSwap(ext.Title, primaryAuthor)
	if len(ext.Authors) > 0 && primaryAuthor != ext.Authors[0] {
		ext.Authors = []string{primaryAuthor}
	} else if len(ext.Authors) == 0 && primaryAuthor != "" {
		ext.Authors = []string{primaryAuthor}
	}

	// ── Publisher: EXTH 101 ──
	ext.Publisher = records[exthPublisher]

	// ── Description: EXTH 103 ──
	ext.Description = cleanDescription(records[exthDescription])

	// ── ISBN: EXTH 104 first, then scan all records for ISBN pattern ──
	if isbn := normalizeISBN(records[exthISBN]); isbn != "" {
		ext.ISBN = isbn
	} else {
		ext.ISBN = extractISBNFromMobi(records)
	}

	// ── Published date: EXTH 106 ──
	if dateStr := records[exthPublishingDate]; dateStr != "" {
		ext.PublishedYear = extractYear([]string{dateStr})
	}

	// ── Language: EXTH 524 ──
	ext.Language = records[exthLanguage]

	// ── Subjects: all EXTH 105 records ──
	for _, raw := range allByType[exthSubject] {
		s := strings.TrimSpace(raw)
		if s != "" {
			ext.Subjects = append(ext.Subjects, s)
		}
	}

	// ── Cover image ──
	// EXTH 201/202 stores the cover image offset relative to the first image record.
	rawRecords, _ := parseEXTHRaw(record0)
	firstImageRecord := mobiFirstImageRecord(record0)
	coverOffset := mobiCoverOffset(rawRecords)
	if firstImageRecord >= 0 && coverOffset >= 0 {
		recordIndex := firstImageRecord + coverOffset
		start, end, err := mobiRecordOffset(data, recordIndex)
		if err == nil && start < end && int(end) <= len(data) {
			cover := data[start:end]
			if isImageData(cover) {
				ext.CoverData = cover
				ext.CoverExt = detectImageExt(cover)
			}
		}
	}

	return ext, nil
}

// ExtractMobiTextPreview returns a plain-text preview of MOBI content.
func ExtractMobiTextPreview(filePath string, maxChars int) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	record0, err := mobiRecord0(data)
	if err != nil {
		return "", err
	}
	if len(record0) < 16 {
		return "", nil
	}

	compression := binary.BigEndian.Uint16(record0[0:2])
	textRecords := binary.BigEndian.Uint16(record0[8:10])
	if textRecords == 0 {
		return "", nil
	}

	var builder strings.Builder
	for i := 1; i <= int(textRecords); i++ {
		start, end, err := mobiRecordOffset(data, i)
		if err != nil || start >= end || int(end) > len(data) {
			continue
		}
		chunk := data[start:end]
		var decoded []byte
		switch compression {
		case 1:
			decoded = chunk
		case 2:
			decoded = decompressPalmDoc(chunk)
		default:
			continue
		}
		text := normalizeWhitespacePreserveLines(string(decoded))
		if text == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		if maxChars > 0 {
			remaining := maxChars - builder.Len()
			if remaining <= 0 {
				break
			}
			if len(text) > remaining {
				builder.WriteString(text[:remaining])
				break
			}
		}
		builder.WriteString(text)
	}
	return builder.String(), nil
}

// ── MOBI binary parsing ─────────────────────────────────────────────

func mobiRecord0(data []byte) ([]byte, error) {
	if len(data) < 78 {
		return nil, fmt.Errorf("mobi file too small")
	}
	numRecords := binary.BigEndian.Uint16(data[76:78])
	if numRecords == 0 {
		return nil, fmt.Errorf("mobi file has no records")
	}
	record0Offset := binary.BigEndian.Uint32(data[78:82])
	record1Offset := uint32(len(data))
	if numRecords > 1 {
		record1Offset = binary.BigEndian.Uint32(data[86:90])
	}
	if record0Offset >= uint32(len(data)) || record1Offset > uint32(len(data)) || record1Offset <= record0Offset {
		return nil, fmt.Errorf("invalid mobi record offsets")
	}
	return data[record0Offset:record1Offset], nil
}

func parseEXTH(record0 []byte) (map[uint32]string, map[uint32][]string, error) {
	raw, err := parseEXTHRaw(record0)
	if err != nil {
		return nil, nil, err
	}
	records := make(map[uint32]string)
	allByType := make(map[uint32][]string)
	for key, values := range raw {
		for _, value := range values {
			valueBytes := bytes.Trim(value, "\x00")
			text := strings.TrimSpace(string(valueBytes))
			if text == "" {
				continue
			}
			allByType[key] = append(allByType[key], text)
			if _, exists := records[key]; !exists {
				records[key] = text
			}
		}
	}
	return records, allByType, nil
}

func parseEXTHRaw(record0 []byte) (map[uint32][][]byte, error) {
	records := make(map[uint32][][]byte)
	start := bytes.Index(record0, []byte("EXTH"))
	if start == -1 {
		return records, nil
	}
	if start+12 > len(record0) {
		return records, fmt.Errorf("invalid EXTH header")
	}

	headerLen := int(binary.BigEndian.Uint32(record0[start+4 : start+8]))
	count := int(binary.BigEndian.Uint32(record0[start+8 : start+12]))
	end := start + headerLen
	if end > len(record0) {
		end = len(record0)
	}

	offset := start + 12
	for i := 0; i < count && offset+8 <= end; i++ {
		recordType := binary.BigEndian.Uint32(record0[offset : offset+4])
		recordLen := int(binary.BigEndian.Uint32(record0[offset+4 : offset+8]))
		if recordLen < 8 || offset+recordLen > end {
			break
		}
		valueBytes := make([]byte, recordLen-8)
		copy(valueBytes, record0[offset+8:offset+recordLen])
		records[recordType] = append(records[recordType], valueBytes)
		offset += recordLen
	}

	return records, nil
}

func mobiRecordOffset(data []byte, index int) (uint32, uint32, error) {
	if len(data) < 78 {
		return 0, 0, fmt.Errorf("mobi file too small")
	}
	numRecords := int(binary.BigEndian.Uint16(data[76:78]))
	if index < 0 || index >= numRecords {
		return 0, 0, fmt.Errorf("invalid record index")
	}
	base := 78 + index*8
	if base+8 > len(data) {
		return 0, 0, fmt.Errorf("invalid record table")
	}
	start := binary.BigEndian.Uint32(data[base : base+4])
	var end uint32
	if index+1 < numRecords {
		next := 78 + (index+1)*8
		if next+4 > len(data) {
			return 0, 0, fmt.Errorf("invalid record table")
		}
		end = binary.BigEndian.Uint32(data[next : next+4])
	} else {
		end = uint32(len(data))
	}
	return start, end, nil
}

// mobiFullName reads the "full name" field from the MOBI header in record 0.
// This is stored at the offset/length specified at bytes 84-91 of record 0.
func mobiFullName(record0 []byte) string {
	if len(record0) < 92 {
		return ""
	}
	nameOffset := binary.BigEndian.Uint32(record0[84:88])
	nameLen := binary.BigEndian.Uint32(record0[88:92])
	if nameLen == 0 || int(nameOffset+nameLen) > len(record0) {
		return ""
	}
	return strings.TrimSpace(string(record0[nameOffset : nameOffset+nameLen]))
}

// fixMobiAuthorTitleSwap detects and corrects cases where EXTH 100 (author)
// and EXTH 503 (title) are swapped, or where the title field contains a
// combined "Author - Title" string (common in old Calibre conversions).
func fixMobiAuthorTitleSwap(title, author string) (string, string) {
	if title == "" && author == "" {
		return title, author
	}

	// Case 1: Title contains "Author - Title" pattern.
	if parts := strings.SplitN(title, " - ", 2); len(parts) == 2 {
		candidateAuthor := strings.TrimSpace(parts[0])
		candidateTitle := strings.TrimSpace(parts[1])

		if candidateAuthor != "" && candidateTitle != "" {
			if looksLikePersonName(candidateAuthor) && !looksLikePersonName(author) {
				return candidateTitle, candidateAuthor
			}
		}
	}

	// Case 2: Title is numeric/very short and author looks like a title.
	// Common in old Calibre where EXTH 503 gets a series number and EXTH 100 gets the title.
	if looksLikeSeriesNumber(title) && author != "" && !looksLikeSeriesNumber(author) {
		// The "title" is just a number — the author is probably the actual title.
		// We can't recover the author, so use the author field as title and clear author.
		return author, ""
	}

	// Case 3: Author and title appear simply swapped.
	if looksLikePersonName(title) && !looksLikePersonName(author) && author != "" {
		return author, title
	}

	// Case 4: Author field contains common title words (prepositions etc.)
	// but the title field doesn't look like a person name either.
	if author != "" && looksLikeBookTitle(author) && !looksLikeBookTitle(title) && !looksLikePersonName(author) {
		return author, ""
	}

	return title, author
}

// looksLikeSeriesNumber returns true if the string is purely numeric,
// a Roman numeral, or a very short non-name string (e.g., "115", "IV", "Book 3").
func looksLikeSeriesNumber(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	// Purely numeric.
	allDigits := true
	for _, r := range s {
		if r < '0' || r > '9' {
			allDigits = false
			break
		}
	}
	if allDigits {
		return true
	}
	// Very short (1-3 chars) — likely a code/number, not a title.
	if len(s) <= 3 {
		return true
	}
	return false
}

// looksLikePersonName returns true if the string looks like a person's name.
func looksLikePersonName(s string) bool {
	words := strings.Fields(s)
	if len(words) < 2 || len(words) > 5 {
		return false
	}
	// "Last, First" format is definitively a name.
	if strings.Contains(s, ",") {
		return true
	}
	// Names don't start with articles.
	first := strings.ToLower(words[0])
	if first == "a" || first == "an" || first == "the" {
		return false
	}
	// Names don't contain common English prepositions/conjunctions.
	for _, w := range words {
		lower := strings.ToLower(w)
		if isTitleWord(lower) {
			return false
		}
		if len(w) > 20 {
			return false
		}
	}
	return true
}

// looksLikeBookTitle returns true if the string contains words commonly
// found in book titles but not in person names.
func looksLikeBookTitle(s string) bool {
	for _, w := range strings.Fields(s) {
		if isTitleWord(strings.ToLower(w)) {
			return true
		}
	}
	return false
}

// isTitleWord returns true for common English words found in titles but not names.
func isTitleWord(lower string) bool {
	switch lower {
	case "at", "in", "of", "the", "and", "or", "to", "for", "from",
		"with", "on", "by", "into", "through", "over", "under",
		"between", "about", "against", "during", "before", "after":
		return true
	}
	return false
}

func mobiCoverOffset(records map[uint32][][]byte) int {
	for _, key := range []uint32{201, 202} {
		raws, ok := records[key]
		if !ok || len(raws) == 0 {
			continue
		}
		raw := raws[0]
		if len(raw) < 4 {
			continue
		}
		offset := int(binary.BigEndian.Uint32(raw[:4]))
		if offset >= 0 {
			return offset
		}
	}
	return -1
}

// mobiFirstImageRecord reads the "first image record" index from the MOBI
// header at byte offset 108 within record 0.
func mobiFirstImageRecord(record0 []byte) int {
	// MOBI header starts at offset 16 in record 0 (after PalmDOC header).
	// The "first image index" field is at MOBI header offset 108,
	// which is record0 offset 16 + 108 = 124... but this varies.
	// A more reliable approach: it's at absolute offset 108 in record 0
	// for MOBI headers that are long enough.
	if len(record0) < 112 {
		return -1
	}
	return int(binary.BigEndian.Uint32(record0[108:112]))
}

// isImageData checks the magic bytes to verify data is an actual image.
func isImageData(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	// JPEG
	if data[0] == 0xFF && data[1] == 0xD8 {
		return true
	}
	// PNG
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return true
	}
	// GIF
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 {
		return true
	}
	// BMP
	if data[0] == 0x42 && data[1] == 0x4D {
		return true
	}
	// WebP (RIFF....WEBP)
	if len(data) >= 12 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
		data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
		return true
	}
	return false
}

func decompressPalmDoc(data []byte) []byte {
	output := make([]byte, 0, len(data)*2)
	for i := 0; i < len(data); i++ {
		c := data[i]
		switch {
		case c == 0x00:
			// skip
		case c >= 0x01 && c <= 0x08:
			count := int(c)
			if i+count >= len(data) {
				return output
			}
			output = append(output, data[i+1:i+1+count]...)
			i += count
		case c >= 0x09 && c <= 0x7f:
			output = append(output, c)
		case c >= 0x80 && c <= 0xbf:
			if i+1 >= len(data) {
				return output
			}
			b := int(c)<<8 | int(data[i+1])
			i++
			off := (b >> 3) & 0x7ff
			length := (b & 0x7) + 3
			start := len(output) - off
			if start < 0 {
				start = 0
			}
			for j := 0; j < length; j++ {
				if start+j >= len(output) {
					break
				}
				output = append(output, output[start+j])
			}
		default:
			output = append(output, ' ', c^0x80)
		}
	}
	return output
}

func extractISBNFromMobi(records map[uint32]string) string {
	candidates := make([]string, 0, len(records))
	for _, v := range records {
		candidates = append(candidates, v)
	}
	return extractISBNFromCandidates(candidates)
}

func detectImageExt(data []byte) string {
	if len(data) >= 4 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4e && data[3] == 0x47 {
		return ".png"
	}
	if len(data) >= 2 {
		if data[0] == 0xff && data[1] == 0xd8 {
			return ".jpg"
		}
		if data[0] == 0x47 && data[1] == 0x49 {
			return ".gif"
		}
	}
	return ".jpg"
}
