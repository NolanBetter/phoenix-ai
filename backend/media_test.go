package backend

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// buildFake compiles one of the stand-in engines in testdata.
func buildFake(t *testing.T, name string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), name)
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	out, err := exec.Command("go", "build", "-o", bin, "./testdata/"+name).CombinedOutput()
	if err != nil {
		t.Fatalf("build %s: %v\n%s", name, err, out)
	}
	return bin
}

// placeModel puts a model file where a download would have.
func placeModel(t *testing.T, a *App, repo, file string, data []byte) string {
	t.Helper()
	p, ok := safeJoin(a.paths.Models, modelID(repo, file))
	if !ok {
		t.Fatal("bad path")
	}
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func fakeSafetensors() []byte {
	head := []byte(`{"__metadata__":{}}`)
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(len(head)))
	return append(append(b, head...), 0, 0, 0, 0)
}

func fakeGGUF() []byte { return append([]byte("GGUF"), make([]byte, 64)...) }

// installPictures sets up SDXL and its decoder with the fake engine.
func installPictures(t *testing.T, a *App) string {
	t.Helper()
	m, _ := findCatalog("sdxl")
	placeModel(t, a, m.Repo, m.File, fakeSafetensors())
	for _, p := range m.Parts {
		placeModel(t, a, p.Repo, p.File, fakeSafetensors())
	}
	return modelID(m.Repo, m.File)
}

func installVideo(t *testing.T, a *App) string {
	t.Helper()
	m, _ := findCatalog("wan2.2-5b")
	placeModel(t, a, m.Repo, m.File, fakeGGUF())
	for _, p := range m.Parts {
		data := fakeSafetensors()
		if strings.HasSuffix(p.File, ".gguf") {
			data = fakeGGUF()
		}
		placeModel(t, a, p.Repo, p.File, data)
	}
	return modelID(m.Repo, m.File)
}

func (r *recorder) eventsNamed(name string) []any {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []any
	for _, e := range r.events {
		if e.name == name {
			out = append(out, e.data)
		}
	}
	return out
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(string(b), "\n")
}

func argAfter(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func TestCatalogKindsAndParts(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range catalog {
		if seen[m.ID] {
			t.Errorf("duplicate catalog id %s", m.ID)
		}
		seen[m.ID] = true
		if !repoPattern.MatchString(m.Repo) || !validModelFile(m.File) {
			t.Errorf("%s: bad repo or file", m.ID)
		}
		for _, p := range m.Parts {
			if !repoPattern.MatchString(p.Repo) || !validModelFile(p.File) || p.Size <= 0 || p.Name == "" {
				t.Errorf("%s: bad part %+v", m.ID, p)
			}
		}
		if key := toolForKind(m.kindOf()); key != "" {
			if _, ok := mustTool(key).Assets["windows/amd64"]; !ok {
				t.Errorf("%s: no Windows engine", m.ID)
			}
		}
	}
	sdxl, _ := findCatalog("sdxl")
	if sdxl.kindOf() != KindImage || sdxl.totalSize() != sdxl.Size+sdxl.Parts[0].Size {
		t.Errorf("sdxl: %+v", sdxl)
	}
	if k, _ := findCatalog("kokoro"); k.totalSize() != k.Archive.Size || countsTowardLimit(k.kindOf()) {
		t.Error("the voice shouldn't count toward the limit")
	}
	if !countsTowardLimit(KindVideo) || countsTowardLimit(KindListen) {
		t.Error("limit rules")
	}
	if _, ok := findCatalog("qwen3-coder-next"); !ok {
		t.Error("coding model missing")
	}
	for _, kind := range []string{KindImage, KindVideo, KindListen, KindSpeakers, KindVoice} {
		if _, ok := catalogByKind(kind); !ok {
			t.Errorf("no %s model", kind)
		}
	}
	if m, p, ok := partOf("city96/umt5-xxl-encoder-gguf", "umt5-xxl-encoder-Q4_K_M.gguf"); !ok || m.ID != "wan2.2-5b" || p.Role != "text_encoder" {
		t.Error("partOf")
	}
	if u := hubFileURL("nvidia/x", "a/b c.gguf", "abc123"); u != hubBase+"/nvidia/x/resolve/abc123/a/b%20c.gguf?download=true" {
		t.Errorf("url %s", u)
	}
}

func TestInstalledKindsAndLimit(t *testing.T) {
	a, _, chatModel := setupApp(t)
	t.Setenv("PHOENIX_SD_SERVER", buildFake(t, "fakesd"))
	pic := installPictures(t, a)
	asr, _ := findCatalog("nemotron-asr")
	placeModel(t, a, asr.Repo, asr.File, fakeGGUF())

	got := map[string]InstalledModel{}
	for _, m := range a.GetInstalledModels() {
		got[m.ID] = m
	}
	if len(got) != 3 {
		t.Fatalf("expected chat, picture and speech models, got %+v", got)
	}
	if m := got[pic]; m.Kind != KindImage || !m.Ready || m.Name != "Stable Diffusion XL" {
		t.Errorf("picture model: %+v", m)
	}
	if m := got[modelID(asr.Repo, asr.File)]; m.Kind != KindListen || m.Ready || m.Missing != "Speech engine" {
		t.Errorf("speech model without its engine: %+v", m)
	}
	if got[chatModel].Kind != KindChat {
		t.Error("chat model kind")
	}
	a.dl.mu.Lock()
	used := a.modelSlotsUsedLocked("")
	a.dl.mu.Unlock()
	if used != 2 {
		t.Errorf("chat + picture count, speech and parts don't: got %d", used)
	}

	// Deleting the picture model takes its decoder too.
	sdxl, _ := findCatalog("sdxl")
	vae, _ := safeJoin(a.paths.Models, modelID(sdxl.Parts[0].Repo, sdxl.Parts[0].File))
	if err := a.DeleteModel(pic); err != nil {
		t.Fatal(err)
	}
	if fileExists(vae) {
		t.Error("the decoder should be deleted with its model")
	}
}

func TestMakePicture(t *testing.T) {
	a, rec, chatModel := setupApp(t)
	dir := t.TempDir()
	argsFile, bodyFile := filepath.Join(dir, "args"), filepath.Join(dir, "body")
	t.Setenv("PHOENIX_SD_SERVER", buildFake(t, "fakesd"))
	t.Setenv("FAKE_SD_ARGS", argsFile)
	t.Setenv("FAKE_SD_BODY", bodyFile)
	model := installPictures(t, a)

	// A picture model doesn't chat.
	if _, err := a.SendMessage("", "hi", model, false, nil); err == nil || !strings.Contains(err.Error(), "doesn't chat") {
		t.Errorf("expected doesn't-chat error, got %v", err)
	}
	if _, err := a.MakeMedia("", "a fox", chatModel, MediaOptions{}, nil); err == nil {
		t.Error("a chat model can't make pictures")
	}

	n := rec.count()
	chatID, err := a.MakeMedia("", "A red fox in the snow, watercolor", model, MediaOptions{Shape: "wide"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d := rec.waitDone(t, n)
	if d.Message.Error != "" || len(d.Message.Media) != 1 {
		t.Fatalf("result: %+v", d.Message)
	}
	media := d.Message.Media[0]
	if media.Kind != KindImage || media.Shape != "wide" || media.Width != 1216/16 || !strings.HasSuffix(media.ID, ".png") {
		t.Errorf("media: %+v", media)
	}
	if !fileExists(filepath.Join(a.paths.Chats, chatID, media.ID)) {
		t.Error("picture not saved in the chat folder")
	}
	args := readLines(t, argsFile)
	sdxl, _ := findCatalog("sdxl")
	if !strings.HasSuffix(argAfter(args, "-m"), sdxl.File) || !strings.HasSuffix(argAfter(args, "--vae"), "sdxl_vae.safetensors") || argAfter(args, "--listen-ip") != "127.0.0.1" {
		t.Errorf("args: %v", args)
	}
	var body map[string]any
	b, _ := os.ReadFile(bodyFile)
	json.Unmarshal(b, &body)
	if body["prompt"] != "A red fox in the snow, watercolor" || body["width"].(float64) != 1216 || body["height"].(float64) != 832 || body["preview"] != "proj" {
		t.Errorf("request: %v", body)
	}
	// Progress came through, with a preview.
	sawPreview := false
	for _, e := range rec.eventsNamed("media:progress") {
		if p, ok := e.(MediaProgress); ok && p.Preview != "" && p.Total == 4 && p.Label == "Making your picture" {
			sawPreview = true
		}
	}
	if !sawPreview {
		t.Error("expected progress with a preview")
	}
	ch, _ := a.GetChat(chatID)
	if len(ch.Messages) != 2 || ch.Title != "A red fox in the snow, watercolor" || len(ch.Messages[1].Media) != 1 {
		t.Fatalf("chat: %+v", ch)
	}
	// The window can show it.
	w := httptest.NewRecorder()
	AttachmentHandler(a).ServeHTTP(w, httptest.NewRequest("GET", "/att/"+chatID+"/"+media.ID, nil))
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" {
		t.Errorf("serve picture: %d %s", w.Code, w.Header().Get("Content-Type"))
	}

	// Make another: same description and shape, new picture.
	n = rec.count()
	if err := a.Regenerate(chatID, model, false); err != nil {
		t.Fatal(err)
	}
	d = rec.waitDone(t, n)
	ch, _ = a.GetChat(chatID)
	if len(ch.Messages) != 4 || ch.Messages[2].Content != ch.Messages[0].Content || d.Message.Media[0].ID == media.ID || d.Message.Media[0].Shape != "wide" {
		t.Errorf("make another: %+v", ch.Messages)
	}

	// A chat model later in the same chat hears about the picture.
	n = rec.count()
	if _, err := a.SendMessage(chatID, "describe it", chatModel, false, nil); err != nil {
		t.Fatal(err)
	}
	if d := rec.waitDone(t, n); d.Message.Error != "" {
		t.Fatalf("chat after picture: %+v", d.Message)
	}
	p := buildPrompt("", ch.Messages, 8192, false, nil)
	if !strings.Contains(fmt.Sprint(p[1].Content), "I made a picture") {
		t.Errorf("prompt: %+v", p)
	}

	// Saving asks where, then copies.
	dest := filepath.Join(dir, "saved.png")
	SetHooks(a, Hooks{SaveFile: func(name string) (string, error) {
		if !strings.HasSuffix(name, ".png") {
			t.Errorf("suggested name %q", name)
		}
		return dest, nil
	}})
	if got, err := a.SaveMedia(chatID, media.ID); err != nil || got != dest || !fileExists(dest) {
		t.Errorf("save: %v %s", err, got)
	}
	if _, err := a.SaveMedia(chatID, "../../settings.json"); err == nil {
		t.Error("bad names must be refused")
	}
}

func TestMakeVideoFromPicture(t *testing.T) {
	a, rec, _ := setupApp(t)
	dir := t.TempDir()
	argsFile, bodyFile := filepath.Join(dir, "args"), filepath.Join(dir, "body")
	t.Setenv("PHOENIX_SD_SERVER", buildFake(t, "fakesd"))
	t.Setenv("FAKE_SD_ARGS", argsFile)
	t.Setenv("FAKE_SD_BODY", bodyFile)
	model := installVideo(t, a)

	pic, err := a.AddAttachmentData("cat.png", base64.StdEncoding.EncodeToString(makePNG(600, 900, false)))
	if err != nil {
		t.Fatal(err)
	}
	n := rec.count()
	chatID, err := a.MakeMedia("", "the cat waves hello", model, MediaOptions{Length: 2}, []string{pic.ID})
	if err != nil {
		t.Fatal(err)
	}
	d := rec.waitDone(t, n)
	if d.Message.Error != "" || len(d.Message.Media) != 1 {
		t.Fatalf("result: %+v", d.Message)
	}
	m := d.Message.Media[0]
	// A tall picture makes a tall video; 2 seconds at 24 fps is 49 frames (4n+1).
	if m.Kind != KindVideo || m.Shape != "tall" || m.Frames != 49 || m.FPS != 24 || !strings.HasSuffix(m.ID, ".webm") {
		t.Errorf("video: %+v", m)
	}
	var body map[string]any
	b, _ := os.ReadFile(bodyFile)
	json.Unmarshal(b, &body)
	if !strings.HasPrefix(fmt.Sprint(body["init_image"]), "data:image/jpeg;base64,") || body["width"].(float64) != 480 || body["height"].(float64) != 832 || body["video_frames"].(float64) != 49 || body["output_format"] != "webm" {
		t.Errorf("request: %v", body)
	}
	args := readLines(t, argsFile)
	if !strings.HasSuffix(argAfter(args, "--t5xxl"), "umt5-xxl-encoder-Q4_K_M.gguf") || !strings.HasSuffix(argAfter(args, "--vae"), "wan2.2_vae.safetensors") || argAfter(args, "--flow-shift") != "3" {
		t.Errorf("args: %v", args)
	}
	w := httptest.NewRecorder()
	AttachmentHandler(a).ServeHTTP(w, httptest.NewRequest("GET", "/att/"+chatID+"/"+m.ID, nil))
	if w.Code != 200 || w.Header().Get("Content-Type") != "video/webm" {
		t.Errorf("serve video: %d %s", w.Code, w.Header().Get("Content-Type"))
	}
	// Only a picture can start a video.
	doc, _ := a.AddAttachmentData("a.txt", base64.StdEncoding.EncodeToString([]byte("hi")))
	if _, err := a.MakeMedia(chatID, "x", model, MediaOptions{}, []string{doc.ID}); err == nil {
		t.Error("a text file can't start a video")
	}
	if videoFrames(5) != 121 || videoFrames(0) != 49 || videoFrames(9) != 121 {
		t.Error("frame counts")
	}
}

func TestMediaStopAndFallback(t *testing.T) {
	a, rec, _ := setupApp(t)
	t.Setenv("PHOENIX_SD_SERVER", buildFake(t, "fakesd"))
	t.Setenv("FAKE_SD_SLOW", "1")
	argsFile := filepath.Join(t.TempDir(), "args")
	t.Setenv("FAKE_SD_ARGS", argsFile)
	t.Setenv("FAKE_SD_FAIL_GPU", "1")
	model := installPictures(t, a)

	n := rec.count()
	if _, err := a.MakeMedia("", "a lighthouse", model, MediaOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	// Wait until it's working, then stop it.
	deadline := time.Now().Add(20 * time.Second)
	for len(rec.eventsNamed("media:progress")) < 3 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	a.StopGeneration()
	d := rec.waitDone(t, n)
	if !d.Message.Stopped || len(d.Message.Media) != 0 {
		t.Errorf("stopped: %+v", d.Message)
	}
	// The graphics card "failed", so it fell back to the processor.
	if !strings.Contains(strings.Join(readLines(t, argsFile), " "), "--backend cpu") {
		t.Error("expected the processor fallback")
	}
	if st := a.GetModelStatus(); st.OnGPU || !strings.Contains(st.Note, "processor") {
		t.Errorf("status: %+v", st)
	}
	// A failed job explains itself.
	t.Setenv("FAKE_SD_SLOW", "")
	t.Setenv("FAKE_SD_FAIL_JOB", "1")
	a.media.stop() // restart the engine so it sees the new settings
	n = rec.count()
	if _, err := a.MakeMedia("", "a boat", model, MediaOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	if d := rec.waitDone(t, n); !strings.Contains(d.Message.Error, "empty results") {
		t.Errorf("failed job: %+v", d.Message)
	}
}

// An engine that refuses one of Phoenix's options says which one, and
// Phoenix doesn't pointlessly retry on the processor.
func TestMediaEngineRefusesOption(t *testing.T) {
	a, rec, _ := setupApp(t)
	t.Setenv("PHOENIX_SD_SERVER", buildFake(t, "fakesd"))
	launches := filepath.Join(t.TempDir(), "launches")
	t.Setenv("FAKE_SD_LAUNCHES", launches)
	t.Setenv("FAKE_SD_REJECT", "--model-dir")
	model := installPictures(t, a)

	n := rec.count()
	if _, err := a.MakeMedia("", "a lighthouse", model, MediaOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	d := rec.waitDone(t, n)
	if !strings.Contains(d.Message.Error, "didn't accept one of Phoenix's settings") || !strings.Contains(d.Message.Error, "unknown argument: --model-dir") {
		t.Errorf("error: %q", d.Message.Error)
	}
	if b, _ := os.ReadFile(launches); strings.Count(string(b), "start") != 1 {
		t.Errorf("started %d times, want 1", strings.Count(string(b), "start"))
	}
}

// After an update changes an engine's version, the new one is fetched for
// models that need it.
func TestOldToolsAreNoticed(t *testing.T) {
	a, _, _ := setupApp(t)
	sd := mustTool(toolPictures)
	if a.needsTool(toolPictures) || a.hasOldTool(sd) {
		t.Fatal("nothing installed yet")
	}
	os.MkdirAll(filepath.Join(a.paths.Tools, "sd-master-920-2f88688"), 0o755)
	os.MkdirAll(filepath.Join(a.paths.Tools, "speech-v0.3.0.tmp"), 0o755)
	installPictures(t, a)
	if !a.needsTool(toolPictures) || !a.hasOldTool(sd) {
		t.Error("the picture engine is out of date")
	}
	if a.needsTool(toolSpeech) || a.hasOldTool(mustTool(toolSpeech)) {
		t.Error("nothing needs the speech engine, and a .tmp folder isn't an install")
	}
}

func TestFriendlyEngineError(t *testing.T) {
	usage := "\x1b[31;1m[E]\x1b[0m common.cpp:353 - error: unknown argument: --model-dir\nUsage: sd-server [options]\n  --negative-prompt-file  path to the file"
	if got := friendlyEngineError(usage); !strings.Contains(got, "(common.cpp:353 - error: unknown argument: --model-dir)") {
		t.Errorf("usage: %q", got)
	}
	if got := friendlyEngineError("loading\n[E] error: failed to read the file\ndone"); got != "The engine stopped: error: failed to read the file" {
		t.Errorf("error line: %q", got)
	}
	if got := friendlyEngineError("just\nthe last line"); got != "The engine stopped: the last line" {
		t.Errorf("last line: %q", got)
	}
	// An error line is kept even after lots of output scrolls it away.
	l := newRingLog(5)
	fmt.Fprintln(l, "error: unknown argument: --nope")
	for i := 0; i < 50; i++ {
		fmt.Fprintln(l, "  --option  help text")
	}
	if !isSettingsError(l.String()) {
		t.Errorf("lost the error line: %q", l.String())
	}
}

// fakeHub stands in for Hugging Face and GitHub downloads.
type fakeHub struct {
	srv   *httptest.Server
	files map[string][]byte // "repo/file" or "/github/<name>"
	mu    sync.Mutex
	hits  map[string]int
}

func newFakeHub(t *testing.T) *fakeHub {
	h := &fakeHub{files: map[string][]byte{}, hits: map[string]int{}}
	h.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		h.hits[r.URL.Path]++
		h.mu.Unlock()
		path := r.URL.Path
		if rest, ok := strings.CutPrefix(path, "/api/models/"); ok {
			// /api/models/<owner>/<name>[/revision/<rev>]
			parts := strings.Split(rest, "/")
			repo := parts[0] + "/" + parts[1]
			type sib struct {
				Name string `json:"rfilename"`
				Size int64  `json:"size"`
				LFS  any    `json:"lfs"`
			}
			out := struct {
				ID       string `json:"id"`
				Siblings []sib  `json:"siblings"`
			}{ID: repo}
			for k, v := range h.files {
				if f, ok := strings.CutPrefix(k, repo+"/"); ok {
					sum := sha256.Sum256(v)
					out.Siblings = append(out.Siblings, sib{Name: f, Size: int64(len(v)), LFS: map[string]any{"sha256": hex.EncodeToString(sum[:]), "size": len(v)}})
				}
			}
			json.NewEncoder(w).Encode(out)
			return
		}
		if i := strings.Index(path, "/resolve/"); i > 0 {
			repo := strings.TrimPrefix(path[:i], "/")
			rest := path[i+len("/resolve/"):]
			file := rest[strings.Index(rest, "/")+1:]
			if b, ok := h.files[repo+"/"+file]; ok {
				http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(b))
				return
			}
		}
		if b, ok := h.files[path]; ok {
			http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(b))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(h.srv.Close)
	old := hubBase
	hubBase = h.srv.URL
	t.Cleanup(func() { hubBase = old })
	return h
}

func setTool(t toolSpec) {
	toolsMu.Lock()
	tools[t.Key] = t
	toolsMu.Unlock()
}

func zipWith(name string, data []byte) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create(name)
	w.Write(data)
	zw.Close()
	return buf.Bytes()
}

func TestDownloadPictureModelWithPartsAndEngine(t *testing.T) {
	a, rec, _ := setupApp(t)
	hub := newFakeHub(t)
	sdxl, _ := findCatalog("sdxl")
	hub.files[sdxl.Repo+"/"+sdxl.File] = fakeSafetensors()
	hub.files[sdxl.Parts[0].Repo+"/"+sdxl.Parts[0].File] = fakeSafetensors()

	// The engine comes from a "GitHub release" with a known checksum.
	exe := mustTool(toolPictures).exeName()
	engineZip := zipWith("bin/"+exe, []byte("#!/bin/sh\n"))
	hub.files["/github/sd.zip"] = engineZip
	sum := sha256.Sum256(engineZip)
	saved := mustTool(toolPictures)
	spec := saved
	spec.Assets = map[string]toolAsset{runtime.GOOS + "/" + runtime.GOARCH: {hub.srv.URL + "/github/sd.zip", int64(len(engineZip)), hex.EncodeToString(sum[:])}}
	setTool(spec)
	t.Cleanup(func() { setTool(saved) })
	t.Setenv("PHOENIX_SD_SERVER", "")

	if err := a.DownloadModel("sdxl"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if a.modelComplete(sdxl) && len(a.GetDownloads()) == 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !a.modelComplete(sdxl) {
		t.Fatalf("not complete: missing %q, downloads %+v, events %v", a.missingFor(sdxl), a.GetDownloads(), rec.eventsNamed("download:progress"))
	}
	if p := a.toolPath(toolPictures); p == "" || !strings.HasSuffix(p, exe) {
		t.Errorf("engine at %q", p)
	}
	names := map[string]bool{}
	for _, e := range rec.eventsNamed("download:progress") {
		st := e.(DownloadState)
		if st.Status == "done" {
			names[st.Name] = true
		}
	}
	if !names["Stable Diffusion XL"] || !names["Picture decoder for Stable Diffusion XL"] || !names["Picture and video engine"] {
		t.Errorf("finished downloads: %v", names)
	}

	// A damaged engine download is refused.
	os.RemoveAll(mustTool(toolPictures).dir(a))
	bad := spec
	bad.Assets = map[string]toolAsset{runtime.GOOS + "/" + runtime.GOARCH: {hub.srv.URL + "/github/sd.zip", int64(len(engineZip)), strings.Repeat("0", 64)}}
	setTool(bad)
	n := len(rec.eventsNamed("download:progress"))
	if err := a.ensureTool(toolPictures); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(10 * time.Second)
	var last DownloadState
	for time.Now().Before(deadline) {
		ev := rec.eventsNamed("download:progress")
		if len(ev) > n {
			last = ev[len(ev)-1].(DownloadState)
			if last.Status == "error" || last.Status == "done" {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
	if last.Status != "error" || !strings.Contains(last.Error, "checksum") {
		t.Errorf("expected a checksum error, got %+v", last)
	}
}

func TestDownloadVoiceArchive(t *testing.T) {
	a, _, _ := setupApp(t)
	hub := newFakeHub(t)
	arc, err := os.ReadFile("testdata/kokoro-test.tar.bz2")
	if err != nil {
		t.Fatal(err)
	}
	hub.files["/github/kokoro.tar.bz2"] = arc
	saved := archiveURL
	archiveURL = func(*ArchiveSource) string { return hub.srv.URL + "/github/kokoro.tar.bz2" }
	t.Cleanup(func() { archiveURL = saved })
	t.Setenv("PHOENIX_SHERPA_TTS", buildFake(t, "faketts"))
	if err := a.DownloadModel("kokoro"); err != nil {
		t.Fatal(err)
	}
	k, _ := findCatalog("kokoro")
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) && !a.GetSpeechStatus().Voice {
		time.Sleep(50 * time.Millisecond)
	}
	if !a.GetSpeechStatus().Voice {
		t.Fatalf("voice not installed: %+v", a.GetDownloads())
	}
	dir := filepath.Join(a.paths.Models, repoDir(k.Repo), k.Archive.Folder)
	for _, f := range []string{"model.int8.onnx", "voices.bin", "tokens.txt", "lexicon-us-en.txt"} {
		if !fileExists(filepath.Join(dir, f)) {
			t.Errorf("%s missing", f)
		}
	}
	if fileExists(filepath.Join(a.paths.Models, repoDir(k.Repo), k.Archive.Folder+".archive.part")) {
		t.Error("the archive should be deleted after unpacking")
	}
	// It's listed, ready, and deleting it removes the folder.
	var id string
	for _, m := range a.GetInstalledModels() {
		if m.Kind == KindVoice {
			id = m.ID
			if !m.Ready {
				t.Errorf("voice not ready: %+v", m)
			}
		}
	}
	if err := a.DeleteModel(id); err != nil || fileExists(dir) {
		t.Errorf("delete voice: %v", err)
	}
}
