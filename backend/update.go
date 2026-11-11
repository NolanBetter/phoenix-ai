package backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Phoenix updates itself from the GitHub Releases page of UpdateRepo.
// A release must contain PhoenixAI-Setup-<version>.exe (the installer)
// plus a checksum: either GitHub's own asset digest or a
// PhoenixAI-Setup-<version>.exe.sha256 file. Phoenix won't run an
// installer it can't check.
//
// The installer is downloaded by Phoenix itself, not a browser, so
// Windows doesn't show the "unknown app" warning for updates.

var updateAPIBase = "https://api.github.com"

const (
	updateFirstCheck = 20 * time.Second
	updateEvery      = 6 * time.Hour
)

// UpdateStatus is sent to the window as "update:status" events.
type UpdateStatus struct {
	Current  string `json:"current"`
	Latest   string `json:"latest"`
	State    string `json:"state"` // idle, checking, uptodate, downloading, ready, installing, error, disabled
	Notes    string `json:"notes"`
	Received int64  `json:"received"`
	Total    int64  `json:"total"`
	Error    string `json:"error"`
	Checked  int64  `json:"checked"` // when Phoenix last asked GitHub (unix ms)
}

type updater struct {
	a        *App
	mu       sync.Mutex
	st       UpdateStatus
	ready    string // checked installer, waiting to run
	running  bool
	launched bool
}

func newUpdater(a *App) *updater {
	st := UpdateStatus{Current: Version, State: "idle"}
	if UpdateRepo == "" {
		st.State = "disabled"
	}
	return &updater{a: a, st: st}
}

func (u *updater) snapshot() UpdateStatus {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.st
}

func (u *updater) set(fn func(s *UpdateStatus)) {
	u.mu.Lock()
	fn(&u.st)
	st := u.st
	u.mu.Unlock()
	u.a.emit("update:status", st)
}

// GetUpdateStatus says whether an update is downloading or ready.
func (a *App) GetUpdateStatus() UpdateStatus { return a.upd.snapshot() }

// CheckForUpdates asks GitHub for a newer version and downloads it if
// there is one. Progress arrives as "update:status" events.
func (a *App) CheckForUpdates() (UpdateStatus, error) {
	if UpdateRepo == "" {
		return a.upd.snapshot(), errors.New("updates are turned off in this version of Phoenix")
	}
	err := a.upd.check(a.ctx)
	return a.upd.snapshot(), err
}

// InstallUpdate closes Phoenix, installs the downloaded update and opens
// Phoenix again.
func (a *App) InstallUpdate() error {
	u := a.upd
	u.mu.Lock()
	path := u.ready
	u.mu.Unlock()
	if path == "" {
		return errors.New("there's no update ready to install yet")
	}
	if err := launchInstaller(path, true); err != nil {
		return fmt.Errorf("couldn't start the update: %w", err)
	}
	u.mu.Lock()
	u.launched = true
	u.mu.Unlock()
	u.set(func(s *UpdateStatus) { s.State = "installing" })
	if a.hooks.Quit != nil {
		go func() {
			time.Sleep(400 * time.Millisecond)
			a.hooks.Quit()
		}()
	}
	return nil
}

// loop checks shortly after launch and then every few hours.
// Start only runs it for real releases (not dev builds).
func (u *updater) loop(ctx context.Context) {
	t := time.NewTimer(updateFirstCheck)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if u.a.getSettings().AutoUpdate {
			_ = u.check(ctx)
		}
		t.Reset(updateEvery)
	}
}

// installOnExit quietly installs a downloaded update when Phoenix closes,
// so the next launch is already up to date.
func (u *updater) installOnExit() {
	u.mu.Lock()
	path, launched := u.ready, u.launched
	u.mu.Unlock()
	if path == "" || launched || !u.a.getSettings().AutoUpdate {
		return
	}
	if launchInstaller(path, false) == nil {
		u.mu.Lock()
		u.launched = true
		u.mu.Unlock()
	}
}

func (u *updater) check(ctx context.Context) error {
	u.mu.Lock()
	if u.running || u.launched || u.ready != "" {
		u.mu.Unlock()
		return nil
	}
	u.running = true
	u.mu.Unlock()
	defer func() {
		u.mu.Lock()
		u.running = false
		u.mu.Unlock()
	}()

	u.set(func(s *UpdateStatus) { s.State, s.Error = "checking", "" })
	err := u.checkAndDownload(ctx)
	if err != nil {
		if ctx.Err() != nil {
			u.set(func(s *UpdateStatus) { s.State = "idle" })
			return err
		}
		u.set(func(s *UpdateStatus) {
			s.State, s.Error, s.Checked = "error", err.Error(), time.Now().UnixMilli()
		})
	}
	return err
}

type ghAsset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
}

type ghLatest struct {
	TagName string    `json:"tag_name"`
	Body    string    `json:"body"`
	Assets  []ghAsset `json:"assets"`
}

var (
	errNoRelease = errors.New("no releases yet")
	setupAsset   = regexp.MustCompile(`^PhoenixAI-Setup-[0-9][0-9A-Za-z.\-]*\.exe$`)
	sha256Hex    = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)
)

func (u *updater) checkAndDownload(ctx context.Context) error {
	client := u.a.http
	rel, err := fetchLatestRelease(ctx, client, UpdateRepo)
	now := time.Now().UnixMilli()
	if errors.Is(err, errNoRelease) {
		u.set(func(s *UpdateStatus) { s.State, s.Latest, s.Checked = "uptodate", Version, now })
		return nil
	}
	if err != nil {
		return err
	}
	latest := strings.TrimPrefix(rel.TagName, "v")
	if !versionNewer(latest, Version) {
		u.set(func(s *UpdateStatus) { s.State, s.Latest, s.Notes, s.Checked = "uptodate", latest, "", now })
		return nil
	}

	var setup, sidecar *ghAsset
	for i := range rel.Assets {
		if setupAsset.MatchString(rel.Assets[i].Name) {
			setup = &rel.Assets[i]
		}
	}
	if setup == nil {
		return fmt.Errorf("version %s is out, but its installer isn't uploaded yet. Phoenix will try again later", latest)
	}
	for i := range rel.Assets {
		if rel.Assets[i].Name == setup.Name+".sha256" {
			sidecar = &rel.Assets[i]
		}
	}
	want := ""
	if d, ok := strings.CutPrefix(setup.Digest, "sha256:"); ok && sha256Hex.MatchString(d) {
		want = strings.ToLower(d)
	} else if sidecar != nil {
		if want, err = fetchSidecar(ctx, client, sidecar.URL); err != nil {
			return err
		}
	}
	if want == "" {
		return errors.New("this update has no checksum, so Phoenix can't check it and won't install it")
	}

	dest := filepath.Join(u.a.paths.Updates, setup.Name)
	if fileSHA256(dest) == want {
		u.markReady(dest, latest, rel.Body, now)
		return nil
	}

	u.set(func(s *UpdateStatus) {
		s.State, s.Latest, s.Notes, s.Total, s.Received, s.Checked = "downloading", latest, rel.Body, setup.Size, 0, now
	})
	part := dest + ".part"
	h := sha256.New()
	th := throttle{interval: 250 * time.Millisecond}
	err = downloadFile(ctx, client, setup.URL, part, true, func(got, total int64) {
		if th.ok() {
			u.set(func(s *UpdateStatus) {
				s.Received = got
				if total > 0 {
					s.Total = total
				}
			})
		}
	}, h)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("the update download stopped (%v). Phoenix will try again later", err)
	}
	if hex.EncodeToString(h.Sum(nil)) != want {
		os.Remove(part)
		return errors.New("the update download was damaged. Phoenix will try again later")
	}
	if err := os.Rename(part, dest); err != nil {
		return err
	}
	u.markReady(dest, latest, rel.Body, now)
	return nil
}

func (u *updater) markReady(path, latest, notes string, checked int64) {
	// Remove installers from older updates.
	if entries, err := os.ReadDir(u.a.paths.Updates); err == nil {
		for _, e := range entries {
			if p := filepath.Join(u.a.paths.Updates, e.Name()); p != path {
				os.RemoveAll(p)
			}
		}
	}
	u.mu.Lock()
	u.ready = path
	u.mu.Unlock()
	u.set(func(s *UpdateStatus) {
		s.State, s.Latest, s.Notes, s.Checked, s.Error = "ready", latest, notes, checked, ""
		s.Received = s.Total
	})
}

func fetchLatestRelease(ctx context.Context, client *http.Client, repo string) (*ghLatest, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/repos/%s/releases/latest", updateAPIBase, repo), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "PhoenixAI/"+Version)
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("couldn't reach GitHub to check for updates. Check your internet connection")
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case 200:
	case 404:
		return nil, errNoRelease
	case 403, 429:
		return nil, errors.New("GitHub is limiting requests right now. Phoenix will check again later")
	default:
		return nil, fmt.Errorf("GitHub answered with status %d", resp.StatusCode)
	}
	var rel ghLatest
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rel); err != nil {
		return nil, fmt.Errorf("couldn't read the update information: %w", err)
	}
	return &rel, nil
}

func fetchSidecar(ctx context.Context, client *http.Client, url string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "PhoenixAI/"+Version)
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.New("couldn't download the update's checksum")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("couldn't download the update's checksum (status %d)", resp.StatusCode)
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	fields := strings.Fields(string(b))
	if len(fields) == 0 || !sha256Hex.MatchString(fields[0]) {
		return "", errors.New("the update's checksum file is unreadable")
	}
	return strings.ToLower(fields[0]), nil
}

func fileSHA256(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

func isDevVersion(v string) bool { return strings.Contains(v, "-dev") }

// parseVersion reads "1.2.3" (anything after "-" is a pre-release tag).
func parseVersion(v string) (nums [3]int, pre bool, ok bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	base, suffix, _ := strings.Cut(v, "-")
	parts := strings.Split(base, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return nums, false, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nums, false, false
		}
		nums[i] = n
	}
	return nums, suffix != "", true
}

// versionNewer reports whether version a is newer than b.
func versionNewer(a, b string) bool {
	na, preA, okA := parseVersion(a)
	nb, preB, okB := parseVersion(b)
	if !okA {
		return false
	}
	if !okB {
		return true
	}
	for i := 0; i < 3; i++ {
		if na[i] != nb[i] {
			return na[i] > nb[i]
		}
	}
	return preB && !preA
}
