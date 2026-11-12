package backend

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// InstalledModel is a model file on this PC.
type InstalledModel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Maker       string `json:"maker"`
	Params      string `json:"params"`
	Description string `json:"description"`
	Repo        string `json:"repo"`
	File        string `json:"file"`
	Quant       string `json:"quant"`
	Size        int64  `json:"size"`
	CatalogID   string `json:"catalogId"`
	Fit         Fit    `json:"fit"`
	// Vision: its image support is downloaded, so it can see pictures.
	// VisionAvailable: it could see pictures after a VisionSize download.
	Vision          bool  `json:"vision"`
	VisionAvailable bool  `json:"visionAvailable"`
	VisionSize      int64 `json:"visionSize,omitempty"`
	// Kind is chat, image, video, listen, speakers or voice.
	Kind string `json:"kind"`
	// Ready is false while parts or the engine it needs are still missing.
	Ready bool `json:"ready"`
	// Missing says what's missing, for the Models page.
	Missing string `json:"missing,omitempty"`
}

// CatalogEntry is a recommended model plus whether it's on this PC already.
type CatalogEntry struct {
	CatalogModel
	Installed   bool   `json:"installed"`
	InstalledID string `json:"installedId"`
	Fit         Fit    `json:"fit"`
	TotalSize   int64  `json:"totalSize"` // with parts and image support
	Kind        string `json:"kind"`
}

// findVisionFile returns the image-support file next to a model, or "".
func findVisionFile(modelPath string) string {
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(modelPath), "*.gguf"))
	names := []string{}
	for _, m := range matches {
		names = append(names, filepath.Base(m))
	}
	if v := pickVisionFile(names); v != "" {
		return filepath.Join(filepath.Dir(modelPath), v)
	}
	return ""
}

// GetCatalog returns the recommended models with how well each runs here.
func (a *App) GetCatalog() []CatalogEntry {
	sys := a.sysSnapshot()
	out := make([]CatalogEntry, 0, len(catalog))
	for _, m := range catalog {
		e := CatalogEntry{CatalogModel: m, Fit: fitForKind(m.kindOf(), m.totalSize(), sys), TotalSize: m.totalSize(), Kind: m.kindOf()}
		id := modelID(m.Repo, m.File)
		if p, ok := safeJoin(a.paths.Models, id); ok {
			if _, err := os.Stat(p); err == nil {
				e.Installed, e.InstalledID = true, id
			}
		}
		out = append(out, e)
	}
	return out
}

// isMainModelFile says whether a file in the models folder is a model you
// can pick (not image support, not a part of another model).
func isMainModelFile(repo, file string) bool {
	if isVisionFile(file) || !hasModelExt(file) {
		return false
	}
	if _, _, ok := partOf(repo, file); ok {
		return false
	}
	if _, ok := catalogByFile(repo, file); ok {
		return true
	}
	// Anything else from Hugging Face is a GGUF chat model.
	return strings.HasSuffix(strings.ToLower(file), ".gguf")
}

// modelComplete says whether a recommended model has everything it needs.
func (a *App) modelComplete(m CatalogModel) bool {
	return a.missingFor(m) == ""
}

// missingFor says what a recommended model still needs, or "".
func (a *App) missingFor(m CatalogModel) string {
	for _, p := range m.Parts {
		if fp, ok := safeJoin(a.paths.Models, modelID(p.Repo, p.File)); !ok || !fileExists(fp) {
			return p.Name
		}
	}
	if key := toolForKind(m.kindOf()); key != "" && a.toolPath(key) == "" {
		return mustTool(key).Name
	}
	return ""
}

// GetInstalledModels lists every model downloaded to this PC.
func (a *App) GetInstalledModels() []InstalledModel {
	sys := a.sysSnapshot()
	out := []InstalledModel{}
	filepath.WalkDir(a.paths.Models, func(p string, de os.DirEntry, err error) error {
		if err != nil || de.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(a.paths.Models, p)
		if err != nil {
			return nil
		}
		id := filepath.ToSlash(rel)
		parts := strings.SplitN(id, "/", 2)
		if len(parts) != 2 {
			return nil
		}
		repo, file := repoFromDir(parts[0]), parts[1]
		if !isMainModelFile(repo, file) {
			return nil
		}
		info, err := de.Info()
		if err != nil {
			return nil
		}
		m := InstalledModel{
			ID: id, Repo: repo, File: file, Size: info.Size(), Quant: quantOf(file),
			Name: displayName(repo, file), Fit: fitFor(info.Size(), sys), Kind: KindChat, Ready: true,
		}
		if c, ok := catalogByFile(repo, file); ok {
			m.Name, m.Maker, m.Params, m.Description, m.CatalogID = c.Name, c.Maker, c.Params, c.Description, c.ID
			m.Kind = c.kindOf()
			m.Size = c.totalSize()
			m.Fit = fitForKind(m.Kind, m.Size, sys)
			if miss := a.missingFor(c); miss != "" {
				m.Ready, m.Missing = false, miss
			}
		} else {
			m.Maker = strings.SplitN(repo, "/", 2)[0]
		}
		if m.Kind == KindChat {
			m.Vision = findVisionFile(p) != ""
			if c, ok := catalogByRepo(repo); ok && c.Vision != "" && !m.Vision {
				m.VisionAvailable, m.VisionSize = true, c.VisionSize
			}
		}
		out = append(out, m)
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Size < out[j].Size })
	return out
}

// installedOfKind returns the path of the first downloaded model of a
// kind that has everything it needs, or "".
func (a *App) installedOfKind(kind string) (string, CatalogModel) {
	for _, m := range catalog {
		if m.kindOf() != kind {
			continue
		}
		if p, ok := safeJoin(a.paths.Models, modelID(m.Repo, m.File)); ok && fileExists(p) && a.missingFor(m) == "" {
			return p, m
		}
	}
	return "", CatalogModel{}
}

// modelSlotsUsedLocked counts the models on this PC plus unfinished
// downloads, leaving out `exclude`. Image support, parts and the voice
// helpers don't count. The caller holds a.dl.mu.
func (a *App) modelSlotsUsedLocked(exclude string) int {
	ids := map[string]bool{}
	counts := func(id string) bool {
		parts := strings.SplitN(id, "/", 2)
		if len(parts) != 2 {
			return false
		}
		repo, file := repoFromDir(parts[0]), parts[1]
		if !isMainModelFile(repo, file) {
			return false
		}
		if c, ok := catalogByFile(repo, file); ok {
			return countsTowardLimit(c.kindOf())
		}
		return true
	}
	for id, j := range a.dl.active {
		if !j.state.Tool && !j.state.Part && counts(id) {
			ids[id] = true
		}
	}
	filepath.WalkDir(a.paths.Models, func(p string, de os.DirEntry, err error) error {
		if err != nil || de.IsDir() {
			return nil
		}
		full := strings.TrimSuffix(p, ".part")
		if rel, err := filepath.Rel(a.paths.Models, full); err == nil && counts(filepath.ToSlash(rel)) {
			ids[filepath.ToSlash(rel)] = true
		}
		return nil
	})
	delete(ids, exclude)
	return len(ids)
}

// DeleteModel removes a downloaded model from this PC.
func (a *App) DeleteModel(id string) error {
	p, err := a.modelPath(id)
	if err != nil {
		return err
	}
	if a.runner.loadedID() == id {
		a.StopGeneration()
		a.runner.stop()
	}
	parts := strings.SplitN(id, "/", 2)
	repo, file := repoFromDir(parts[0]), parts[1]
	cat, isCat := catalogByFile(repo, file)
	if isCat {
		// Stop whichever engine has it loaded so Windows lets us delete it.
		switch cat.kindOf() {
		case KindImage, KindVideo:
			a.StopGeneration()
			a.media.stop()
		case KindListen, KindSpeakers:
			a.speech.stop()
		}
	}
	if isCat && cat.Archive != nil {
		// The voice is a folder.
		if err := os.RemoveAll(filepath.Join(a.paths.Models, parts[0], cat.Archive.Folder)); err != nil {
			return errors.New("couldn't delete the voice. Close anything that might be using it and try again")
		}
	} else if err := os.Remove(p); err != nil {
		return errors.New("couldn't delete the model file. Close anything that might be using it and try again")
	}
	// Its parts go too, unless another downloaded model uses them.
	if isCat {
		for _, part := range cat.Parts {
			if a.partInUse(part, cat.ID) {
				continue
			}
			if pp, ok := safeJoin(a.paths.Models, modelID(part.Repo, part.File)); ok {
				os.Remove(pp)
				removeEmptyDirs(filepath.Dir(pp), a.paths.Models)
			}
		}
	}
	// Its image support goes too, unless another version of the same model uses it.
	if v := findVisionFile(p); v != "" {
		others := false
		if matches, _ := filepath.Glob(filepath.Join(filepath.Dir(p), "*.gguf")); len(matches) > 0 {
			for _, m := range matches {
				if !isVisionFile(m) {
					others = true
				}
			}
		}
		if !others {
			matches, _ := filepath.Glob(filepath.Join(filepath.Dir(p), "*mmproj*"))
			for _, m := range matches {
				os.Remove(m)
			}
		}
	}
	removeEmptyDirs(filepath.Dir(p), a.paths.Models)
	if a.getSettings().LastModel == id {
		_ = a.updateSettings(func(s *Settings) { s.LastModel = "" })
	}
	a.emit("models:changed", nil)
	return nil
}

// partInUse says whether a model other than `except` that's downloaded uses a part.
func (a *App) partInUse(part ModelPart, except string) bool {
	for _, m := range catalog {
		if m.ID == except {
			continue
		}
		for _, p := range m.Parts {
			if p.Repo == part.Repo && p.File == part.File {
				if mp, ok := safeJoin(a.paths.Models, modelID(m.Repo, m.File)); ok && fileExists(mp) {
					return true
				}
			}
		}
	}
	return false
}

func (a *App) modelPath(id string) (string, error) {
	if id == "" || !validModelFile(id) {
		return "", errors.New("pick a model first")
	}
	p, ok := safeJoin(a.paths.Models, id)
	if !ok {
		return "", errors.New("that model path isn't allowed")
	}
	if _, err := os.Stat(p); err != nil {
		return "", errors.New("that model isn't downloaded anymore")
	}
	return p, nil
}

// modelKind says what a downloaded model does.
func (a *App) modelKind(id string) string {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) == 2 {
		if c, ok := catalogByFile(repoFromDir(parts[0]), parts[1]); ok {
			return c.kindOf()
		}
	}
	return KindChat
}

func (a *App) modelName(id string) string {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 {
		return id
	}
	repo, file := repoFromDir(parts[0]), parts[1]
	if c, ok := catalogByFile(repo, file); ok {
		return c.Name
	}
	return displayName(repo, file)
}

var trailingQuant = regexp.MustCompile(`(?i)[-_.](UD[-_])?(IQ\d_[A-Z]+|Q\d_K_[A-Z]+|Q\d_K|Q\d_\d|Q\d|BF16|F16|F32|MXFP4(_MOE)?)$`)

// displayName turns "Llama-3.2-3B-Instruct-Q4_K_M.gguf" into "Llama 3.2 3B Instruct".
func displayName(repo, file string) string {
	if c, ok := catalogByFile(repo, file); ok {
		return c.Name
	}
	base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	for i := 0; i < 2; i++ {
		base = trailingQuant.ReplaceAllString(base, "")
	}
	base = strings.NewReplacer("-", " ", "_", " ").Replace(base)
	base = strings.Join(strings.Fields(base), " ")
	if base == "" {
		return file
	}
	return base
}
