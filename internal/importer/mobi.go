package importer

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
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

	records, err := parseEXTH(record0)
	if err != nil {
		return nil, fmt.Errorf("parse EXTH: %w", err)
	}

	ext := &Extracted{
		Title:  records[503],
		Format: "mobi",
	}

	if author := records[100]; author != "" {
		// MOBI may have multiple authors separated by ; or &
		for _, a := range strings.Split(author, ";") {
			a = strings.TrimSpace(a)
			if a != "" {
				ext.Authors = append(ext.Authors, a)
			}
		}
		if len(ext.Authors) == 0 {
			ext.Authors = []string{author}
		}
	}

	ext.ISBN = extractISBNFromMobi(records)

	// Extract cover.
	raw, err := parseEXTHRaw(record0)
	if err == nil {
		recordIndex := mobiCoverRecordIndex(raw)
		if recordIndex >= 0 {
			start, end, err := mobiRecordOffset(data, recordIndex)
			if err == nil && start < end && int(end) <= len(data) {
				cover := data[start:end]
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
		remaining := maxChars - builder.Len()
		if remaining <= 0 {
			break
		}
		if len(text) > remaining {
			builder.WriteString(text[:remaining])
			break
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

func parseEXTH(record0 []byte) (map[uint32]string, error) {
	raw, err := parseEXTHRaw(record0)
	if err != nil {
		return nil, err
	}
	records := make(map[uint32]string)
	for key, value := range raw {
		valueBytes := bytes.Trim(value, "\x00")
		text := strings.TrimSpace(string(valueBytes))
		if text == "" {
			continue
		}
		if _, exists := records[key]; !exists {
			records[key] = text
		}
	}
	return records, nil
}

func parseEXTHRaw(record0 []byte) (map[uint32][]byte, error) {
	records := make(map[uint32][]byte)
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
		valueBytes := record0[offset+8 : offset+recordLen]
		if _, exists := records[recordType]; !exists {
			records[recordType] = valueBytes
		}
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

func mobiCoverRecordIndex(records map[uint32][]byte) int {
	for _, key := range []uint32{201, 202} {
		raw, ok := records[key]
		if !ok || len(raw) < 4 {
			continue
		}
		index := int(binary.BigEndian.Uint32(raw[:4]))
		if index > 0 {
			return index
		}
	}
	return -1
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
