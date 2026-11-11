package backend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ModelStatus is sent as "model:status" events.
type ModelStatus struct {
	State   string `json:"state"` // idle, loading, ready, error
	ModelID string `json:"modelId"`
	Name    string `json:"name"`
	OnGPU   bool   `json:"onGpu"`
	Vision  bool   `json:"vision"` // the loaded model can see images
	Note    string `json:"note"`
	Error   string `json:"error"`
}

type launchConfig struct {
	Exe    string
	Model  string
	MMProj string // image support file, or ""
	Ctx    int
	GPU    bool
}

// runner starts and stops llama-server with one model loaded at a time.
type runner struct {
	a       *App
	startMu sync.Mutex

	mu       sync.Mutex
	cmd      *exec.Cmd
	done     chan struct{}
	base     string
	key      string
	cfg      launchConfig
	id       string
	status   ModelStatus
	logs     *ringLog
	stopping bool
	vision   bool
}

func newRunner(a *App) *runner {
	return &runner{a: a, status: ModelStatus{State: "idle"}}
}

// GetModelStatus says which model is loaded right now.
func (a *App) GetModelStatus() ModelStatus {
	a.runner.mu.Lock()
	defer a.runner.mu.Unlock()
	return a.runner.status
}

// LoadModel starts loading a model in the background so the first answer
// comes faster. Progress arrives as "model:status" events.
func (a *App) LoadModel(id string) error {
	if _, err := a.modelPath(id); err != nil {
		return err
	}
	_ = a.updateSettings(func(s *Settings) { s.LastModel = id })
	go a.runner.ensure(a.ctx, id)
	return nil
}

// UnloadModel frees the memory the model is using.
func (a *App) UnloadModel() {
	a.StopGeneration()
	a.runner.stop()
}

// apiKey is a random password for this run of the engine, so other
// programs on the PC can't use the model behind Phoenix's back.
func (r *runner) apiKey() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.key
}

func (r *runner) loadedID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cmd == nil {
		return ""
	}
	return r.id
}

func (r *runner) setStatus(s ModelStatus) {
	r.mu.Lock()
	r.status = s
	r.mu.Unlock()
	r.a.emit("model:status", s)
}

// ensure makes sure the given model is loaded and returns the server address.
func (r *runner) ensure(ctx context.Context, id string) (string, error) {
	r.startMu.Lock()
	defer r.startMu.Unlock()

	exe := r.a.engine.exePath()
	if exe == "" {
		return "", errors.New("the AI engine isn't set up yet. Open Settings and choose Set up engine")
	}
	path, err := r.a.modelPath(id)
	if err != nil {
		return "", err
	}
	s := r.a.getSettings()
	cfg := launchConfig{Exe: exe, Model: path, MMProj: findVisionFile(path), Ctx: s.ContextSize, GPU: s.UseGPU}
	name := r.a.modelName(id)

	r.mu.Lock()
	if r.cmd != nil && r.cfg == cfg && r.status.State == "ready" {
		base := r.base
		r.mu.Unlock()
		return base, nil
	}
	r.mu.Unlock()

	r.stop()
	r.setStatus(ModelStatus{State: "loading", ModelID: id, Name: name})

	try := cfg
	base, onGPU, note, err := r.start(ctx, try, id)
	if err != nil && try.GPU && ctx.Err() == nil {
		// Try again on the processor. Slower, but it usually works.
		gpuErr := err
		try.GPU = false
		base, onGPU, note, err = r.start(ctx, try, id)
		if err == nil {
			note = "Your graphics card couldn't load this model, so it's running on the processor. Answers will be slower."
		} else {
			try.GPU = cfg.GPU
			err = gpuErr
		}
	}
	if err != nil && try.MMProj != "" && ctx.Err() == nil {
		// Image support uses extra memory and can be what tips it over.
		// Chatting without pictures beats not chatting at all.
		firstErr := err
		try.MMProj = ""
		base, onGPU, note, err = r.start(ctx, try, id)
		if err == nil {
			note = strings.TrimSpace("This model's image support couldn't load, so it can only read text right now. " + note)
		} else {
			err = firstErr
		}
	}
	if err != nil {
		r.setStatus(ModelStatus{State: "error", ModelID: id, Name: name, Error: err.Error()})
		return "", err
	}
	r.mu.Lock()
	r.cfg = cfg // remember what was asked for, so a fallback isn't retried every message
	r.vision = try.MMProj != ""
	r.mu.Unlock()
	r.setStatus(ModelStatus{State: "ready", ModelID: id, Name: name, OnGPU: onGPU, Vision: try.MMProj != "", Note: note})
	return base, nil
}

// hasVision says whether the loaded model can see images right now.
func (r *runner) hasVision() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cmd != nil && r.vision
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

var offloadLine = regexp.MustCompile(`offloaded (\d+)/(\d+) layers to GPU`)

func (r *runner) start(ctx context.Context, cfg launchConfig, id string) (base string, onGPU bool, note string, err error) {
	port, err := freePort()
	if err != nil {
		return "", false, "", err
	}
	args := []string{
		"-m", cfg.Model,
		"--host", "127.0.0.1",
		"--port", strconv.Itoa(port),
		"-c", strconv.Itoa(cfg.Ctx),
		"--no-webui",
	}
	if cfg.MMProj != "" {
		args = append(args, "--mmproj", cfg.MMProj)
	}
	if !cfg.GPU {
		args = append(args, "-ngl", "0")
	}
	keyBytes := make([]byte, 24)
	_, _ = rand.Read(keyBytes)
	key := hex.EncodeToString(keyBytes)

	logs := newRingLog(120)
	cmd := exec.Command(cfg.Exe, args...)
	cmd.Dir = filepath.Dir(cfg.Exe)
	// Passed through the environment so it doesn't show in the process list.
	cmd.Env = append(os.Environ(), "LLAMA_API_KEY="+key)
	cmd.Stdout = logs
	cmd.Stderr = logs
	prepareCmd(cmd)
	if err := cmd.Start(); err != nil {
		return "", false, "", fmt.Errorf("couldn't start the AI engine: %w", err)
	}
	bindToApp(cmd.Process)

	done := make(chan struct{})
	base = fmt.Sprintf("http://127.0.0.1:%d", port)
	r.mu.Lock()
	r.cmd, r.done, r.base, r.key, r.id, r.logs, r.stopping = cmd, done, base, key, id, logs, false
	r.mu.Unlock()

	go func() {
		cmd.Wait()
		close(done)
		r.mu.Lock()
		if r.cmd != cmd {
			r.mu.Unlock()
			return
		}
		r.cmd = nil
		unexpected := !r.stopping && r.status.State == "ready"
		st := r.status
		r.mu.Unlock()
		if unexpected {
			st.State = "error"
			st.Error = "The model stopped unexpectedly. " + friendlyEngineError(logs.String())
			r.setStatus(st)
		}
	}()

	if err := waitHealthy(ctx, base, done, 20*time.Minute); err != nil {
		r.kill(cmd, done)
		if errors.Is(err, errExited) {
			return "", false, "", errors.New(friendlyEngineError(logs.String()))
		}
		return "", false, "", err
	}

	if m := offloadLine.FindStringSubmatch(logs.String()); m != nil {
		got, _ := strconv.Atoi(m[1])
		all, _ := strconv.Atoi(m[2])
		onGPU = got > 0
		if got > 0 && got < all {
			note = fmt.Sprintf("This model is a little big for your graphics card, so %d of %d parts run on the processor. Answers will be slower.", all-got, all)
		}
	}
	return base, onGPU, note, nil
}

var errExited = errors.New("engine exited")

func waitHealthy(ctx context.Context, base string, done <-chan struct{}, limit time.Duration) error {
	client := &http.Client{Timeout: 3 * time.Second}
	deadline := time.Now().Add(limit)
	for {
		select {
		case <-done:
			return errExited
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if time.Now().After(deadline) {
			return errors.New("the model took too long to load")
		}
		resp, err := client.Get(base + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		select {
		case <-done:
			return errExited
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
}

func (r *runner) kill(cmd *exec.Cmd, done chan struct{}) {
	r.mu.Lock()
	if r.cmd == cmd {
		r.stopping = true
	}
	r.mu.Unlock()
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
	}
	r.mu.Lock()
	if r.cmd == cmd {
		r.cmd = nil
	}
	r.mu.Unlock()
}

// stop unloads the current model.
func (r *runner) stop() {
	r.mu.Lock()
	cmd, done := r.cmd, r.done
	r.mu.Unlock()
	if cmd == nil {
		return
	}
	r.kill(cmd, done)
	r.mu.Lock()
	r.id = ""
	r.mu.Unlock()
	r.setStatus(ModelStatus{State: "idle"})
}

// friendlyEngineError turns the engine's log into something a person can act on.
func friendlyEngineError(logText string) string {
	l := strings.ToLower(logText)
	switch {
	case strings.Contains(l, "out of memory") || strings.Contains(l, "failed to allocate") || strings.Contains(l, "unable to allocate") || strings.Contains(l, "erroroutofdevicememory"):
		return "There isn't enough memory to load this model. Try a smaller model, or close other programs and try again."
	case strings.Contains(l, "unknown model architecture") || strings.Contains(l, "unsupported model architecture"):
		return "This model is newer than your AI engine. Update the engine in Settings and try again."
	case strings.Contains(l, "failed to load model") || strings.Contains(l, "error loading model") || strings.Contains(l, "invalid magic"):
		return "The model file couldn't be loaded. It may be damaged. Try deleting it and downloading it again."
	case strings.Contains(l, "couldn't bind") || strings.Contains(l, "address already in use"):
		return "The engine couldn't open its connection. Try again."
	}
	lines := strings.Split(strings.TrimSpace(logText), "\n")
	if len(lines) > 0 && lines[len(lines)-1] != "" {
		return "The engine stopped: " + strings.TrimSpace(lines[len(lines)-1])
	}
	return "The engine stopped without saying why."
}

// ringLog keeps the last lines the engine printed.
type ringLog struct {
	mu    sync.Mutex
	lines []string
	max   int
	part  string
	keep  []string // important lines kept even after they scroll away
}

func newRingLog(max int) *ringLog { return &ringLog{max: max} }

func (l *ringLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	text := l.part + string(p)
	parts := strings.Split(text, "\n")
	l.part = parts[len(parts)-1]
	for _, line := range parts[:len(parts)-1] {
		line = strings.TrimRight(line, "\r")
		if offloadLine.MatchString(line) {
			l.keep = append(l.keep, line)
		}
		l.lines = append(l.lines, line)
		if len(l.lines) > l.max {
			l.lines = l.lines[len(l.lines)-l.max:]
		}
	}
	return len(p), nil
}

func (l *ringLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	all := append(append([]string{}, l.keep...), l.lines...)
	if l.part != "" {
		all = append(all, l.part)
	}
	return strings.Join(all, "\n")
}
