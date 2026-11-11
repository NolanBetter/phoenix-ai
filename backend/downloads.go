package backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const hubBase = "https://huggingface.co"

// DownloadState is sent to the window as "download:progress" events.
type DownloadState struct {
	ID        string  `json:"id"`
	CatalogID string  `json:"catalogId"`
	Name      string  `json:"name"`
	Repo      string  `json:"repo"`
	File      string  `json:"file"`
	Received  int64   `json:"received"`
	Total     int64   `json:"total"`
	Speed     float64 `json:"speed"`
	Status    string  `json:"status"` // downloading, verifying, paused, done, error
	Error     string  `json:"error"`
}

type dlJob struct {
	state  DownloadState
	cancel context.CancelCauseFunc
	// vision is the model's image-support file, downloaded right after the
	// model itself. Filled in from the catalog, or from Hugging Face's file list.
	vision string
}

// isVisionFile reports whether a file is a model's image support (an
// "mmproj"), not a model. They don't count toward the model limit.
func isVisionFile(file string) bool {
	return strings.Contains(strings.ToLower(filepath.Base(file)), "mmproj")
}

// pickVisionFile chooses the image-support file from a repository's files:
// F16 if there is one (best quality for its size), otherwise the next best.
func pickVisionFile(names []string) string {
	best, bestScore := "", 0
	for _, n := range names {
		l := strings.ToLower(n)
		if !isVisionFile(n) || !strings.HasSuffix(l, ".gguf") || strings.Contains(n, "/") {
			continue
		}
		score := 1
		switch {
		case strings.Contains(l, "bf16"):
			score = 4
		case strings.Contains(l, "f16"):
			score = 5
		case strings.Contains(l, "q8"):
			score = 3
		case strings.Contains(l, "f32"):
			score = 0
		}
		if score > bestScore || best == "" {
			best, bestScore = n, score
		}
	}
	return best
}

// visionName is how an image-support download is shown.
func visionName(repo string) string {
	if c, ok := catalogByRepo(repo); ok {
		return "Image support for " + c.Name
	}
	name := repo[strings.Index(repo, "/")+1:]
	return "Image support for " + strings.TrimSuffix(name, "-GGUF")
}

// errUserCanceled marks a download the user canceled (its partial file is
// deleted). Downloads stopped because Phoenix is closing are kept to resume.
var errUserCanceled = errors.New("canceled by user")

type downloadManager struct {
	a      *App
	mu     sync.Mutex
	active map[string]*dlJob
}

func newDownloadManager(a *App) *downloadManager {
	return &downloadManager{a: a, active: map[string]*dlJob{}}
}

func (d *downloadManager) cancelAll() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, j := range d.active {
		j.cancel(context.Canceled)
	}
}

var (
	repoPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)
	splitShard  = regexp.MustCompile(`(?i)-\d{5}-of-\d{5}\.gguf$`)
)

func repoDir(repo string) string { return strings.ReplaceAll(repo, "/", "__") }

func repoFromDir(dir string) string { return strings.Replace(dir, "__", "/", 1) }

func modelID(repo, file string) string { return repoDir(repo) + "/" + file }

func validModelFile(file string) bool {
	if !strings.HasSuffix(strings.ToLower(file), ".gguf") || strings.Contains(file, "\\") {
		return false
	}
	for _, part := range strings.Split(file, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func hubFileURL(repo, file string) string {
	parts := strings.Split(file, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return fmt.Sprintf("%s/%s/resolve/main/%s?download=true", hubBase, repo, strings.Join(parts, "/"))
}

// DownloadModel downloads one of the recommended models.
func (a *App) DownloadModel(catalogID string) error {
	m, ok := findCatalog(catalogID)
	if !ok {
		return errors.New("that model isn't in the list")
	}
	return a.dl.start(m.Repo, m.File, m.Name, m.ID, m.Vision)
}

// DownloadFromHub downloads any GGUF file from a Hugging Face repository.
// It also resumes a download that was interrupted.
func (a *App) DownloadFromHub(repo, file string) error {
	if isVisionFile(file) {
		return a.dl.start(repo, file, visionName(repo), "", "")
	}
	if cat, ok := catalogByFile(repo, file); ok {
		return a.dl.start(repo, file, cat.Name, cat.ID, cat.Vision)
	}
	return a.dl.start(repo, file, displayName(repo, file), "", "")
}

// AddImageSupport downloads the image-support file for a model that's
// already on this PC, so it can see pictures.
func (a *App) AddImageSupport(modelID string) error {
	if _, err := a.modelPath(modelID); err != nil {
		return err
	}
	repo := repoFromDir(strings.SplitN(modelID, "/", 2)[0])
	file := ""
	if c, ok := catalogByRepo(repo); ok {
		file = c.Vision
	} else {
		info, err := fetchRepoInfo(a.ctx, a.http, repo)
		if err != nil {
			return err
		}
		names := make([]string, 0, len(info.Siblings))
		for _, s := range info.Siblings {
			names = append(names, s.Name)
		}
		file = pickVisionFile(names)
	}
	if file == "" {
		return errors.New("this model can't see images")
	}
	return a.dl.start(repo, file, visionName(repo), "", "")
}

// CancelDownload stops a download and deletes what was downloaded so far.
func (a *App) CancelDownload(id string) error {
	a.dl.mu.Lock()
	j, ok := a.dl.active[id]
	a.dl.mu.Unlock()
	if ok {
		j.cancel(errUserCanceled)
		return nil
	}
	// A paused download from an earlier session: just delete the partial file.
	if p, ok := safeJoin(a.paths.Models, id); ok {
		os.Remove(p + ".part")
		removeEmptyDirs(filepath.Dir(p), a.paths.Models)
	}
	a.emit("download:progress", DownloadState{ID: id, Status: "canceled"})
	return nil
}

// GetDownloads lists running downloads and unfinished ones from earlier.
func (a *App) GetDownloads() []DownloadState {
	a.dl.mu.Lock()
	out := []DownloadState{}
	seen := map[string]bool{}
	for id, j := range a.dl.active {
		out = append(out, j.state)
		seen[id] = true
	}
	a.dl.mu.Unlock()

	filepath.WalkDir(a.paths.Models, func(p string, de os.DirEntry, err error) error {
		if err != nil || de.IsDir() || !strings.HasSuffix(de.Name(), ".gguf.part") {
			return nil
		}
		rel, err := filepath.Rel(a.paths.Models, strings.TrimSuffix(p, ".part"))
		if err != nil {
			return nil
		}
		id := filepath.ToSlash(rel)
		if seen[id] {
			return nil
		}
		parts := strings.SplitN(id, "/", 2)
		if len(parts) != 2 {
			return nil
		}
		repo, file := repoFromDir(parts[0]), parts[1]
		info, _ := de.Info()
		st := DownloadState{ID: id, Repo: repo, File: file, Name: displayName(repo, file), Status: "paused"}
		if info != nil {
			st.Received = info.Size()
		}
		if c, ok := catalogByFile(repo, file); ok {
			st.CatalogID, st.Name, st.Total = c.ID, c.Name, c.Size
		} else if isVisionFile(file) {
			st.Name = visionName(repo)
			if c, ok := catalogByRepo(repo); ok && c.Vision == file {
				st.Total = c.VisionSize
			}
		}
		out = append(out, st)
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (d *downloadManager) start(repo, file, name, catalogID, vision string) error {
	if !repoPattern.MatchString(repo) || !validModelFile(file) {
		return errors.New("that doesn't look like a valid model file")
	}
	id := modelID(repo, file)
	final, ok := safeJoin(d.a.paths.Models, id)
	if !ok {
		return errors.New("that doesn't look like a valid model file")
	}
	companion := isVisionFile(file)
	if _, err := os.Stat(final); err == nil {
		if companion {
			return errors.New("image support is already downloaded")
		}
		return errors.New("this model is already downloaded")
	}

	d.mu.Lock()
	if _, busy := d.active[id]; busy {
		d.mu.Unlock()
		return errors.New("this is already downloading")
	}
	if !companion && d.a.modelSlotsUsedLocked(id) >= MaxModels {
		d.mu.Unlock()
		return fmt.Errorf("you can keep up to %d models. Delete one on the Models page to download another", MaxModels)
	}
	ctx, cancel := context.WithCancelCause(d.a.ctx)
	j := &dlJob{cancel: cancel, vision: vision, state: DownloadState{
		ID: id, CatalogID: catalogID, Name: name, Repo: repo, File: file, Status: "downloading",
	}}
	if c, ok := findCatalog(catalogID); ok {
		j.state.Total = c.Size
		if companion {
			j.state.Total = c.VisionSize
		}
	}
	d.active[id] = j
	d.mu.Unlock()
	d.a.emit("download:progress", j.state)

	go func() {
		err := d.run(ctx, j, final)
		d.mu.Lock()
		delete(d.active, id)
		st := j.state
		d.mu.Unlock()
		userCanceled := errors.Is(context.Cause(ctx), errUserCanceled)
		cancel(nil)
		switch {
		case err == nil:
			st.Status, st.Received = "done", st.Total
			d.a.emit("download:progress", st)
			d.a.emit("models:changed", nil)
			// Now fetch the model's image support, if it has one.
			if v := j.vision; v != "" && !companion {
				if p, ok := safeJoin(d.a.paths.Models, modelID(repo, v)); ok {
					if _, err := os.Stat(p); err != nil {
						_ = d.start(repo, v, visionName(repo), catalogID, "")
					}
				}
			}
			if !companion {
				d.a.notifyIfAway(st.Name+" is ready", "It finished downloading. Click to start chatting.", "models")
			}
		case userCanceled:
			os.Remove(final + ".part")
			removeEmptyDirs(filepath.Dir(final), d.a.paths.Models)
			st.Status = "canceled"
			d.a.emit("download:progress", st)
		case ctx.Err() != nil:
			st.Status, st.Speed = "paused", 0
			d.a.emit("download:progress", st)
		default:
			st.Status, st.Error = "error", err.Error()
			d.a.emit("download:progress", st)
		}
	}()
	return nil
}

func (d *downloadManager) update(j *dlJob, fn func(s *DownloadState)) {
	d.mu.Lock()
	fn(&j.state)
	st := j.state
	d.mu.Unlock()
	d.a.emit("download:progress", st)
}

func (d *downloadManager) run(ctx context.Context, j *dlJob, final string) error {
	repo, file := j.state.Repo, j.state.File
	part := final + ".part"

	// Ask Hugging Face for the exact size and checksum.
	info, vision, err := hubFileInfo(ctx, d.a.http, repo, file)
	if err != nil && !errors.Is(err, errNoInfo) {
		return err
	}
	if j.vision == "" && !isVisionFile(file) && vision != "" {
		d.mu.Lock()
		j.vision = vision
		d.mu.Unlock()
	}
	if info.Size > 0 {
		d.update(j, func(s *DownloadState) { s.Total = info.Size })
	}

	var have int64
	if st, err := os.Stat(part); err == nil {
		have = st.Size()
	}
	if info.Size > 0 {
		free := diskFree(d.a.paths.Models)
		need := info.Size - have + 256<<20
		if free > 0 && free < need {
			return fmt.Errorf("not enough disk space: this model needs %s and only %s is free", humanBytes(need), humanBytes(free))
		}
	}
	if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
		return err
	}

	var speed float64
	lastBytes, lastTime := int64(-1), time.Now()
	th := throttle{interval: 250 * time.Millisecond}
	progress := func(got, total int64) {
		if !th.ok() {
			return
		}
		now := time.Now()
		if lastBytes >= 0 {
			dt := now.Sub(lastTime).Seconds()
			if dt > 0 {
				inst := float64(got-lastBytes) / dt
				if speed == 0 {
					speed = inst
				} else {
					speed = speed*0.8 + inst*0.2
				}
			}
		}
		lastBytes, lastTime = got, now
		d.update(j, func(s *DownloadState) {
			s.Received, s.Speed = got, speed
			if total > 0 {
				s.Total = total
			}
		})
	}

	var h hash.Hash
	src := hubFileURL(repo, file)
	for attempt := 0; ; attempt++ {
		if info.SHA256 != "" {
			h = sha256.New()
		}
		err = downloadFile(ctx, d.a.http, src, part, true, progress, h)
		if err == nil || ctx.Err() != nil || attempt >= 5 || isPermanent(err) {
			break
		}
		// Network hiccup: wait a little, then resume where we left off.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(2+attempt*3) * time.Second):
		}
	}
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}

	d.update(j, func(s *DownloadState) { s.Status = "verifying" })
	st, err := os.Stat(part)
	if err != nil {
		return err
	}
	if info.Size > 0 && st.Size() != info.Size {
		os.Remove(part)
		return fmt.Errorf("the download was incomplete (%s of %s). Please try again", humanBytes(st.Size()), humanBytes(info.Size))
	}
	if h != nil && info.SHA256 != "" {
		if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, info.SHA256) {
			os.Remove(part)
			return errors.New("the downloaded file was damaged (checksum didn't match). Please try again")
		}
	}
	if !isGGUF(part) {
		os.Remove(part)
		return errors.New("the downloaded file isn't a model file Phoenix can use")
	}
	return os.Rename(part, final)
}

type permanentError struct{ msg string }

func (e permanentError) Error() string { return e.msg }

func isPermanent(err error) bool {
	var p permanentError
	return errors.As(err, &p)
}

// downloadFile downloads url into dest. With resume it continues a partial
// file. If h is set, the whole file (including the already-downloaded part)
// is hashed while it downloads.
func downloadFile(ctx context.Context, client *http.Client, src, dest string, resume bool, progress func(got, total int64), h hash.Hash) error {
	flags := os.O_CREATE | os.O_WRONLY
	var off int64
	if resume {
		if st, err := os.Stat(dest); err == nil {
			off = st.Size()
		}
	} else {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(dest, flags, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	if off > 0 && h != nil {
		if err := hashPrefix(dest, off, h); err != nil {
			return err
		}
	}

	// Cancel the request if no data arrives for 60 seconds.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stall := time.AfterFunc(60*time.Second, cancel)
	defer stall.Stop()

	req, err := http.NewRequestWithContext(ctx, "GET", src, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "PhoenixAI/"+Version)
	if off > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", off))
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && off > 0:
		return nil // already complete
	case resp.StatusCode == http.StatusPartialContent && off > 0:
		if _, err := f.Seek(off, io.SeekStart); err != nil {
			return err
		}
	case resp.StatusCode == http.StatusOK:
		// Server ignored the range (or this is a fresh download): start over.
		if off > 0 {
			if err := f.Truncate(0); err != nil {
				return err
			}
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				return err
			}
			if h != nil {
				h.Reset()
			}
			off = 0
		}
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		return permanentError{"this model requires accepting its license on huggingface.co first, so Phoenix can't download it"}
	case resp.StatusCode == 404:
		return permanentError{"that file no longer exists on Hugging Face"}
	default:
		return fmt.Errorf("the server answered with status %d", resp.StatusCode)
	}

	total := int64(-1)
	if resp.ContentLength >= 0 {
		total = off + resp.ContentLength
	}
	var w io.Writer = f
	if h != nil {
		w = io.MultiWriter(f, h)
	}
	buf := make([]byte, 1<<20)
	got := off
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			stall.Reset(60 * time.Second)
			if _, err := w.Write(buf[:n]); err != nil {
				return err
			}
			got += int64(n)
			if progress != nil {
				progress(got, total)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			// The caller checks whether the user canceled; a stall or a
			// dropped connection is retried and resumes from here.
			return rerr
		}
	}
	if total > 0 && got != total {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func hashPrefix(path string, n int64, h hash.Hash) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.CopyN(h, f, n)
	return err
}

// isGGUF checks the file starts with the GGUF magic bytes.
func isGGUF(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	b := make([]byte, 4)
	if _, err := io.ReadFull(f, b); err != nil {
		return false
	}
	return string(b) == "GGUF"
}

var errNoInfo = errors.New("no file info")

type hubFile struct {
	Size   int64
	SHA256 string
}

type hubRepoInfo struct {
	ID       string `json:"id"`
	Gated    any    `json:"gated"`
	Siblings []struct {
		Name string `json:"rfilename"`
		Size int64  `json:"size"`
		LFS  *struct {
			SHA256 string `json:"sha256"`
			Size   int64  `json:"size"`
		} `json:"lfs"`
	} `json:"siblings"`
}

func fetchRepoInfo(ctx context.Context, client *http.Client, repo string) (*hubRepoInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/api/models/%s?blobs=true", hubBase, repo), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "PhoenixAI/"+Version)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("couldn't reach Hugging Face. Check your internet connection")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 || resp.StatusCode == 401 {
		return nil, permanentError{"that model wasn't found on Hugging Face"}
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Hugging Face answered with status %d", resp.StatusCode)
	}
	var info hubRepoInfo
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

func isGated(v any) bool {
	switch g := v.(type) {
	case bool:
		return g
	case string:
		return g != "" && g != "false"
	}
	return false
}

// hubFileInfo returns a file's size and checksum, plus the repository's
// image-support file if it has one.
func hubFileInfo(ctx context.Context, client *http.Client, repo, file string) (hubFile, string, error) {
	info, err := fetchRepoInfo(ctx, client, repo)
	if err != nil {
		if isPermanent(err) {
			return hubFile{}, "", err
		}
		return hubFile{}, "", errNoInfo // carry on without a checksum
	}
	if isGated(info.Gated) {
		return hubFile{}, "", permanentError{"this model requires accepting its license on huggingface.co first, so Phoenix can't download it"}
	}
	names := make([]string, 0, len(info.Siblings))
	for _, s := range info.Siblings {
		names = append(names, s.Name)
	}
	vision := pickVisionFile(names)
	for _, s := range info.Siblings {
		if s.Name == file {
			hf := hubFile{Size: s.Size}
			if s.LFS != nil {
				hf.SHA256 = s.LFS.SHA256
				if s.LFS.Size > 0 {
					hf.Size = s.LFS.Size
				}
			}
			return hf, vision, nil
		}
	}
	return hubFile{}, "", permanentError{"that file no longer exists on Hugging Face"}
}

// HubModel is a search result from Hugging Face.
type HubModel struct {
	Repo      string `json:"repo"`
	Downloads int    `json:"downloads"`
	Likes     int    `json:"likes"`
	Updated   string `json:"updated"`
}

// SearchHub searches Hugging Face for models in the GGUF format Phoenix runs.
func (a *App) SearchHub(query string) ([]HubModel, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []HubModel{}, nil
	}
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	q := url.Values{}
	q.Set("search", query)
	q.Set("filter", "gguf")
	q.Set("sort", "downloads")
	q.Set("direction", "-1")
	q.Set("limit", "30")
	req, _ := http.NewRequestWithContext(ctx, "GET", hubBase+"/api/models?"+q.Encode(), nil)
	req.Header.Set("User-Agent", "PhoenixAI/"+Version)
	resp, err := a.http.Do(req)
	if err != nil {
		return nil, errors.New("couldn't reach Hugging Face. Check your internet connection")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Hugging Face answered with status %d", resp.StatusCode)
	}
	var raw []struct {
		ID           string `json:"id"`
		Downloads    int    `json:"downloads"`
		Likes        int    `json:"likes"`
		LastModified string `json:"lastModified"`
		CreatedAt    string `json:"createdAt"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&raw); err != nil {
		return nil, err
	}
	out := make([]HubModel, 0, len(raw))
	for _, r := range raw {
		if !repoPattern.MatchString(r.ID) {
			continue
		}
		upd := r.LastModified
		if upd == "" {
			upd = r.CreatedAt
		}
		out = append(out, HubModel{Repo: r.ID, Downloads: r.Downloads, Likes: r.Likes, Updated: upd})
	}
	return out, nil
}

// HubFile is one downloadable version (size/quality) of a model.
type HubFile struct {
	File        string `json:"file"`
	Size        int64  `json:"size"`
	Quant       string `json:"quant"`
	Fit         Fit    `json:"fit"`
	Installed   bool   `json:"installed"`
	Recommended bool   `json:"recommended"`
	Vision      bool   `json:"vision"` // the model can see images (its image support downloads too)
}

var quantPattern = regexp.MustCompile(`(?i)(UD-)?(IQ\d_[A-Z]+|Q\d_K_[A-Z]+|Q\d_K|Q\d_\d|Q\d|BF16|F16|F32|MXFP4(?:_MOE)?)`)

func quantOf(file string) string {
	base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	m := quantPattern.FindAllString(base, -1)
	if len(m) == 0 {
		return ""
	}
	return strings.ToUpper(m[len(m)-1])
}

// GetHubFiles lists the GGUF files in a Hugging Face repository.
func (a *App) GetHubFiles(repo string) ([]HubFile, error) {
	if !repoPattern.MatchString(repo) {
		return nil, errors.New("that isn't a valid Hugging Face model name")
	}
	info, err := fetchRepoInfo(a.ctx, a.http, repo)
	if err != nil {
		return nil, err
	}
	if isGated(info.Gated) {
		return nil, errors.New("this model requires accepting its license on huggingface.co, so Phoenix can't download it")
	}
	sys := a.sysSnapshot()
	out := []HubFile{}
	names := make([]string, 0, len(info.Siblings))
	for _, s := range info.Siblings {
		names = append(names, s.Name)
	}
	vision := pickVisionFile(names) != ""
	for _, s := range info.Siblings {
		name := s.Name
		lower := strings.ToLower(name)
		if !validModelFile(name) || strings.Contains(lower, "mmproj") || splitShard.MatchString(name) {
			continue
		}
		size := s.Size
		if s.LFS != nil && s.LFS.Size > 0 {
			size = s.LFS.Size
		}
		hf := HubFile{File: name, Size: size, Quant: quantOf(name), Fit: fitFor(size, sys), Vision: vision}
		if p, ok := safeJoin(a.paths.Models, modelID(repo, name)); ok {
			if _, err := os.Stat(p); err == nil {
				hf.Installed = true
			}
		}
		out = append(out, hf)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Size < out[j].Size })
	pickRecommended(out)
	return out, nil
}

// pickRecommended marks the usual best balance of size and quality.
func pickRecommended(files []HubFile) {
	for _, want := range []string{"Q4_K_M", "UD-Q4_K_XL", "Q4_K_S", "MXFP4_MOE", "MXFP4", "Q4_0"} {
		for i := range files {
			if files[i].Quant == want {
				files[i].Recommended = true
				return
			}
		}
	}
}

func removeEmptyDirs(dir, stop string) {
	for dir != stop && strings.HasPrefix(dir, stop) {
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}
