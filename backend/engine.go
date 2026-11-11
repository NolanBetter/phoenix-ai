package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// The engine is llama.cpp's llama-server, the open-source program that
// actually runs the models. Phoenix downloads the official build from
// GitHub into %LOCALAPPDATA%\PhoenixAI\engine on first launch.

const releasesURL = "https://api.github.com/repos/ggml-org/llama.cpp/releases?per_page=30"

// EngineStatus is shown in setup and in Settings.
type EngineStatus struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Busy      bool   `json:"busy"`
	Stage     string `json:"stage"`
	Received  int64  `json:"received"`
	Total     int64  `json:"total"`
	Error     string `json:"error"`
}

// EngineUpdate tells Settings whether a newer engine exists.
type EngineUpdate struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
}

type engineManager struct {
	a      *App
	mu     sync.Mutex
	status EngineStatus
}

func newEngineManager(a *App) *engineManager { return &engineManager{a: a} }

// exePath returns the installed llama-server, or "" if there isn't one.
func (e *engineManager) exePath() string {
	if p := os.Getenv("PHOENIX_LLAMA_SERVER"); p != "" {
		return p
	}
	s := e.a.getSettings()
	if s.EngineExe == "" {
		return ""
	}
	full, ok := safeJoin(e.a.paths.Engine, s.EngineExe)
	if !ok {
		return ""
	}
	if _, err := os.Stat(full); err != nil {
		return ""
	}
	return full
}

func (e *engineManager) snapshot() EngineStatus {
	e.mu.Lock()
	st := e.status
	e.mu.Unlock()
	exe := e.exePath()
	st.Installed = exe != ""
	st.Version = e.a.getSettings().EngineTag
	if os.Getenv("PHOENIX_LLAMA_SERVER") != "" {
		st.Version = "custom"
	}
	return st
}

func (e *engineManager) set(fn func(s *EngineStatus)) {
	e.mu.Lock()
	fn(&e.status)
	e.mu.Unlock()
	e.a.emit("engine:status", e.snapshot())
}

// GetEngineStatus reports whether the engine is installed.
func (a *App) GetEngineStatus() EngineStatus { return a.engine.snapshot() }

// InstallEngine downloads the newest engine in the background.
// Progress arrives as "engine:status" events.
func (a *App) InstallEngine() error {
	e := a.engine
	e.mu.Lock()
	if e.status.Busy {
		e.mu.Unlock()
		return errors.New("the engine is already being set up")
	}
	e.status = EngineStatus{Busy: true, Stage: "Finding the latest engine"}
	e.mu.Unlock()
	a.emit("engine:status", e.snapshot())

	go func() {
		err := e.install(a.ctx)
		e.set(func(s *EngineStatus) {
			s.Busy = false
			s.Stage = ""
			if err != nil {
				s.Error = err.Error()
			} else {
				s.Error = ""
			}
		})
	}()
	return nil
}

// CheckEngineUpdate looks on GitHub for a newer engine build.
func (a *App) CheckEngineUpdate() (EngineUpdate, error) {
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	rel, err := findEngineRelease(ctx, a.http, releasesURL, engineAssetPattern())
	if err != nil {
		return EngineUpdate{}, err
	}
	cur := a.getSettings().EngineTag
	return EngineUpdate{Current: cur, Latest: rel.Tag, Available: cur == "" || buildNumber(rel.Tag) > buildNumber(cur)}, nil
}

func engineAssetPattern() *regexp.Regexp {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64":
		// Vulkan works on NVIDIA, AMD and Intel graphics cards, and falls back to the processor.
		return regexp.MustCompile(`^llama-(b\d+)-bin-win-vulkan-x64\.zip$`)
	case "windows/arm64":
		return regexp.MustCompile(`^llama-(b\d+)-bin-win-cpu-arm64\.zip$`)
	case "linux/amd64":
		return regexp.MustCompile(`^llama-(b\d+)-bin-ubuntu-vulkan-x64\.tar\.gz$`)
	case "linux/arm64":
		return regexp.MustCompile(`^llama-(b\d+)-bin-ubuntu-arm64\.tar\.gz$`)
	case "darwin/arm64":
		return regexp.MustCompile(`^llama-(b\d+)-bin-macos-arm64\.tar\.gz$`)
	default:
		return regexp.MustCompile(`^llama-(b\d+)-bin-macos-x64\.tar\.gz$`)
	}
}

type engineRelease struct {
	Tag  string
	Name string
	URL  string
	Size int64
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	Draft   bool   `json:"draft"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

// findEngineRelease walks recent llama.cpp releases (newest first) and
// returns the first one that has a build for this PC. Version releases
// like "v0.7.0" carry no binaries; the nightly "bNNNNN" ones do.
func findEngineRelease(ctx context.Context, client *http.Client, url string, pattern *regexp.Regexp) (engineRelease, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return engineRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "PhoenixAI/"+Version)
	resp, err := client.Do(req)
	if err != nil {
		return engineRelease{}, fmt.Errorf("couldn't reach GitHub to download the engine. Check your internet connection. (%v)", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 403 || resp.StatusCode == 429 {
		return engineRelease{}, errors.New("GitHub is limiting requests right now. Wait a few minutes and try again")
	}
	if resp.StatusCode != 200 {
		return engineRelease{}, fmt.Errorf("GitHub answered with status %d", resp.StatusCode)
	}
	var rels []ghRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&rels); err != nil {
		return engineRelease{}, fmt.Errorf("couldn't read the release list from GitHub: %w", err)
	}
	for _, r := range rels {
		if r.Draft {
			continue
		}
		for _, as := range r.Assets {
			if m := pattern.FindStringSubmatch(as.Name); m != nil {
				return engineRelease{Tag: m[1], Name: as.Name, URL: as.URL, Size: as.Size}, nil
			}
		}
	}
	return engineRelease{}, errors.New("no engine build for this kind of PC was found in the latest releases")
}

func buildNumber(tag string) int {
	n := 0
	for _, c := range strings.TrimPrefix(tag, "b") {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func (e *engineManager) install(ctx context.Context) error {
	a := e.a
	rel, err := findEngineRelease(ctx, a.http, releasesURL, engineAssetPattern())
	if err != nil {
		return err
	}
	e.set(func(s *EngineStatus) { s.Stage = "Downloading the engine"; s.Total = rel.Size; s.Received = 0 })

	archive := filepath.Join(a.paths.Engine, "download-"+rel.Name+".part")
	defer os.Remove(archive)
	th := throttle{interval: 200 * time.Millisecond}
	err = downloadFile(ctx, a.http, rel.URL, archive, false, func(got, total int64) {
		if th.ok() {
			e.set(func(s *EngineStatus) {
				s.Received = got
				if total > 0 {
					s.Total = total
				}
			})
		}
	}, nil)
	if err != nil {
		return fmt.Errorf("the engine download failed: %w", err)
	}

	e.set(func(s *EngineStatus) { s.Stage = "Unpacking"; s.Received = s.Total })
	dest := filepath.Join(a.paths.Engine, rel.Tag)
	tmp := dest + ".tmp"
	os.RemoveAll(tmp)
	if err := extractArchive(archive, tmp); err != nil {
		os.RemoveAll(tmp)
		return fmt.Errorf("couldn't unpack the engine: %w", err)
	}
	exe := findFile(tmp, engineExeName)
	if exe == "" {
		os.RemoveAll(tmp)
		return errors.New("the engine download didn't contain " + engineExeName)
	}

	// Swap in the new engine. Stop any running model first, since
	// Windows won't let us delete files that are in use.
	a.runner.stop()
	os.RemoveAll(dest)
	if err := os.Rename(tmp, dest); err != nil {
		os.RemoveAll(tmp)
		return fmt.Errorf("couldn't install the engine: %w", err)
	}
	relExe, _ := filepath.Rel(tmp, exe)
	if err := a.updateSettings(func(s *Settings) {
		s.EngineTag = rel.Tag
		s.EngineExe = filepath.ToSlash(filepath.Join(rel.Tag, relExe))
	}); err != nil {
		return err
	}
	// Remove older engine versions.
	if entries, err := os.ReadDir(a.paths.Engine); err == nil {
		for _, en := range entries {
			if en.IsDir() && en.Name() != rel.Tag {
				os.RemoveAll(filepath.Join(a.paths.Engine, en.Name()))
			}
		}
	}
	a.resetGPUInfo()
	return nil
}

func findFile(root, name string) string {
	var found string
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.EqualFold(d.Name(), name) {
			found = p
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
