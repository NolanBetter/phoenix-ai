package backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVersionNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.2.1", "0.2.0", true},
		{"v1.0.0", "0.9.9", true},
		{"0.2.0", "0.2.0", false},
		{"0.1.9", "0.2.0", false},
		{"0.10.0", "0.9.0", true},
		{"0.2.0", "0.2.0-dev", true},
		{"0.2.0", "0.0.0-dev", true},
		{"garbage", "0.1.0", false},
		{"0.2", "0.1.5", true},
	}
	for _, c := range cases {
		if got := versionNewer(c.a, c.b); got != c.want {
			t.Errorf("versionNewer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// fakeGitHub serves /repos/<repo>/releases/latest and the asset downloads.
func fakeGitHub(t *testing.T, tag string, payload []byte, digest, sidecar string) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := "PhoenixAI-Setup-" + strings.TrimPrefix(tag, "v") + ".exe"
		switch {
		case r.URL.Path == "/repos/"+UpdateRepo+"/releases/latest":
			if tag == "" {
				http.NotFound(w, r)
				return
			}
			assets := fmt.Sprintf(`{"name":%q,"browser_download_url":%q,"size":%d,"digest":%q}`, name, srv.URL+"/dl/"+name, len(payload), digest)
			if sidecar != "" {
				assets += fmt.Sprintf(`,{"name":%q,"browser_download_url":%q,"size":80}`, name+".sha256", srv.URL+"/dl/"+name+".sha256")
			}
			fmt.Fprintf(w, `{"tag_name":%q,"body":"Redder theme and auto-updates.","assets":[%s]}`, tag, assets)
		case r.URL.Path == "/dl/"+name:
			w.Write(payload)
		case r.URL.Path == "/dl/"+name+".sha256":
			fmt.Fprintf(w, "%s  %s\n", sidecar, name)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func updaterApp(t *testing.T, srv *httptest.Server) *App {
	t.Helper()
	oldV, oldBase := Version, updateAPIBase
	Version, updateAPIBase = "0.2.0", srv.URL
	t.Cleanup(func() { Version, updateAPIBase = oldV, oldBase })
	t.Setenv("PHOENIX_HOME", t.TempDir())
	t.Setenv("PHOENIX_NO_UPDATE", "1")
	a := NewApp()
	if err := Start(a, context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestUpdateDownloadsAndVerifies(t *testing.T) {
	payload := []byte(strings.Repeat("MZ-installer-", 5000))
	sum := sha256.Sum256(payload)
	srv := fakeGitHub(t, "v0.3.0", payload, "sha256:"+hex.EncodeToString(sum[:]), "")
	a := updaterApp(t, srv)

	st, err := a.CheckForUpdates()
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "ready" || st.Latest != "0.3.0" || st.Notes == "" {
		t.Fatalf("status: %+v", st)
	}
	path := filepath.Join(a.paths.Updates, "PhoenixAI-Setup-0.3.0.exe")
	if b, err := os.ReadFile(path); err != nil || len(b) != len(payload) {
		t.Fatal("installer not saved")
	}
	// Installing is Windows-only; elsewhere it must fail cleanly and not quit.
	if err := a.InstallUpdate(); err == nil {
		t.Error("expected an error outside Windows")
	}
}

func TestUpdateRejectsDamagedFile(t *testing.T) {
	srv := fakeGitHub(t, "v0.3.0", []byte("tampered"), "sha256:"+strings.Repeat("ab", 32), "")
	a := updaterApp(t, srv)
	st, err := a.CheckForUpdates()
	if err == nil || st.State != "error" || !strings.Contains(st.Error, "damaged") {
		t.Fatalf("expected damaged error, got %v %+v", err, st)
	}
	entries, _ := os.ReadDir(a.paths.Updates)
	if len(entries) != 0 {
		t.Fatalf("damaged download left behind: %v", entries)
	}
}

func TestUpdateUsesSidecarChecksum(t *testing.T) {
	payload := []byte("installer bytes")
	sum := sha256.Sum256(payload)
	srv := fakeGitHub(t, "v0.2.1", payload, "", hex.EncodeToString(sum[:]))
	a := updaterApp(t, srv)
	if st, err := a.CheckForUpdates(); err != nil || st.State != "ready" {
		t.Fatalf("sidecar: %v %+v", err, st)
	}
}

func TestUpdateRefusesWithoutChecksum(t *testing.T) {
	srv := fakeGitHub(t, "v0.2.1", []byte("x"), "", "")
	a := updaterApp(t, srv)
	if st, err := a.CheckForUpdates(); err == nil || !strings.Contains(st.Error, "checksum") {
		t.Fatalf("expected refusal, got %v %+v", err, st)
	}
}

func TestUpdateUpToDate(t *testing.T) {
	for _, tag := range []string{"v0.2.0", "v0.1.0", ""} {
		srv := fakeGitHub(t, tag, []byte("x"), "", "")
		a := updaterApp(t, srv)
		st, err := a.CheckForUpdates()
		if err != nil || st.State != "uptodate" {
			t.Fatalf("tag %q: %v %+v", tag, err, st)
		}
	}
}

func TestChatLimitAndUnread(t *testing.T) {
	a, rec, model := setupApp(t)

	// Answer finishes while another screen is showing: unread.
	a.SetViewingChat("")
	n := rec.count()
	id, err := a.SendMessage("", "hello", model, false)
	if err != nil {
		t.Fatal(err)
	}
	rec.waitDone(t, n)
	if c := a.ListChats(); len(c) != 1 || !c[0].Unread {
		t.Fatalf("expected unread chat, got %+v", c)
	}
	a.SetViewingChat(id)
	if a.ListChats()[0].Unread {
		t.Fatal("opening the chat should mark it read")
	}
	// Answer finishes while the chat is on screen: stays read.
	n = rec.count()
	a.SendMessage(id, "again", model, false)
	rec.waitDone(t, n)
	if a.ListChats()[0].Unread {
		t.Fatal("chat on screen shouldn't become unread")
	}

	// Fill up to the limit.
	for i := 1; i < MaxChats; i++ {
		a.chats.save(&Chat{ID: newChatID(), Title: fmt.Sprintf("old %d", i), Updated: int64(i)})
	}
	if _, err := a.SendMessage("", "one too many", model, false); err == nil || !strings.Contains(err.Error(), "10 chats") {
		t.Fatalf("expected chat limit error, got %v", err)
	}
	// Existing chats still work.
	n = rec.count()
	if _, err := a.SendMessage(id, "still fine", model, false); err != nil {
		t.Fatal(err)
	}
	rec.waitDone(t, n)
	// Deleting one frees a slot.
	a.DeleteChat(a.ListChats()[MaxChats-1].ID)
	n = rec.count()
	if _, err := a.SendMessage("", "now it fits", model, false); err != nil {
		t.Fatalf("after delete: %v", err)
	}
	rec.waitDone(t, n)
}

func TestModelLimit(t *testing.T) {
	a, _, _ := setupApp(t) // one model already
	add := func(rel string) {
		p := filepath.Join(a.paths.Models, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("GGUF"), 0o644)
	}
	add("x__a/a-Q4_K_M.gguf")
	add("x__b/b-Q4_K_M.gguf")
	add("x__c/c-Q4_K_M.gguf")
	add("x__c/mmproj-c-F16.gguf")  // vision helper file, not a model
	add("x__d/d-Q4_K_M.gguf.part") // unfinished download counts too

	a.dl.mu.Lock()
	used := a.modelSlotsUsedLocked("")
	resume := a.modelSlotsUsedLocked("x__d/d-Q4_K_M.gguf")
	a.dl.mu.Unlock()
	if used != 5 || resume != 4 {
		t.Fatalf("slots used = %d (want 5), when resuming = %d (want 4)", used, resume)
	}
	err := a.DownloadFromHub("someone/new-GGUF", "new-Q4_K_M.gguf")
	if err == nil || !strings.Contains(err.Error(), "up to 5 models") {
		t.Fatalf("expected model limit error, got %v", err)
	}
	if len(a.GetInstalledModels()) != 4 {
		t.Fatalf("installed list should skip mmproj and .part files")
	}
	_ = time.Second
}
