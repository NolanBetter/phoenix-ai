// Package backend holds everything Phoenix AI does behind the window:
// installing the AI engine, downloading models, running them and chatting.
// It never imports Wails, so it can be tested on any OS.
package backend

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

// Version is set at build time from the VERSION file
// (go build -ldflags "-X phoenixai/backend.Version=0.2.0").
var Version = "0.0.0-dev"

// UpdateRepo is the GitHub repository whose Releases page Phoenix
// updates itself from. Empty turns updates off.
var UpdateRepo = "NolanBetter/phoenix-ai"

// Limits on how much Phoenix keeps on the PC.
const (
	MaxChats  = 10
	MaxModels = 5
)

// App is bound to the frontend. Every exported method becomes
// window.go.backend.App.<Method>() in JavaScript.
type App struct {
	ctx   context.Context
	emit  func(event string, data any)
	hooks Hooks
	paths *Paths

	settingsMu sync.Mutex
	settings   Settings

	http   *http.Client
	engine *engineManager
	runner *runner
	media  *mediaRunner  // pictures and videos (stable-diffusion.cpp)
	speech *speechRunner // speech recognition (NeMo-Speech.cpp)
	dl     *downloadManager
	chats  *chatStore

	genMu sync.Mutex
	gen   *generation

	sysMu sync.Mutex
	sys   *SystemInfo

	// The chat the window is showing right now ("" for none), so
	// answers that finish somewhere else get marked unread.
	viewMu  sync.Mutex
	viewing string
	// Whether Phoenix is the window in front, so Windows notifications
	// only appear when you're somewhere else.
	active bool

	upd     *updater
	pending pendingAttachments
}

// Hooks are the window features the backend uses. main.go fills them in
// with Wails; tests leave them empty.
type Hooks struct {
	Quit      func()                            // close the window (to restart for an update)
	PickFiles func() ([]string, error)          // Windows' open-file dialog
	Focus     func()                            // bring the window to the front
	SaveFile  func(name string) (string, error) // Windows' save-file dialog
}

// NewApp creates the app. Call Start before using it.
func NewApp() *App {
	return &App{emit: func(string, any) {}, active: true, pending: pendingAttachments{items: map[string]Attachment{}}}
}

// SetHooks connects the backend to the window.
func SetHooks(a *App, h Hooks) { a.hooks = h }

// Start wires the app up. emit sends events to the window.
// It is a package function (not a method) so Wails doesn't expose it to JS.
func Start(a *App, ctx context.Context, emit func(event string, data any)) error {
	a.ctx = ctx
	if emit != nil {
		a.emit = emit
	}
	p, err := resolvePaths()
	if err != nil {
		return fmt.Errorf("could not create the Phoenix AI data folder: %w", err)
	}
	a.paths = p
	a.settings = loadSettings(p)
	a.http = &http.Client{Timeout: 0} // downloads can take a long time; requests use contexts
	a.engine = newEngineManager(a)
	a.runner = newRunner(a)
	a.media = newMediaRunner(a)
	a.speech = newSpeechRunner(a)
	a.dl = newDownloadManager(a)
	a.chats = newChatStore(p.Chats)
	cleanupPending(p.Pending)
	cleanupVoice(p.Voice)
	a.upd = newUpdater(a)
	go func() {
		select {
		case <-time.After(5 * time.Second):
			a.updateOldTools()
		case <-ctx.Done():
		}
	}()
	if UpdateRepo != "" && !isDevVersion(Version) && os.Getenv("PHOENIX_NO_UPDATE") == "" {
		go a.upd.loop(ctx)
	}
	return nil
}

// Stop shuts the model down. Called when the window closes.
func Stop(a *App) {
	a.StopGeneration()
	if a.dl != nil {
		a.dl.cancelAll()
	}
	if a.runner != nil {
		a.runner.stop()
	}
	if a.media != nil {
		a.media.stop()
	}
	if a.speech != nil {
		a.speech.stop()
	}
	if a.upd != nil {
		a.upd.installOnExit()
	}
}

func (a *App) getSettings() Settings {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	return a.settings
}

func (a *App) updateSettings(fn func(s *Settings)) error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	fn(&a.settings)
	return saveSettings(a.paths, a.settings)
}

// AppInfo is the first thing the window asks for.
type AppInfo struct {
	Version   string `json:"version"`
	DataDir   string `json:"dataDir"`
	Platform  string `json:"platform"`
	MaxChats  int    `json:"maxChats"`
	MaxModels int    `json:"maxModels"`
	MaxFiles  int    `json:"maxFiles"`
}

// GetAppInfo returns the version, limits and where data is stored.
func (a *App) GetAppInfo() AppInfo {
	return AppInfo{
		Version: Version, DataDir: a.paths.Root, Platform: runtime.GOOS,
		MaxChats: MaxChats, MaxModels: MaxModels, MaxFiles: MaxAttachments,
	}
}

// GetSettings returns the user's settings.
func (a *App) GetSettings() Settings { return a.getSettings() }

// SaveSettings stores the settings the user can change in the Settings screen.
func (a *App) SaveSettings(in Settings) (Settings, error) {
	err := a.updateSettings(func(s *Settings) {
		s.UseGPU = in.UseGPU
		s.ContextSize = clampContext(in.ContextSize)
		s.SystemPrompt = in.SystemPrompt
		s.Thinking = in.Thinking
		s.AutoUpdate = in.AutoUpdate
		s.Notifications = in.Notifications
		if in.Theme == "dark" || in.Theme == "light" || in.Theme == "system" {
			s.Theme = in.Theme
		}
		if _, ok := kokoroVoices[in.Voice]; ok {
			s.Voice = in.Voice
		}
		if in.VoiceSpeed >= 0.7 && in.VoiceSpeed <= 1.5 {
			s.VoiceSpeed = in.VoiceSpeed
		}
	})
	return a.getSettings(), err
}

// OpenDataFolder opens the Phoenix AI folder in File Explorer.
func (a *App) OpenDataFolder() error {
	return openInFileManager(a.paths.Root)
}

func openInFileManager(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	// explorer.exe returns exit code 1 even on success, so only report start errors.
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

// throttle returns true at most once per interval.
type throttle struct {
	last     time.Time
	interval time.Duration
}

func (t *throttle) ok() bool {
	now := time.Now()
	if now.Sub(t.last) < t.interval {
		return false
	}
	t.last = now
	return true
}
