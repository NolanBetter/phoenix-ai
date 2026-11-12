package backend

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif" // decoders for image.Decode
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
	_ "golang.org/x/image/bmp"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Attachments are images, PDFs, Word documents and text or code files added
// to a message. Phoenix keeps its own copy in chats/<chat id>/, so a chat
// still works if the original file is moved or deleted. Images are shrunk
// and saved as JPEG; everything else is saved as the text Phoenix read from
// it, which is what the model gets.

const (
	maxAttachBytes      = 25 << 20 // per file, before processing
	maxImageSide        = 1280     // px; bigger images are scaled down
	maxStoredChars      = 400_000  // text kept per file
	MaxAttachments      = 10       // per message
	attachmentKindImage = "image"
)

// Attachment describes a file added to a message.
type Attachment struct {
	ID        string `json:"id"`   // stored file, e.g. "3f9ac1d27b6e0a44.jpg"
	Name      string `json:"name"` // the original file name
	Kind      string `json:"kind"` // image, pdf, doc, text, audio
	Size      int64  `json:"size"` // original size in bytes
	Chars     int    `json:"chars,omitempty"`
	Pages     int    `json:"pages,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Truncated bool   `json:"truncated,omitempty"` // the file was longer than Phoenix keeps

	// Audio files: ID is the transcript (.txt), Audio the sound itself.
	Audio        string  `json:"audio,omitempty"`
	Duration     float64 `json:"duration,omitempty"` // seconds
	Speakers     int     `json:"speakers,omitempty"` // people heard, when speaker detection is on
	Transcribing bool    `json:"transcribing,omitempty"`
}

const attachmentKindAudio = "audio"

// audioExts are the audio files Phoenix accepts, and the name each is kept as.
var audioExts = map[string]string{
	".mp3": ".mp3", ".wav": ".wav", ".m4a": ".m4a", ".mp4": ".m4a", ".aac": ".aac",
	".ogg": ".ogg", ".oga": ".ogg", ".opus": ".opus", ".flac": ".flac", ".weba": ".weba",
}

var attIDPattern = regexp.MustCompile(`^[a-f0-9]{16}\.(jpg|txt|png|webm|wav|mp3|m4a|ogg|opus|flac|aac|weba)$`)

// attContentType is how the window should treat each kind of file.
var attContentType = map[string]string{
	".jpg": "image/jpeg", ".png": "image/png", ".txt": "text/plain; charset=utf-8",
	".webm": "video/webm", ".wav": "audio/wav", ".mp3": "audio/mpeg", ".m4a": "audio/mp4",
	".ogg": "audio/ogg", ".opus": "audio/ogg", ".flac": "audio/flac", ".aac": "audio/aac", ".weba": "audio/webm",
}

// PickerPattern is the file picker's main filter. Any text file works,
// these are just the common ones.
var PickerPattern = "*." + strings.Join([]string{
	"png", "jpg", "jpeg", "gif", "webp", "bmp",
	"pdf", "docx", "txt", "md", "csv", "json", "xml", "yaml", "yml", "toml", "ini", "log",
	"html", "htm", "css", "scss", "js", "mjs", "jsx", "ts", "tsx", "vue", "svelte",
	"py", "ipynb", "go", "rs", "java", "kt", "c", "h", "cpp", "hpp", "cs", "php", "rb",
	"swift", "lua", "r", "sql", "sh", "bat", "ps1", "svg",
	"mp3", "wav", "m4a", "aac", "ogg", "opus", "flac",
}, ";*.")

// pendingAttachments are files added to the message box but not sent yet.
type pendingAttachments struct {
	mu    sync.Mutex
	items map[string]Attachment
}

// PickFiles opens Windows' file picker and returns the chosen paths.
func (a *App) PickFiles() ([]string, error) {
	if a.hooks.PickFiles == nil {
		return nil, errors.New("the file picker isn't available")
	}
	return a.hooks.PickFiles()
}

// AddAttachmentFromPath reads a file from disk (chosen in the picker or
// dropped on the window) and gets it ready to send.
func (a *App) AddAttachmentFromPath(path string) (Attachment, error) {
	name := filepath.Base(path)
	st, err := os.Stat(path)
	if err != nil {
		return Attachment{}, fmt.Errorf("couldn't open %s", name)
	}
	if st.IsDir() {
		return Attachment{}, fmt.Errorf("%s is a folder. Add the files inside it instead", name)
	}
	if st.Size() > maxAttachBytes {
		return Attachment{}, fmt.Errorf("%s is too big (%s). Files can be up to %s", name, humanBytes(st.Size()), humanBytes(maxAttachBytes))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Attachment{}, fmt.Errorf("couldn't read %s", name)
	}
	return a.ingest(name, data)
}

// AddAttachmentData adds a file the window already has in memory, like a
// pasted screenshot. data is base64, optionally as a data: URL.
func (a *App) AddAttachmentData(name, data string) (Attachment, error) {
	if i := strings.Index(data, ";base64,"); strings.HasPrefix(data, "data:") && i > 0 {
		data = data[i+len(";base64,"):]
	}
	if base64.StdEncoding.DecodedLen(len(data)) > maxAttachBytes+3 {
		return Attachment{}, fmt.Errorf("that file is too big. Files can be up to %s", humanBytes(maxAttachBytes))
	}
	b, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return Attachment{}, errors.New("couldn't read that file")
	}
	if strings.TrimSpace(name) == "" {
		name = "Pasted image.png"
	}
	return a.ingest(filepath.Base(name), b)
}

// RemovePendingAttachment drops a file from the message box before sending.
func (a *App) RemovePendingAttachment(id string) {
	if !attIDPattern.MatchString(id) {
		return
	}
	a.pending.mu.Lock()
	att, ok := a.pending.items[id]
	delete(a.pending.items, id)
	a.pending.mu.Unlock()
	os.Remove(filepath.Join(a.paths.Pending, id))
	if ok && att.Audio != "" && attIDPattern.MatchString(att.Audio) {
		os.Remove(filepath.Join(a.paths.Pending, att.Audio))
	}
}

var errNoSpeechForAudio = errors.New("to add audio files, Phoenix needs speech recognition. Get it on the Models page, under Voice")

func newAttachmentID(ext string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b) + ext
}

func (a *App) ingest(name string, data []byte) (Attachment, error) {
	if len(data) == 0 {
		return Attachment{}, fmt.Errorf("%s is empty", name)
	}
	att, stored, err := processAttachment(name, data)
	if err != nil {
		return Attachment{}, err
	}
	dest := att.ID
	if att.Kind == attachmentKindAudio {
		// The sound is kept to play back; the transcript comes later.
		if st := a.GetSpeechStatus(); !st.Listen {
			return Attachment{}, errNoSpeechForAudio
		}
		dest = att.Audio
	}
	if err := os.WriteFile(filepath.Join(a.paths.Pending, dest), stored, 0o644); err != nil {
		return Attachment{}, fmt.Errorf("couldn't save %s: %w", name, err)
	}
	a.pending.mu.Lock()
	a.pending.items[att.ID] = att
	a.pending.mu.Unlock()
	return att, nil
}

// processAttachment works out what kind of file this is and turns it into
// what Phoenix stores: a JPEG for images, UTF-8 text for everything else.
func processAttachment(name string, data []byte) (Attachment, []byte, error) {
	att := Attachment{Name: name, Size: int64(len(data))}
	ext := strings.ToLower(filepath.Ext(name))
	sniff := http.DetectContentType(data)

	switch {
	case audioExts[ext] != "" || strings.HasPrefix(sniff, "audio/"):
		keep := audioExts[ext]
		if keep == "" {
			keep = map[string]string{"audio/mpeg": ".mp3", "audio/wave": ".wav", "audio/wav": ".wav"}[strings.Split(sniff, ";")[0]]
		}
		if keep == "" {
			return att, nil, fmt.Errorf("Phoenix can't play %s. Try MP3, WAV, M4A, OGG or FLAC", name)
		}
		id := newAttachmentID("")
		att.Kind, att.ID, att.Audio, att.Transcribing = attachmentKindAudio, id+".txt", id+keep, true
		return att, data, nil

	case strings.HasPrefix(sniff, "image/") || ext == ".webp" || ext == ".bmp":
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return att, nil, fmt.Errorf("couldn't open the picture %s. Try saving it as PNG or JPEG", name)
		}
		out, w, h, err := shrinkToJPEG(img)
		if err != nil {
			return att, nil, err
		}
		att.Kind, att.ID, att.Width, att.Height = attachmentKindImage, newAttachmentID(".jpg"), w, h
		return att, out, nil

	case bytes.HasPrefix(data, []byte("%PDF")):
		text, pages, err := pdfText(data)
		if err != nil {
			return att, nil, fmt.Errorf("couldn't read %s: %v", name, err)
		}
		if strings.TrimSpace(text) == "" {
			return att, nil, fmt.Errorf("%s has no text Phoenix can read. It may be scanned pages. Try adding screenshots of the pages instead", name)
		}
		att.Kind, att.Pages = "pdf", pages
		return finishText(att, text)

	case ext == ".docx" && bytes.HasPrefix(data, []byte("PK\x03\x04")):
		text, err := docxText(data)
		if err != nil {
			return att, nil, fmt.Errorf("couldn't read %s: %v", name, err)
		}
		att.Kind = "doc"
		return finishText(att, text)

	default:
		text, ok := decodeText(data)
		if !ok {
			return att, nil, fmt.Errorf("Phoenix can't read %s. It works with pictures, PDFs, Word documents (.docx), and text or code files", name)
		}
		att.Kind = "text"
		return finishText(att, text)
	}
}

func finishText(att Attachment, text string) (Attachment, []byte, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if utf8.RuneCountInString(text) > maxStoredChars {
		r := []rune(text)
		text = string(r[:maxStoredChars])
		att.Truncated = true
	}
	att.ID = newAttachmentID(".txt")
	att.Chars = utf8.RuneCountInString(text)
	return att, []byte(text), nil
}

// shrinkToJPEG scales an image down so its longest side is at most
// maxImageSide, puts it on white (JPEG has no transparency) and encodes it.
func shrinkToJPEG(src image.Image) ([]byte, int, int, error) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, 0, 0, errors.New("that picture is empty")
	}
	if w > maxImageSide || h > maxImageSide {
		if w >= h {
			h = max(1, h*maxImageSide/w)
			w = maxImageSide
		} else {
			w = max(1, w*maxImageSide/h)
			h = maxImageSide
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, xdraw.Src)
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 88}); err != nil {
		return nil, 0, 0, err
	}
	return buf.Bytes(), w, h, nil
}

// pdfText pulls the text out of a PDF, page by page.
func pdfText(data []byte) (text string, pages int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("the PDF is damaged or uses features Phoenix can't read")
		}
	}()
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "encrypt") {
			return "", 0, errors.New("the PDF is password-protected")
		}
		return "", 0, errors.New("the PDF is damaged or uses features Phoenix can't read")
	}
	pages = r.NumPage()
	var sb strings.Builder
	for i := 1; i <= pages; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		t, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		fmt.Fprintf(&sb, "[Page %d]\n%s", i, t)
		if sb.Len() > maxStoredChars*4 {
			break
		}
	}
	return sb.String(), pages, nil
}

// docxText reads the paragraphs of a Word document.
func docxText(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", errors.New("the document is damaged")
	}
	var doc *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			doc = f
		}
	}
	if doc == nil {
		return "", errors.New("it doesn't look like a Word document")
	}
	rc, err := doc.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	dec := xml.NewDecoder(io.LimitReader(rc, 64<<20))
	var sb strings.Builder
	inText := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", errors.New("the document is damaged")
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				inText = true
			case "tab":
				sb.WriteString("\t")
			case "br", "cr":
				sb.WriteString("\n")
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				sb.WriteString("\n")
			}
		case xml.CharData:
			if inText {
				sb.Write(t)
			}
		}
	}
	text := strings.TrimSpace(sb.String())
	if text == "" {
		return "", errors.New("the document has no text")
	}
	return text, nil
}

// decodeText accepts UTF-8 (with or without a byte-order mark) and UTF-16
// text, and refuses binary files.
func decodeText(data []byte) (string, bool) {
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		data = data[3:]
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}), bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		le := data[0] == 0xFF
		data = data[2:]
		u := make([]uint16, len(data)/2)
		for i := range u {
			if le {
				u[i] = uint16(data[2*i]) | uint16(data[2*i+1])<<8
			} else {
				u[i] = uint16(data[2*i])<<8 | uint16(data[2*i+1])
			}
		}
		return string(utf16.Decode(u)), true
	}
	head := data
	if len(head) > 8192 {
		head = head[:8192]
	}
	if bytes.IndexByte(head, 0) >= 0 {
		return "", false
	}
	if !utf8.Valid(head) {
		// Allow a cut-off character at the end of the sample.
		trimmed := head
		for i := 0; i < 3 && len(trimmed) > 0 && !utf8.Valid(trimmed); i++ {
			trimmed = trimmed[:len(trimmed)-1]
		}
		if !utf8.Valid(trimmed) {
			return "", false
		}
	}
	return strings.ToValidUTF8(string(data), "�"), true
}

// takePending moves files from the message box into a chat's folder.
func (a *App) takePending(chatID string, ids []string) ([]Attachment, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) > MaxAttachments {
		return nil, fmt.Errorf("you can add up to %d files to one message", MaxAttachments)
	}
	a.pending.mu.Lock()
	defer a.pending.mu.Unlock()
	out := make([]Attachment, 0, len(ids))
	for _, id := range ids {
		att, ok := a.pending.items[id]
		if !ok || !attIDPattern.MatchString(id) {
			return nil, errors.New("one of the files is no longer there. Remove it and add it again")
		}
		if att.Transcribing {
			return nil, fmt.Errorf("%s is still being written down. Wait a moment and send again", att.Name)
		}
		out = append(out, att)
	}
	dir := filepath.Join(a.paths.Chats, chatID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	for _, att := range out {
		if err := os.Rename(filepath.Join(a.paths.Pending, att.ID), filepath.Join(dir, att.ID)); err != nil {
			return nil, fmt.Errorf("couldn't save %s: %w", att.Name, err)
		}
		if att.Audio != "" && attIDPattern.MatchString(att.Audio) {
			_ = os.Rename(filepath.Join(a.paths.Pending, att.Audio), filepath.Join(dir, att.Audio))
		}
		delete(a.pending.items, att.ID)
	}
	return out, nil
}

// pendingAttachment returns a file in the message box, for checks before sending.
func (a *App) pendingAttachment(id string) (Attachment, bool) {
	a.pending.mu.Lock()
	defer a.pending.mu.Unlock()
	att, ok := a.pending.items[id]
	return att, ok
}

// cleanupPending clears out files left in the message box by an earlier run.
func cleanupPending(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		os.Remove(filepath.Join(dir, e.Name()))
	}
}

// AttachmentHandler serves attachment files to the window at
// /att/<chat id>/<file> and /att/pending/<file>. Wails sends it every
// request that isn't one of the app's own files.
func AttachmentHandler(a *App) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		rest, ok := strings.CutPrefix(r.URL.Path, "/att/")
		parts := strings.Split(rest, "/")
		if !ok || len(parts) != 2 || !attIDPattern.MatchString(parts[1]) {
			http.NotFound(w, r)
			return
		}
		var dir string
		switch {
		case parts[0] == "pending":
			dir = a.paths.Pending
		case parts[0] == "voice":
			dir = a.paths.Voice
		case chatIDPattern.MatchString(parts[0]):
			dir = filepath.Join(a.paths.Chats, parts[0])
		default:
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(filepath.Join(dir, parts[1]))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		ctype := attContentType[filepath.Ext(parts[1])]
		if ctype == "" {
			ctype = "application/octet-stream"
		}
		w.Header().Set("Content-Type", ctype)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "private, max-age=86400")
		http.ServeContent(w, r, "", st.ModTime(), f)
	})
}

// readAttachmentText and readAttachmentImage load a sent file for the prompt.
func (a *App) readAttachmentText(chatID string, att Attachment) string {
	b, err := os.ReadFile(filepath.Join(a.paths.Chats, chatID, att.ID))
	if err != nil {
		return "(Phoenix couldn't find this file anymore.)"
	}
	return string(b)
}

func (a *App) readAttachmentImage(chatID string, att Attachment) (string, bool) {
	b, err := os.ReadFile(filepath.Join(a.paths.Chats, chatID, att.ID))
	if err != nil {
		return "", false
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(b), true
}
