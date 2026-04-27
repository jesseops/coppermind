package web

import (
	"testing"
	"time"
)

func TestSendStoreGenerateAndGet(t *testing.T) {
	ss := newSendStore()

	code, err := ss.generate("Mozilla/5.0 Kobo")
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != keyLength {
		t.Errorf("code length = %d, want %d", len(code), keyLength)
	}

	sk := ss.get(code)
	if sk == nil {
		t.Fatal("expected to find key")
	}
	if sk.agent != "Mozilla/5.0 Kobo" {
		t.Errorf("agent = %q", sk.agent)
	}

	// Non-existent key.
	if ss.get("ZZZZ") != nil {
		t.Error("expected nil for non-existent key")
	}
}

func TestSendStoreExpiry(t *testing.T) {
	ss := newSendStore()

	code, _ := ss.generate("Test Agent")

	// Key should exist.
	if ss.get(code) == nil {
		t.Fatal("key should exist immediately after generation")
	}

	// Manually remove.
	ss.remove(code)
	if ss.get(code) != nil {
		t.Error("key should be gone after remove")
	}
}

func TestSendStoreKeepAlive(t *testing.T) {
	ss := newSendStore()
	code, _ := ss.generate("Test Agent")

	sk := ss.get(code)
	oldAlive := sk.alive

	time.Sleep(10 * time.Millisecond)
	ss.keepAlive(code)

	sk2 := ss.get(code)
	if !sk2.alive.After(oldAlive) {
		t.Error("keepAlive should update alive time")
	}
}

func TestSendStoreSetFile(t *testing.T) {
	ss := newSendStore()
	code, _ := ss.generate("Mozilla/5.0 Kobo")

	ss.setFile(code, &sendFile{Name: "test.epub", Path: "/tmp/test.epub"})

	f := ss.getFile(code, "Mozilla/5.0 Kobo")
	if f == nil {
		t.Fatal("expected file")
	}
	if f.Name != "test.epub" {
		t.Errorf("Name = %q", f.Name)
	}

	// Wrong agent should not return file.
	f2 := ss.getFile(code, "Different Agent")
	if f2 != nil {
		t.Error("expected nil for wrong agent")
	}
}

func TestSendStoreReplaceFile(t *testing.T) {
	ss := newSendStore()
	code, _ := ss.generate("Agent")

	ss.setFile(code, &sendFile{Name: "first.epub", Path: "/tmp/nonexistent1"})
	ss.setFile(code, &sendFile{Name: "second.epub", Path: "/tmp/nonexistent2"})

	f := ss.getFile(code, "Agent")
	if f == nil || f.Name != "second.epub" {
		t.Errorf("expected second file, got %+v", f)
	}
}

func TestDetectDevice(t *testing.T) {
	tests := []struct {
		ua   string
		want deviceType
	}{
		{"Mozilla/5.0 (Linux; U; Android 4.4.2) AppleWebKit/537.36 Kobo Touch", deviceKobo},
		{"Mozilla/5.0 (X11; U; Linux armv7l like Android; en-us) AppleWebKit/531.2+ (KHTML, like Gecko) Kindle/3.0+", deviceKindle},
		{"Mozilla/5.0 Tolino", deviceTolino},
		{"Mozilla/5.0 (compatible; eReader)", deviceTolino},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120", deviceUnknown},
	}
	for _, tt := range tests {
		got := detectDevice(tt.ua)
		if got != tt.want {
			t.Errorf("detectDevice(%q) = %v, want %v", tt.ua, got, tt.want)
		}
	}
}

func TestSanitizeFilenameForKindle(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"Book Title.epub", "Book_Title.epub"},
		{"Ünïcödé Bøøk.epub", "_n_c_d__B__k.epub"},
		{"simple.mobi", "simple.mobi"},
		{"has spaces and (parens).epub", "has_spaces_and_(parens).epub"},
	}
	for _, tt := range tests {
		got := sanitizeFilenameForKindle(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeFilenameForKindle(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRandomCodeUniqueness(t *testing.T) {
	codes := make(map[string]bool)
	for i := 0; i < 100; i++ {
		code := randomCode()
		if len(code) != keyLength {
			t.Errorf("code length = %d", len(code))
		}
		codes[code] = true
	}
	// With 29^4 = 707,281 possible codes, 100 should all be unique.
	if len(codes) < 95 {
		t.Errorf("expected mostly unique codes, got %d unique out of 100", len(codes))
	}
}
