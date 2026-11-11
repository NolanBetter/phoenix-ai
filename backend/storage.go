package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Paths are the folders Phoenix AI uses. Everything lives under one folder:
// %LOCALAPPDATA%\PhoenixAI on Windows, kept apart from the user's own files.
type Paths struct {
	Root    string
	Models  string
	Engine  string
	Chats   string
	Updates string
}

func resolvePaths() (*Paths, error) {
	root := os.Getenv("PHOENIX_HOME")
	if root == "" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			if d, err := os.UserConfigDir(); err == nil {
				base = d
			} else {
				base = os.TempDir()
			}
		}
		root = filepath.Join(base, "PhoenixAI")
	}
	p := &Paths{
		Root:    root,
		Models:  filepath.Join(root, "models"),
		Engine:  filepath.Join(root, "engine"),
		Chats:   filepath.Join(root, "chats"),
		Updates: filepath.Join(root, "updates"),
	}
	for _, d := range []string{p.Root, p.Models, p.Engine, p.Chats, p.Updates} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// Settings are saved to settings.json.
type Settings struct {
	UseGPU       bool   `json:"useGPU"`
	ContextSize  int    `json:"contextSize"`
	SystemPrompt string `json:"systemPrompt"`
	Thinking     bool   `json:"thinking"`
	LastModel    string `json:"lastModel"`
	Theme        string `json:"theme"`
	AutoUpdate   bool   `json:"autoUpdate"`

	// Managed by Phoenix, not the user.
	EngineTag string `json:"engineTag"`
	EngineExe string `json:"engineExe"`
}

func defaultSettings() Settings {
	return Settings{UseGPU: true, ContextSize: 8192, Theme: "system", AutoUpdate: true}
}

func clampContext(n int) int {
	switch {
	case n <= 0:
		return 8192
	case n < 2048:
		return 2048
	case n > 131072:
		return 131072
	}
	return n
}

func loadSettings(p *Paths) Settings {
	s := defaultSettings()
	b, err := os.ReadFile(filepath.Join(p.Root, "settings.json"))
	if err == nil {
		_ = json.Unmarshal(b, &s)
	}
	s.ContextSize = clampContext(s.ContextSize)
	if s.Theme == "" {
		s.Theme = "system"
	}
	return s
}

func saveSettings(p *Paths, s Settings) error {
	return writeJSONAtomic(filepath.Join(p.Root, "settings.json"), s)
}

// writeJSONAtomic writes to a temp file then renames it, so a crash
// never leaves a half-written file behind.
func writeJSONAtomic(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// safeJoin joins a relative path onto root and refuses anything that
// would escape root (like "..\..\Windows").
func safeJoin(root, rel string) (string, bool) {
	rel = filepath.FromSlash(rel)
	if filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" {
		return "", false
	}
	full := filepath.Join(root, rel)
	r, err := filepath.Rel(root, full)
	if err != nil || r == "." || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", false
	}
	return full, true
}
