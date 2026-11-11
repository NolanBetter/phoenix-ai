package backend

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

// makePDF writes a small but valid PDF with one line of text per page.
func makePDF(pages ...string) []byte {
	var b bytes.Buffer
	offsets := []int{}
	obj := func(body string) {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", len(offsets), body)
	}
	b.WriteString("%PDF-1.4\n")
	kids := []string{}
	for i := range pages {
		kids = append(kids, fmt.Sprintf("%d 0 R", 4+i*2))
	}
	obj("<< /Type /Catalog /Pages 2 0 R >>")
	obj(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pages)))
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	for i, text := range pages {
		obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", 5+i*2))
		stream := fmt.Sprintf("BT /F1 12 Tf 72 720 Td (%s) Tj ET", text)
		obj(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, o := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xref)
	return b.Bytes()
}

func makeDocx(paragraphs ...string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("word/document.xml")
	fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	for _, p := range paragraphs {
		fmt.Fprintf(w, `<w:p><w:r><w:t>%s</w:t></w:r></w:p>`, p)
	}
	fmt.Fprint(w, `</w:body></w:document>`)
	zw.Close()
	return buf.Bytes()
}

func makePNG(w, h int, transparent bool) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.NRGBA{R: uint8(x), G: uint8(y), B: 90, A: 255}
			if transparent && x < w/2 {
				c.A = 0
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func TestProcessAttachment(t *testing.T) {
	// A big picture is shrunk to 1280 px on its longest side and becomes a JPEG.
	att, out, err := processAttachment("photo.png", makePNG(3000, 2000, true))
	if err != nil {
		t.Fatal(err)
	}
	if att.Kind != "image" || att.Width != 1280 || att.Height != 853 || !strings.HasSuffix(att.ID, ".jpg") || !attIDPattern.MatchString(att.ID) {
		t.Fatalf("image: %+v", att)
	}
	img, format, err := image.Decode(bytes.NewReader(out))
	if err != nil || format != "jpeg" || img.Bounds().Dx() != 1280 {
		t.Fatalf("stored image: %v %s", err, format)
	}
	// Transparent parts turn white, not black.
	if r, g, b, _ := img.At(10, 400).RGBA(); r>>8 < 240 || g>>8 < 240 || b>>8 < 240 {
		t.Errorf("transparent area should be white, got %d %d %d", r>>8, g>>8, b>>8)
	}
	// A small picture keeps its size.
	if att, _, _ := processAttachment("icon.png", makePNG(64, 40, false)); att.Width != 64 || att.Height != 40 {
		t.Errorf("small image: %+v", att)
	}

	att, out, err = processAttachment("notes.pdf", makePDF("Hello from page one", "Second page here"))
	if err != nil {
		t.Fatal(err)
	}
	if att.Kind != "pdf" || att.Pages != 2 || !strings.Contains(string(out), "[Page 1]") || !strings.Contains(string(out), "Hello from page one") || !strings.Contains(string(out), "[Page 2]\nSecond page here") {
		t.Fatalf("pdf: %+v %q", att, out)
	}
	if _, _, err := processAttachment("broken.pdf", []byte("%PDF-1.4 nonsense")); err == nil || !strings.Contains(err.Error(), "damaged") {
		t.Errorf("broken pdf: %v", err)
	}

	att, out, err = processAttachment("essay.docx", makeDocx("First paragraph.", "Second &amp; last."))
	if err != nil || att.Kind != "doc" || string(out) != "First paragraph.\nSecond & last." {
		t.Fatalf("docx: %v %+v %q", err, att, out)
	}

	att, out, err = processAttachment("main.go", []byte("package main\r\n\r\nfunc main() {}\r\n"))
	if err != nil || att.Kind != "text" || string(out) != "package main\n\nfunc main() {}\n" || att.Chars != len(out) {
		t.Fatalf("code: %v %+v %q", err, att, out)
	}

	u := utf16.Encode([]rune("héllo wörld"))
	le := []byte{0xFF, 0xFE}
	for _, c := range u {
		le = append(le, byte(c), byte(c>>8))
	}
	if _, out, err := processAttachment("notes.txt", le); err != nil || string(out) != "héllo wörld" {
		t.Errorf("utf-16: %v %q", err, out)
	}
	if _, out, err := processAttachment("bom.txt", append([]byte{0xEF, 0xBB, 0xBF}, "hi"...)); err != nil || string(out) != "hi" {
		t.Errorf("bom: %v %q", err, out)
	}

	if _, _, err := processAttachment("program.exe", []byte("MZ\x90\x00\x03\x00\x00\x00")); err == nil || !strings.Contains(err.Error(), "can't read") {
		t.Errorf("binary should be refused: %v", err)
	}
	if _, _, err := processAttachment("fake.png", []byte("\x89PNG\r\n\x1a\nnot really")); err == nil {
		t.Error("broken picture should be refused")
	}

	att, out, _ = processAttachment("big.txt", []byte(strings.Repeat("é", maxStoredChars+10)))
	if !att.Truncated || att.Chars != maxStoredChars || len([]rune(string(out))) != maxStoredChars {
		t.Errorf("long text: %+v", att)
	}
}

type fakeFiles map[string]string

func (f fakeFiles) attachmentText(att Attachment) string { return f[att.ID] }
func (f fakeFiles) attachmentImage(att Attachment) (string, bool) {
	v, ok := f[att.ID]
	return v, ok
}

func TestBuildPromptWithFiles(t *testing.T) {
	img := Attachment{ID: "aaaaaaaaaaaaaaaa.jpg", Name: "cat.jpg", Kind: "image", Width: 1280, Height: 960}
	doc := Attachment{ID: "bbbbbbbbbbbbbbbb.txt", Name: "notes.md", Kind: "text"}
	files := fakeFiles{img.ID: "data:image/jpeg;base64,AAAA", doc.ID: "# Notes\nbuy milk"}
	msgs := []Message{{Role: "user", Content: "What's in these?", Attachments: []Attachment{img, doc}}}

	p := buildPrompt("", msgs, 8192, true, files)
	parts, ok := p[0].Content.([]contentPart)
	if !ok || len(parts) != 2 {
		t.Fatalf("expected picture + text parts, got %#v", p[0].Content)
	}
	if parts[0].Type != "image_url" || parts[0].ImageURL.URL != "data:image/jpeg;base64,AAAA" {
		t.Errorf("picture part: %+v", parts[0])
	}
	want := "<file name=\"notes.md\">\n# Notes\nbuy milk\n</file>\n\nWhat's in these?"
	if parts[1].Type != "text" || parts[1].Text != want {
		t.Errorf("text part: %q", parts[1].Text)
	}

	// A model that can't see gets a note instead of the picture.
	p = buildPrompt("", msgs, 8192, false, files)
	text, ok := p[0].Content.(string)
	if !ok || !strings.Contains(text, `can't see pictures`) || !strings.Contains(text, "cat.jpg") || !strings.HasSuffix(text, "What's in these?") {
		t.Errorf("no-vision prompt: %#v", p[0].Content)
	}

	// Only pictures, no words.
	p = buildPrompt("", []Message{{Role: "user", Attachments: []Attachment{img}}}, 8192, true, files)
	if parts, ok := p[0].Content.([]contentPart); !ok || len(parts) != 1 {
		t.Errorf("picture only: %#v", p[0].Content)
	}

	// A file too long for the model's memory is cut, and the model is told.
	huge := Attachment{ID: "cccccccccccccccc.txt", Name: "log.txt", Kind: "text"}
	small := Attachment{ID: "dddddddddddddddd.txt", Name: "small.txt", Kind: "text"}
	files[huge.ID] = strings.Repeat("x", 200_000)
	files[small.ID] = "tiny file"
	p = buildPrompt("", []Message{{Role: "user", Content: "summarize", Attachments: []Attachment{huge, small}}}, 4096, true, files)
	text = p[0].Content.(string)
	if len(text) > int(float64(4096-1536)*charsPerToken)+200 {
		t.Errorf("prompt should fit, got %d chars", len(text))
	}
	if !strings.Contains(text, "tiny file") || !strings.Contains(text, "Phoenix cut the rest") || !strings.HasSuffix(text, "summarize") {
		t.Errorf("cut prompt: %q…", text[:200])
	}

	// Old turns with pictures are dropped first when the chat gets long.
	var long []Message
	for i := 0; i < 6; i++ {
		long = append(long, Message{Role: "user", Content: fmt.Sprint("look ", i), Attachments: []Attachment{img}}, Message{Role: "assistant", Content: "ok"})
	}
	long = append(long, Message{Role: "user", Content: "last one"})
	p = buildPrompt("", long, 4096, true, files)
	if len(p) >= len(long) || p[len(p)-1].Content != "last one" || p[0].Role != "user" {
		t.Errorf("long chat kept %d of %d", len(p), len(long))
	}
}

func TestFitFiles(t *testing.T) {
	fs := []promptFile{{text: strings.Repeat("a", 1000)}, {text: "short"}, {text: strings.Repeat("é", 500)}}
	fitFiles(fs, 605)
	total := 0
	for _, f := range fs {
		total += len(f.text)
		if !strings.HasPrefix(f.text, "a") && !strings.HasPrefix(f.text, "s") && !strings.HasPrefix(f.text, "é") && f.text != "" {
			t.Errorf("bad cut %q", f.text)
		}
	}
	if total > 605 || fs[1].text != "short" || fs[1].cut || !fs[0].cut || !fs[2].cut {
		t.Errorf("fit: total %d, %+v", total, fs)
	}
	if !strings.HasSuffix(fs[2].text, "é") || len(fs[2].text)%2 != 0 {
		t.Errorf("cut in the middle of a character: %q", fs[2].text[len(fs[2].text)-3:])
	}
}

func TestAttachmentsFlow(t *testing.T) {
	a, rec, model := setupApp(t)
	pic := base64.StdEncoding.EncodeToString(makePNG(800, 600, false))

	// A model without image support refuses pictures before anything is sent.
	att, err := a.AddAttachmentData("screenshot.png", "data:image/png;base64,"+pic)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.paths.Pending, att.ID)); err != nil {
		t.Fatal("picture should wait in pending")
	}
	if _, err := a.SendMessage("", "what is this", model, false, []string{att.ID}); err == nil || !strings.Contains(err.Error(), "can't see pictures") {
		t.Fatalf("expected no-vision error, got %v", err)
	}
	if len(a.ListChats()) != 0 {
		t.Fatal("no chat should be made when sending fails")
	}

	// Give the model image support and send again.
	mp, _ := a.modelPath(model)
	os.WriteFile(filepath.Join(filepath.Dir(mp), "mmproj-F16.gguf"), append([]byte("GGUF"), make([]byte, 64)...), 0o644)
	if m := a.GetInstalledModels(); len(m) != 1 || !m[0].Vision {
		t.Fatalf("model should see images now: %+v", m)
	}

	src := filepath.Join(t.TempDir(), "app.js")
	os.WriteFile(src, []byte("console.log('hi')\n"), 0o644)
	code, err := a.AddAttachmentFromPath(src)
	if err != nil || code.Kind != "text" || code.Name != "app.js" {
		t.Fatalf("code file: %v %+v", err, code)
	}

	n := rec.count()
	chatID, err := a.SendMessage("", "what is this", model, false, []string{att.ID, code.ID})
	if err != nil {
		t.Fatal(err)
	}
	d := rec.waitDone(t, n)
	if d.Message.Error != "" || !strings.Contains(d.Message.Content, "1 pictures then 1 files then what is this") {
		t.Fatalf("answer: %+v", d.Message)
	}
	if st := a.GetModelStatus(); !st.Vision {
		t.Errorf("status should say the model sees images: %+v", st)
	}
	ch, _ := a.GetChat(chatID)
	if len(ch.Messages[0].Attachments) != 2 || ch.Messages[0].Attachments[0].Name != "screenshot.png" {
		t.Fatalf("attachments not saved: %+v", ch.Messages[0])
	}
	for _, id := range []string{att.ID, code.ID} {
		if _, err := os.Stat(filepath.Join(a.paths.Chats, chatID, id)); err != nil {
			t.Errorf("%s should be in the chat folder", id)
		}
		if _, err := os.Stat(filepath.Join(a.paths.Pending, id)); err == nil {
			t.Errorf("%s should have left pending", id)
		}
	}
	// The window can show them.
	h := AttachmentHandler(a)
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	if w := get("/att/" + chatID + "/" + att.ID); w.Code != 200 || w.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("serve picture: %d %s", w.Code, w.Header().Get("Content-Type"))
	}
	if w := get("/att/" + chatID + "/" + code.ID); w.Code != 200 || w.Body.String() != "console.log('hi')\n" || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/plain") {
		t.Errorf("serve text: %d %q", w.Code, w.Body.String())
	}
	for _, bad := range []string{"/att/../settings.json", "/att/" + chatID + "/../" + chatID + ".json", "/att/pending/x.jpg", "/att/" + chatID + "/" + att.ID + "/x", "/settings.json"} {
		if w := get(bad); w.Code != http.StatusNotFound {
			t.Errorf("%s should be refused, got %d", bad, w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/att/"+chatID+"/"+att.ID, nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST should be refused, got %d", w.Code)
	}

	// Files only, no words: the chat is named after the file.
	doc, _ := a.AddAttachmentData("Report.docx", base64.StdEncoding.EncodeToString(makeDocx("Quarterly numbers.")))
	n = rec.count()
	id2, err := a.SendMessage("", "", model, false, []string{doc.ID})
	if err != nil {
		t.Fatal(err)
	}
	rec.waitDone(t, n)
	if ch, _ := a.GetChat(id2); ch.Title != "Report.docx" {
		t.Errorf("title = %q", ch.Title)
	}
	if _, err := a.SendMessage("", "", model, false, nil); err == nil {
		t.Error("an empty message should be refused")
	}
	if _, err := a.SendMessage("", "x", model, false, []string{"0123456789abcdef.jpg"}); err == nil {
		t.Error("an unknown file should be refused")
	}

	// Deleting the chat deletes its files.
	if err := a.DeleteChat(chatID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.paths.Chats, chatID)); !os.IsNotExist(err) {
		t.Error("chat folder should be gone")
	}

	// Removing a file from the message box deletes it.
	p, _ := a.AddAttachmentData("a.txt", base64.StdEncoding.EncodeToString([]byte("hello")))
	a.RemovePendingAttachment(p.ID)
	if _, err := os.Stat(filepath.Join(a.paths.Pending, p.ID)); !os.IsNotExist(err) {
		t.Error("removed file should be deleted")
	}
}

func TestImageSupportFallback(t *testing.T) {
	t.Setenv("FAKE_FAIL_MMPROJ", "1")
	a, rec, model := setupApp(t)
	mp, _ := a.modelPath(model)
	os.WriteFile(filepath.Join(filepath.Dir(mp), "mmproj-F16.gguf"), []byte("GGUF"), 0o644)

	// Text still works when the image support won't load.
	n := rec.count()
	chatID, err := a.SendMessage("", "hello", model, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d := rec.waitDone(t, n); d.Message.Error != "" {
		t.Fatalf("text should still work: %+v", d.Message)
	}
	if st := a.GetModelStatus(); st.Vision || !strings.Contains(st.Note, "image support couldn't load") {
		t.Errorf("status: %+v", st)
	}
	// A picture gets a clear error instead of a confused answer.
	pic, _ := a.AddAttachmentData("p.png", base64.StdEncoding.EncodeToString(makePNG(50, 50, false)))
	n = rec.count()
	if _, err := a.SendMessage(chatID, "see?", model, false, []string{pic.ID}); err != nil {
		t.Fatal(err)
	}
	if d := rec.waitDone(t, n); !strings.Contains(d.Message.Error, "image support didn't load") {
		t.Errorf("expected image support error, got %+v", d.Message)
	}
}

func TestNotifications(t *testing.T) {
	a, rec, model := setupApp(t)
	type shown struct{ title, body, target string }
	got := make(chan shown, 4)
	old := showNotification
	showNotification = func(_ *App, title, body, target string) error {
		got <- shown{title, body, target}
		return nil
	}
	t.Cleanup(func() { showNotification = old })

	// In front: no notification.
	a.SetWindowActive(true)
	n := rec.count()
	chatID, _ := a.SendMessage("", "hello", model, false, nil)
	rec.waitDone(t, n)

	// Away: a notification that opens the chat.
	a.SetWindowActive(false)
	n = rec.count()
	a.SendMessage(chatID, "again", model, false, nil)
	rec.waitDone(t, n)
	select {
	case s := <-got:
		if s.title != "New answer in “hello”" || s.target != "chat:"+chatID || !strings.HasPrefix(s.body, "You said again and there are") {
			t.Errorf("notification: %+v", s)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no notification")
	}

	// Turned off in Settings: nothing.
	s := a.GetSettings()
	s.Notifications = false
	a.SaveSettings(s)
	n = rec.count()
	a.SendMessage(chatID, "once more", model, false, nil)
	rec.waitDone(t, n)
	select {
	case s := <-got:
		t.Errorf("notifications are off but got %+v", s)
	default:
	}
	if len(got) != 0 {
		t.Error("only one notification expected")
	}

	// Clicking one brings the window forward and opens the chat.
	focused := false
	SetHooks(a, Hooks{Focus: func() { focused = true }})
	n = rec.count()
	a.onNotificationClick("chat:" + chatID)
	if !focused {
		t.Error("window should come forward")
	}
	rec.mu.Lock()
	ok := false
	for _, e := range rec.events[n:] {
		if e.name == "notify:open" && e.data == "chat:"+chatID {
			ok = true
		}
	}
	rec.mu.Unlock()
	if !ok {
		t.Error("expected notify:open event")
	}

	if p := previewText("## Title\n\nHere is **bold** and `code` and [a link](http://x).", 200); p != "Title Here is bold and code and a link." {
		t.Errorf("preview = %q", p)
	}
	if p := previewText(strings.Repeat("word ", 50), 30); !strings.HasSuffix(p, "…") || len([]rune(p)) > 31 {
		t.Errorf("preview = %q", p)
	}
}
