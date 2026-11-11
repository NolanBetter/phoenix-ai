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
}

// CatalogEntry is a recommended model plus whether it's on this PC already.
type CatalogEntry struct {
	CatalogModel
	Installed   bool   `json:"installed"`
	InstalledID string `json:"installedId"`
	Fit         Fit    `json:"fit"`
}

// GetCatalog returns the recommended models with how well each runs here.
func (a *App) GetCatalog() []CatalogEntry {
	sys := a.sysSnapshot()
	out := make([]CatalogEntry, 0, len(catalog))
	for _, m := range catalog {
		e := CatalogEntry{CatalogModel: m, Fit: fitFor(m.Size, sys)}
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

// GetInstalledModels lists every model downloaded to this PC.
func (a *App) GetInstalledModels() []InstalledModel {
	sys := a.sysSnapshot()
	out := []InstalledModel{}
	filepath.WalkDir(a.paths.Models, func(p string, de os.DirEntry, err error) error {
		if err != nil || de.IsDir() {
			return nil
		}
		name := de.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".gguf") || strings.Contains(strings.ToLower(name), "mmproj") {
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
		info, err := de.Info()
		if err != nil {
			return nil
		}
		m := InstalledModel{
			ID: id, Repo: repo, File: file, Size: info.Size(), Quant: quantOf(file),
			Name: displayName(repo, file), Fit: fitFor(info.Size(), sys),
		}
		if c, ok := catalogByFile(repo, file); ok {
			m.Name, m.Maker, m.Params, m.Description, m.CatalogID = c.Name, c.Maker, c.Params, c.Description, c.ID
		} else {
			m.Maker = strings.SplitN(repo, "/", 2)[0]
		}
		out = append(out, m)
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Size < out[j].Size })
	return out
}

// modelSlotsUsedLocked counts the models on this PC plus unfinished
// downloads, leaving out `exclude`. The caller holds a.dl.mu.
func (a *App) modelSlotsUsedLocked(exclude string) int {
	ids := map[string]bool{}
	for id := range a.dl.active {
		ids[id] = true
	}
	filepath.WalkDir(a.paths.Models, func(p string, de os.DirEntry, err error) error {
		if err != nil || de.IsDir() {
			return nil
		}
		name := strings.ToLower(de.Name())
		if strings.Contains(name, "mmproj") {
			return nil
		}
		full := p
		switch {
		case strings.HasSuffix(name, ".gguf"):
		case strings.HasSuffix(name, ".gguf.part"):
			full = strings.TrimSuffix(p, ".part")
		default:
			return nil
		}
		if rel, err := filepath.Rel(a.paths.Models, full); err == nil {
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
	if err := os.Remove(p); err != nil {
		return errors.New("couldn't delete the model file. Close anything that might be using it and try again")
	}
	removeEmptyDirs(filepath.Dir(p), a.paths.Models)
	if a.getSettings().LastModel == id {
		_ = a.updateSettings(func(s *Settings) { s.LastModel = "" })
	}
	a.emit("models:changed", nil)
	return nil
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
