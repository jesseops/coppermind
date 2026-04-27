package web

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pgaskin/kepubify/v4/kepub"
)

// ── Key store ───────────────────────────────────────────────────────

const (
	keyChars      = "23456789ACDEFGHJKLMNPRSTUVWXYZ"
	keyLength     = 4
	keyExpireTime = 30 * time.Second
	keyMaxLife    = 1 * time.Hour
)

type sendKey struct {
	mu        sync.Mutex
	key       string
	agent     string
	created   time.Time
	alive     time.Time
	file      *sendFile
	expiryTmr *time.Timer
}

type sendFile struct {
	Name string
	Path string // temp file on disk
}

type sendStore struct {
	mu   sync.Mutex
	keys map[string]*sendKey
}

func newSendStore() *sendStore {
	return &sendStore{keys: make(map[string]*sendKey)}
}

func (ss *sendStore) generate(agent string) (string, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	var code string
	for attempts := 0; attempts < 100; attempts++ {
		code = randomCode()
		if _, exists := ss.keys[code]; !exists {
			break
		}
		code = ""
	}
	if code == "" {
		return "", fmt.Errorf("unable to generate unique code")
	}

	sk := &sendKey{
		key:     code,
		agent:   agent,
		created: time.Now(),
		alive:   time.Now(),
	}
	sk.expiryTmr = time.AfterFunc(keyExpireTime, func() { ss.remove(code) })

	// Hard max lifetime.
	time.AfterFunc(keyMaxLife, func() { ss.remove(code) })

	ss.keys[code] = sk
	slog.Debug("send: generated key", "key", code, "agent", agent)
	return code, nil
}

func (ss *sendStore) get(code string) *sendKey {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return ss.keys[strings.ToUpper(code)]
}

func (ss *sendStore) keepAlive(code string) {
	sk := ss.get(code)
	if sk == nil {
		return
	}
	sk.mu.Lock()
	defer sk.mu.Unlock()
	sk.alive = time.Now()
	sk.expiryTmr.Reset(keyExpireTime)
}

func (ss *sendStore) remove(code string) {
	ss.mu.Lock()
	sk, ok := ss.keys[strings.ToUpper(code)]
	if !ok {
		ss.mu.Unlock()
		return
	}
	delete(ss.keys, strings.ToUpper(code))
	ss.mu.Unlock()

	sk.mu.Lock()
	defer sk.mu.Unlock()
	sk.expiryTmr.Stop()
	if sk.file != nil {
		os.Remove(sk.file.Path)
		slog.Debug("send: removed file", "key", code, "path", sk.file.Path)
		sk.file = nil
	}
	slog.Debug("send: removed key", "key", code)
}

func (ss *sendStore) setFile(code string, f *sendFile) {
	sk := ss.get(code)
	if sk == nil {
		return
	}
	sk.mu.Lock()
	defer sk.mu.Unlock()
	// Remove old file if present.
	if sk.file != nil {
		os.Remove(sk.file.Path)
	}
	sk.file = f
	sk.expiryTmr.Reset(keyExpireTime)
}

func (ss *sendStore) getFile(code, agent string) *sendFile {
	sk := ss.get(code)
	if sk == nil {
		return nil
	}
	sk.mu.Lock()
	defer sk.mu.Unlock()
	// Verify User-Agent matches.
	if sk.agent != agent {
		return nil
	}
	return sk.file
}

func randomCode() string {
	var b strings.Builder
	for i := 0; i < keyLength; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(keyChars))))
		b.WriteByte(keyChars[n.Int64()])
	}
	return b.String()
}

// ── Device detection ────────────────────────────────────────────────

type deviceType int

const (
	deviceUnknown deviceType = iota
	deviceKobo
	deviceKindle
	deviceTolino
)

func detectDevice(ua string) deviceType {
	switch {
	case strings.Contains(ua, "Kobo"):
		return deviceKobo
	case strings.Contains(ua, "Kindle"):
		return deviceKindle
	case strings.Contains(ua, "Tolino"), strings.Contains(ua, "tolino"), strings.Contains(ua, "eReader"):
		return deviceTolino
	default:
		return deviceUnknown
	}
}

func (d deviceType) String() string {
	switch d {
	case deviceKobo:
		return "Kobo"
	case deviceKindle:
		return "Kindle"
	case deviceTolino:
		return "Tolino"
	default:
		return "Unknown"
	}
}

func (d deviceType) IsEReader() bool {
	return d != deviceUnknown
}

// ── KEPUB conversion ────────────────────────────────────────────────

func convertToKepub(epubPath string) (string, error) {
	f, err := os.Open(epubPath)
	if err != nil {
		return "", err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return "", err
	}
	zr, err := zip.NewReader(f, fi.Size())
	if err != nil {
		f.Close()
		return "", fmt.Errorf("open epub zip: %w", err)
	}

	outPath := strings.TrimSuffix(epubPath, filepath.Ext(epubPath)) + ".kepub.epub"
	outFile, err := os.Create(outPath)
	if err != nil {
		f.Close()
		return "", err
	}

	converter := kepub.NewConverter()
	err = converter.Convert(context.Background(), outFile, zr)
	f.Close()
	outFile.Close()

	if err != nil {
		os.Remove(outPath)
		return "", fmt.Errorf("kepubify: %w", err)
	}

	return outPath, nil
}

// sanitizeFilenameForKindle strips special chars from filename for Kindle browser compat.
func sanitizeFilenameForKindle(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.', r == '-', r == '_', r == '\'', r == '"', r == '(', r == ')':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// ── Handlers ────────────────────────────────────────────────────────

// handleReceivePage serves the e-reader-facing page.
// Auto-detects e-reader User-Agent; non-e-readers see a help page.
func (s *Server) handleReceivePage(w http.ResponseWriter, r *http.Request) {
	device := detectDevice(r.UserAgent())
	s.render(w, r, "receive.html", templateData{
		"IsEReader":  device.IsEReader(),
		"DeviceType": device.String(),
	})
}

// handleReceiveGenerate creates a new receive code for an e-reader.
func (s *Server) handleReceiveGenerate(w http.ResponseWriter, r *http.Request) {
	code, err := s.sends.generate(r.UserAgent())
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(code))
}

// handleReceiveStatus is the polling endpoint for the e-reader.
func (s *Server) handleReceiveStatus(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "key")
	sk := s.sends.get(code)
	if sk == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "unknown key"})
		return
	}

	// Validate User-Agent.
	if sk.agent != r.UserAgent() {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	s.sends.keepAlive(code)

	sk.mu.Lock()
	var fileInfo map[string]string
	if sk.file != nil {
		fileInfo = map[string]string{"name": sk.file.Name}
	}
	sk.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"file": fileInfo,
	})
}

// handleReceiveDownload serves the file to the e-reader.
func (s *Server) handleReceiveDownload(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "key")
	sf := s.sends.getFile(code, r.UserAgent())
	if sf == nil {
		http.NotFound(w, r)
		return
	}
	s.sends.keepAlive(code)

	device := detectDevice(r.UserAgent())
	filename := sf.Name
	if device == deviceKindle {
		filename = sanitizeFilenameForKindle(filename)
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	http.ServeFile(w, r, sf.Path)
}

// handleSendWithCode sends an edition's file to a receive code.
// POST /receive/send/{edition_id} with form field "code" and optional "kepubify".
func (s *Server) handleSendWithCode(w http.ResponseWriter, r *http.Request) {
	editionID, _ := strconv.ParseInt(chi.URLParam(r, "edition_id"), 10, 64)
	code := strings.ToUpper(strings.TrimSpace(r.FormValue("code")))
	doKepubify := r.FormValue("kepubify") == "on" || r.FormValue("kepubify") == "true"

	edition, err := s.store.GetEdition(editionID)
	if err != nil {
		respondSendError(w, r, "Edition not found")
		return
	}

	sk := s.sends.get(code)
	if sk == nil {
		respondSendError(w, r, "Invalid or expired code. Make sure the receive page is open on your e-reader.")
		return
	}

	// Determine target device type.
	device := detectDevice(sk.agent)

	// Copy file to temp location.
	srcPath := edition.FilePath
	filename := filepath.Base(srcPath)

	tmpDir, err := os.MkdirTemp("", "coppermind-send-*")
	if err != nil {
		respondSendError(w, r, "Server error creating temp directory")
		return
	}

	destPath := filepath.Join(tmpDir, filename)

	// Convert if needed.
	isEpub := strings.EqualFold(filepath.Ext(srcPath), ".epub")
	if isEpub && device == deviceKobo && doKepubify {
		// Copy to temp first, then convert.
		if err := copyFileForSend(srcPath, destPath); err != nil {
			os.RemoveAll(tmpDir)
			respondSendError(w, r, "Failed to prepare file")
			return
		}
		kepubPath, err := convertToKepub(destPath)
		if err != nil {
			os.RemoveAll(tmpDir)
			respondSendError(w, r, fmt.Sprintf("KEPUB conversion failed: %v", err))
			return
		}
		os.Remove(destPath) // remove original copy
		destPath = kepubPath
		filename = strings.TrimSuffix(filename, filepath.Ext(filename)) + ".kepub.epub"
	} else {
		if err := copyFileForSend(srcPath, destPath); err != nil {
			os.RemoveAll(tmpDir)
			respondSendError(w, r, "Failed to prepare file")
			return
		}
	}

	if device == deviceKindle {
		filename = sanitizeFilenameForKindle(filename)
	}

	s.sends.setFile(code, &sendFile{
		Name: filename,
		Path: destPath,
	})

	work, _ := s.store.GetWork(edition.WorkID)
	title := ""
	if work != nil {
		title = work.Title
	}

	slog.Info("send: file sent to device",
		"key", code,
		"device", device.String(),
		"edition_id", editionID,
		"filename", filename,
		"kepubify", doKepubify,
	)

	msg := fmt.Sprintf("Sent \"%s\" to %s device. The download should appear on the e-reader shortly.", title, device.String())
	if !device.IsEReader() {
		msg = fmt.Sprintf("Sent \"%s\" to device. The download should appear on the e-reader shortly.", title)
	}

	respondSendSuccess(w, r, msg)
}

func respondSendError(w http.ResponseWriter, r *http.Request, msg string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `<div class="error" style="padding:0.5rem;">%s</div>`, msg)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func respondSendSuccess(w http.ResponseWriter, r *http.Request, msg string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<div style="padding:0.5rem;color:#16a34a;">✓ %s</div>`, msg)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": msg})
}

func copyFileForSend(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Sync()
}
