package backend

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseDevices(t *testing.T) {
	out := "ggml_vulkan: Found 1 Vulkan devices:\nAvailable devices:\n  Vulkan0: NVIDIA GeForce RTX 4070 (16311 MiB, 15112 MiB free)\n  Vulkan1: Intel(R) UHD Graphics (8000 MiB, 7000 MiB free)\n"
	g := parseDevices(out)
	if len(g) != 2 || g[0].Name != "NVIDIA GeForce RTX 4070" || g[0].VRAM != 16311<<20 || g[1].Name != "Intel(R) UHD Graphics" {
		t.Fatalf("got %+v", g)
	}
	if len(parseDevices("Available devices:\n  (none)\n")) != 0 {
		t.Fatal("expected none")
	}
}

func TestNamesAndQuants(t *testing.T) {
	cases := map[string]string{
		"Llama-3.2-3B-Instruct-Q4_K_M.gguf":          "Q4_K_M",
		"Qwen3.8-27B-UD-Q4_K_XL.gguf":                "UD-Q4_K_XL",
		"gemma-4-12b-it-qat-q4_0.gguf":               "Q4_0",
		"gpt-oss-20b-MXFP4_MOE.gguf":                 "MXFP4_MOE",
		"sub/folder/Mistral-7B-Instruct-IQ4_XS.gguf": "IQ4_XS",
	}
	for f, want := range cases {
		if got := quantOf(f); got != want {
			t.Errorf("quantOf(%q) = %q, want %q", f, got, want)
		}
	}
	if got := displayName("bartowski/x", "Llama-3.2-3B-Instruct-Q4_K_M.gguf"); got != "Llama 3.2 3B Instruct" {
		t.Errorf("displayName = %q", got)
	}
	if got := displayName("unsloth/Qwen3.8-27B-GGUF", "Qwen3.8-27B-UD-Q4_K_M.gguf"); got != "Qwen 3.8" {
		t.Errorf("catalog displayName = %q", got)
	}
	if got := displayName("a/b", "Qwen3.8-27B-UD-Q4_K_XL.gguf"); got != "Qwen3.8 27B" {
		t.Errorf("UD displayName = %q", got)
	}
}

func TestSafeJoin(t *testing.T) {
	root := filepath.Join(t.TempDir(), "models")
	for _, bad := range []string{"../x.gguf", "a/../../x.gguf", "/etc/passwd", "..", ""} {
		if _, ok := safeJoin(root, bad); ok {
			t.Errorf("safeJoin allowed %q", bad)
		}
	}
	if p, ok := safeJoin(root, "a__b/c.gguf"); !ok || !strings.HasPrefix(p, root) {
		t.Errorf("safeJoin rejected a good path")
	}
	if validModelFile("../a.gguf") || validModelFile("a.txt") || !validModelFile("dir/a.gguf") {
		t.Error("validModelFile is wrong")
	}
}

func TestFit(t *testing.T) {
	sys := &SystemInfo{RAM: 32 * gb, BestVRAM: 16 * gb}
	if f := fitFor(5*gb, sys); f.Level != "great" {
		t.Errorf("5GB on 16GB card: %v", f)
	}
	if f := fitFor(20*gb, sys); f.Level != "slow" {
		t.Errorf("20GB: %v", f)
	}
	if f := fitFor(60*gb, sys); f.Level != "toobig" {
		t.Errorf("60GB: %v", f)
	}
	potato := &SystemInfo{RAM: 8 * gb}
	if f := fitFor(1*gb, potato); f.Level != "ok" {
		t.Errorf("1GB on potato: %v", f)
	}
	if f := fitFor(4*gb, potato); f.Level != "slow" {
		t.Errorf("4GB on potato: %v", f)
	}
}

func TestThinkSplitter(t *testing.T) {
	run := func(chunks ...string) (string, string) {
		s := &thinkSplitter{}
		var th, co string
		for _, c := range chunks {
			a, b := s.feed(c)
			th += a
			co += b
		}
		a, b := s.finish()
		return th + a, co + b
	}
	if th, co := run("<thi", "nk>abc</th", "ink>\n\nHello"); th != "abc" || co != "Hello" {
		t.Errorf("split tags: %q %q", th, co)
	}
	if th, co := run("Hello ", "world"); th != "" || co != "Hello world" {
		t.Errorf("plain: %q %q", th, co)
	}
	if th, co := run("<", "b>bold</b>"); th != "" || co != "<b>bold</b>" {
		t.Errorf("html start: %q %q", th, co)
	}
	if th, co := run("<think>never closed"); th != "never closed" || co != "" {
		t.Errorf("unclosed: %q %q", th, co)
	}
}

func TestBuildPrompt(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: strings.Repeat("a", 20000)},
		{Role: "assistant", Content: strings.Repeat("b", 20000)},
		{Role: "user", Content: "latest"},
		{Role: "assistant", Error: "boom"},
		{Role: "user", Content: "again"},
	}
	p := buildPrompt("Be nice.", msgs, 8192)
	if p[0].Role != "system" || p[len(p)-1].Content != "again" {
		t.Fatalf("bad prompt %+v", p)
	}
	for _, m := range p {
		if m.Content == "" {
			t.Fatal("empty message sent")
		}
	}
	if p[1].Role != "user" {
		t.Fatalf("prompt should start with user after system, got %s", p[1].Role)
	}
	if len(p) > 4 {
		t.Fatalf("old turns should be dropped, got %d messages", len(p))
	}
	if titleFrom("  how do i make\n a minecraft plugin with paper and gradle please help") != "how do i make" {
		t.Errorf("title = %q", titleFrom("  how do i make\n a minecraft"))
	}
	if got := titleFrom("Write a long story about a phoenix that rises from ashes over the mountains"); !strings.HasSuffix(got, "…") || len([]rune(got)) > 43 {
		t.Errorf("title = %q", got)
	}
}

func TestFindEngineRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[
		  {"tag_name":"v0.7.0","draft":false,"assets":[]},
		  {"tag_name":"b11670","draft":true,"assets":[{"name":"llama-b11670-bin-win-vulkan-x64.zip","browser_download_url":"https://x/draft","size":1}]},
		  {"tag_name":"b11669","draft":false,"assets":[
		    {"name":"llama-b11669-bin-win-cuda-12.4-x64.zip","browser_download_url":"https://x/cuda","size":1},
		    {"name":"llama-b11669-bin-win-vulkan-x64.zip","browser_download_url":"https://x/vulkan","size":4242}]}
		]`))
	}))
	defer srv.Close()
	rel, err := findEngineRelease(context.Background(), srv.Client(), srv.URL, regexp.MustCompile(`^llama-(b\d+)-bin-win-vulkan-x64\.zip$`))
	if err != nil {
		t.Fatal(err)
	}
	if rel.Tag != "b11669" || rel.URL != "https://x/vulkan" || rel.Size != 4242 {
		t.Fatalf("got %+v", rel)
	}
	if buildNumber("b11669") <= buildNumber("b9000") || buildNumber("v0.7.0") != 0 {
		t.Error("buildNumber is wrong")
	}
}

func TestExtractZip(t *testing.T) {
	dir := t.TempDir()
	mk := func(name string, files map[string]string) string {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		for n, c := range files {
			w, _ := zw.Create(n)
			w.Write([]byte(c))
		}
		zw.Close()
		p := filepath.Join(dir, name)
		os.WriteFile(p, buf.Bytes(), 0o644)
		return p
	}
	good := mk("good.zip", map[string]string{"llama-server.exe": "MZ", "ggml-vulkan.dll": "x"})
	if err := extractArchive(good, filepath.Join(dir, "out")); err != nil {
		t.Fatal(err)
	}
	if findFile(filepath.Join(dir, "out"), "LLAMA-SERVER.EXE") == "" {
		t.Fatal("exe not found after extract")
	}
	evil := mk("evil.zip", map[string]string{"../../evil.txt": "x"})
	if err := extractArchive(evil, filepath.Join(dir, "out2")); err == nil {
		t.Fatal("zip-slip archive was accepted")
	}
}

func TestDownloadResumeAndHash(t *testing.T) {
	data := bytes.Repeat([]byte("GGUF-phoenix-"), 200000)
	sum := sha256.Sum256(data)
	var ranged bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			ranged = true
		}
		http.ServeContent(w, r, "m.gguf", time.Now(), bytes.NewReader(data))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "m.gguf.part")
	os.WriteFile(dest, data[:700000], 0o644) // pretend an earlier download stopped here
	h := sha256.New()
	var last int64
	err := downloadFile(context.Background(), srv.Client(), srv.URL, dest, true, func(got, total int64) { last = got }, h)
	if err != nil {
		t.Fatal(err)
	}
	if !ranged {
		t.Error("download didn't resume with a Range request")
	}
	b, _ := os.ReadFile(dest)
	if !bytes.Equal(b, data) || last != int64(len(data)) {
		t.Fatal("resumed file doesn't match")
	}
	if hex.EncodeToString(h.Sum(nil)) != hex.EncodeToString(sum[:]) {
		t.Fatal("hash over resumed file is wrong")
	}
}

// ---- Full flow against a fake llama-server ----

type recorder struct {
	mu     sync.Mutex
	events []struct {
		name string
		data any
	}
}

func (r *recorder) emit(name string, data any) {
	r.mu.Lock()
	r.events = append(r.events, struct {
		name string
		data any
	}{name, data})
	r.mu.Unlock()
}

func (r *recorder) waitDone(t *testing.T, after int) chatDone {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		for i := after; i < len(r.events); i++ {
			if r.events[i].name == "chat:done" {
				d := r.events[i].data.(chatDone)
				r.mu.Unlock()
				return d
			}
		}
		r.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("no chat:done event")
	return chatDone{}
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

func (r *recorder) deltas(after int) (content, thinking string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.events[after:] {
		if d, ok := e.data.(chatDelta); ok {
			content += d.Content
			thinking += d.Thinking
		}
	}
	return
}

func setupApp(t *testing.T) (*App, *recorder, string) {
	t.Helper()
	home := t.TempDir()
	bin := filepath.Join(t.TempDir(), "fake-llama-server")
	out, err := exec.Command("go", "build", "-o", bin, "./testdata/fakeserver").CombinedOutput()
	if err != nil {
		t.Fatalf("build fake server: %v\n%s", err, out)
	}
	t.Setenv("PHOENIX_HOME", home)
	t.Setenv("PHOENIX_LLAMA_SERVER", bin)
	rec := &recorder{}
	a := NewApp()
	if err := Start(a, context.Background(), rec.emit); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { Stop(a) })
	id := modelID("someone/tiny-GGUF", "tiny-model-Q4_K_M.gguf")
	p, _ := safeJoin(a.paths.Models, id)
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, append([]byte("GGUF"), make([]byte, 1024)...), 0o644)
	return a, rec, id
}

func TestChatFlow(t *testing.T) {
	a, rec, id := setupApp(t)

	sys := a.GetSystemInfo()
	if len(sys.GPUs) != 1 || sys.BestVRAM != 16311<<20 {
		t.Fatalf("GPU detection: %+v", sys)
	}
	models := a.GetInstalledModels()
	if len(models) != 1 || models[0].ID != id || models[0].Name != "tiny model" || models[0].Quant != "Q4_K_M" {
		t.Fatalf("installed: %+v", models)
	}

	// 1. Plain answer, new chat.
	n := rec.count()
	chatID, err := a.SendMessage("", "hello there", id, false)
	if err != nil {
		t.Fatal(err)
	}
	d := rec.waitDone(t, n)
	if d.Message.Error != "" || d.Message.Content != "You said **hello there** and there are 1 messages." {
		t.Fatalf("answer: %+v", d.Message)
	}
	if d.Message.TokensPerSec != 42.5 {
		t.Errorf("tps = %v", d.Message.TokensPerSec)
	}
	if c, _ := rec.deltas(n); c != d.Message.Content {
		t.Errorf("streamed %q", c)
	}
	st := a.GetModelStatus()
	if st.State != "ready" || !st.OnGPU || !strings.Contains(st.Note, "13 of 33") {
		t.Errorf("status: %+v", st)
	}

	// 2. Thinking, same chat. History is sent along.
	n = rec.count()
	if _, err := a.SendMessage(chatID, "think about it", id, true); err != nil {
		t.Fatal(err)
	}
	d = rec.waitDone(t, n)
	if d.Message.Thinking != "The user said: think about it" || !strings.Contains(d.Message.Content, "3 messages") {
		t.Fatalf("thinking answer: %+v", d.Message)
	}

	// 3. <think> tags inside content are split out.
	n = rec.count()
	a.SendMessage(chatID, "use tags", id, false)
	d = rec.waitDone(t, n)
	if d.Message.Thinking != "raw thoughts" || d.Message.Content != "Tagged answer." {
		t.Fatalf("tags: %+v", d.Message)
	}

	// 4. Context overflow gives a clear message.
	n = rec.count()
	a.SendMessage(chatID, "this is too long", id, false)
	d = rec.waitDone(t, n)
	if !strings.Contains(d.Message.Error, "too long for the model's memory") {
		t.Fatalf("overflow: %+v", d.Message)
	}

	// 5. Retry replaces the failed answer.
	n = rec.count()
	ch, _ := a.GetChat(chatID)
	before := len(ch.Messages)
	if err := a.Regenerate(chatID, id, false); err != nil {
		t.Fatal(err)
	}
	rec.waitDone(t, n)
	ch, _ = a.GetChat(chatID)
	if len(ch.Messages) != before {
		t.Fatalf("regenerate should replace, had %d now %d", before, len(ch.Messages))
	}

	// 6. Stop keeps the partial answer.
	n = rec.count()
	a.SendMessage(chatID, "go slow please now ok", id, false)
	time.Sleep(700 * time.Millisecond)
	if _, err := a.SendMessage(chatID, "second", id, false); err == nil {
		t.Error("should refuse a second message while answering")
	}
	a.StopGeneration()
	d = rec.waitDone(t, n)
	if !d.Message.Stopped || d.Message.Content == "" {
		t.Fatalf("stopped: %+v", d.Message)
	}

	// Chat list and rename.
	list := a.ListChats()
	if len(list) != 1 || list[0].Title != "hello there" {
		t.Fatalf("list: %+v", list)
	}
	a.RenameChat(chatID, "Renamed")
	if a.ListChats()[0].Title != "Renamed" {
		t.Error("rename failed")
	}

	// Settings change restarts the model with the new memory length.
	a.SaveSettings(Settings{UseGPU: true, ContextSize: 4096, Theme: "dark"})
	n = rec.count()
	a.SendMessage(chatID, "after settings", id, false)
	if d = rec.waitDone(t, n); d.Message.Error != "" {
		t.Fatal(d.Message.Error)
	}
	if a.runner.cfg.Ctx != 4096 {
		t.Errorf("ctx not applied: %d", a.runner.cfg.Ctx)
	}

	if err := a.DeleteChat(chatID); err != nil || len(a.ListChats()) != 0 {
		t.Fatal("delete chat failed")
	}
	if err := a.DeleteModel(id); err != nil || len(a.GetInstalledModels()) != 0 {
		t.Fatalf("delete model failed: %v", err)
	}
	if a.GetModelStatus().State != "idle" {
		t.Error("model should be unloaded after delete")
	}
}

func TestGPUFallbackAndErrors(t *testing.T) {
	a, rec, id := setupApp(t)

	t.Setenv("FAKE_FAIL_GPU", "1")
	n := rec.count()
	a.SendMessage("", "hi", id, false)
	d := rec.waitDone(t, n)
	if d.Message.Error != "" {
		t.Fatalf("fallback failed: %s", d.Message.Error)
	}
	st := a.GetModelStatus()
	if st.OnGPU || !strings.Contains(st.Note, "processor") {
		t.Errorf("status after fallback: %+v", st)
	}
	a.UnloadModel()

	t.Setenv("FAKE_FAIL_GPU", "")
	t.Setenv("FAKE_FAIL_ALL", "1")
	n = rec.count()
	a.SendMessage("", "hi again", id, false)
	d = rec.waitDone(t, n)
	if !strings.Contains(d.Message.Error, "newer than your AI engine") {
		t.Fatalf("expected friendly error, got %q", d.Message.Error)
	}
	if a.GetModelStatus().State != "error" {
		t.Error("status should be error")
	}
}

// Real download from Hugging Face. Run with PHOENIX_NET_TEST=1.
func TestRealDownload(t *testing.T) {
	if os.Getenv("PHOENIX_NET_TEST") == "" {
		t.Skip("set PHOENIX_NET_TEST=1 to download a real model")
	}
	t.Setenv("PHOENIX_HOME", t.TempDir())
	rec := &recorder{}
	a := NewApp()
	if err := Start(a, context.Background(), rec.emit); err != nil {
		t.Fatal(err)
	}
	defer Stop(a)

	if err := a.DownloadModel("qwen3.5-0.8b"); err != nil {
		t.Fatal(err)
	}
	// Let it get partway, then simulate closing the app.
	for {
		ds := a.GetDownloads()
		if len(ds) == 1 && ds[0].Received > 80<<20 {
			t.Logf("got %d MB at %.1f MB/s, closing", ds[0].Received>>20, ds[0].Speed/1e6)
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	a.dl.cancelAll()
	time.Sleep(500 * time.Millisecond)
	ds := a.GetDownloads()
	if len(ds) != 1 || ds[0].Status != "paused" || ds[0].CatalogID != "qwen3.5-0.8b" {
		t.Fatalf("expected a paused download, got %+v", ds)
	}

	// Resume it.
	if err := a.DownloadFromHub(ds[0].Repo, ds[0].File); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		var final *DownloadState
		rec.mu.Lock()
		for i := range rec.events {
			if s, ok := rec.events[i].data.(DownloadState); ok && (s.Status == "done" || s.Status == "error") {
				final = &s
			}
		}
		rec.mu.Unlock()
		if final != nil {
			if final.Status != "done" {
				t.Fatalf("download failed: %s", final.Error)
			}
			ms := a.GetInstalledModels()
			if len(ms) != 1 || ms[0].Size != 532517120 || ms[0].Name != "Qwen 3.5 Mini" {
				t.Fatalf("installed: %+v", ms)
			}
			cat := a.GetCatalog()
			if !cat[0].Installed {
				t.Error("catalog should show installed")
			}
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("download timed out")
}

func TestRealHubSearch(t *testing.T) {
	if os.Getenv("PHOENIX_NET_TEST") == "" {
		t.Skip("set PHOENIX_NET_TEST=1")
	}
	t.Setenv("PHOENIX_HOME", t.TempDir())
	a := NewApp()
	Start(a, context.Background(), nil)
	res, err := a.SearchHub("llama 3.2 3b instruct")
	if err != nil || len(res) == 0 {
		t.Fatalf("search: %v %d", err, len(res))
	}
	files, err := a.GetHubFiles("bartowski/Llama-3.2-3B-Instruct-GGUF")
	if err != nil {
		t.Fatal(err)
	}
	rec := 0
	for _, f := range files {
		if f.Recommended {
			rec++
			if f.Quant != "Q4_K_M" {
				t.Errorf("recommended %s", f.File)
			}
		}
	}
	if rec != 1 || len(files) < 5 {
		t.Fatalf("files: %d recommended %d", len(files), rec)
	}
	t.Logf("search top: %s, files: %d", res[0].Repo, len(files))
}
