package backend

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Pictures and videos are made by stable-diffusion.cpp's sd-server, which
// Phoenix starts with the chosen model loaded, like llama-server for chat.
// Only one of the two runs at a time, since both want the graphics card.

// Media is a picture or video Phoenix made, saved in the chat's folder.
type Media struct {
	ID      string  `json:"id"`   // file name in chats/<chat>/: <hex>.png or <hex>.webm
	Kind    string  `json:"kind"` // image or video
	Width   int     `json:"width"`
	Height  int     `json:"height"`
	Frames  int     `json:"frames,omitempty"`
	FPS     int     `json:"fps,omitempty"`
	Seed    int64   `json:"seed"`
	Shape   string  `json:"shape"`
	Seconds float64 `json:"seconds"` // how long it took
}

// MediaOptions are the choices next to the message box.
type MediaOptions struct {
	Shape  string `json:"shape"`  // square, wide or tall
	Length int    `json:"length"` // video length in seconds
}

// MediaProgress is sent as "media:progress" events while something is made.
type MediaProgress struct {
	ChatID    string `json:"chatId"`
	Stage     string `json:"stage"` // loading, preparing, sampling, decoding, saving
	Label     string `json:"label"`
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
	ElapsedMs int64  `json:"elapsedMs"`
	Preview   string `json:"preview,omitempty"` // data URL of the picture so far
}

// Wan's recommended negative prompt (it was trained with Chinese ones):
// oversaturated, static, blurry, subtitles, worst quality, deformed hands
// and faces, extra limbs, cluttered background, walking backwards...
const wanNegative = "色调艳丽，过曝，静态，细节模糊不清，字幕，风格，作品，画作，画面，静止，整体发灰，最差质量，低质量，JPEG压缩残留，丑陋的，残缺的，多余的手指，画得不好的手部，画得不好的脸部，畸形的，毁容的，形态畸形的肢体，手指融合，静止不动的画面，杂乱的背景，三条腿，背景人很多，倒着走"

const sdxlNegative = "blurry, low quality, worst quality, jpeg artifacts, watermark, text, signature, deformed, disfigured, extra fingers, bad hands, bad anatomy"

const videoFPS = 24 // Wan 2.2 5B makes 24 frames a second

// mediaSize picks the picture size for a shape. Sizes are multiples of 64
// (pictures) or 32 (videos), close to what each model was trained on.
func mediaSize(kind, shape string) (w, h int) {
	if kind == KindVideo {
		// Smaller than Wan's 1280×704 so a video takes minutes, not an hour,
		// on a typical graphics card.
		switch shape {
		case "tall":
			return 480, 832
		case "square":
			return 640, 640
		}
		return 832, 480
	}
	switch shape {
	case "wide":
		return 1216, 832
	case "tall":
		return 832, 1216
	}
	return 1024, 1024
}

func videoFrames(seconds int) int {
	if seconds < 1 {
		seconds = 2
	}
	if seconds > 5 {
		seconds = 5
	}
	n := seconds * videoFPS
	return n - n%4 + 1 // Wan wants 4n+1 frames
}

// ---------- the engine ----------

type mediaRunner struct {
	a       *App
	startMu sync.Mutex

	mu       sync.Mutex
	cmd      *exec.Cmd
	done     chan struct{}
	base     string
	key      string // model ID + settings it was started with
	cpu      bool
	logs     *ringLog
	stopping bool
}

func newMediaRunner(a *App) *mediaRunner { return &mediaRunner{a: a} }

func (m *mediaRunner) running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cmd != nil
}

// stop unloads the picture/video model.
func (m *mediaRunner) stop() {
	m.mu.Lock()
	cmd, done := m.cmd, m.done
	if cmd != nil {
		m.stopping = true
	}
	m.mu.Unlock()
	if cmd == nil {
		return
	}
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
	}
	m.mu.Lock()
	if m.cmd == cmd {
		m.cmd = nil
	}
	m.mu.Unlock()
}

// mediaArgs builds sd-server's command line for a model.
func (a *App) mediaArgs(model CatalogModel, path string, port int, cpu bool) ([]string, error) {
	part := func(role string) (string, error) {
		for _, p := range model.Parts {
			if p.Role == role {
				fp, ok := safeJoin(a.paths.Models, modelID(p.Repo, p.File))
				if !ok || !fileExists(fp) {
					return "", fmt.Errorf("%s is still missing. Open the Models page to finish downloading it", p.Name)
				}
				return fp, nil
			}
		}
		return "", nil
	}
	empty := filepath.Join(a.paths.Tools, "sd-models")
	_ = os.MkdirAll(empty, 0o755)
	args := []string{
		"--listen-ip", "127.0.0.1",
		"--listen-port", strconv.Itoa(port),
		"--model-dir", empty, // so it doesn't scan for other models
	}
	vram := int64(0)
	if sys := a.sysSnapshot(); sys != nil {
		vram = sys.BestVRAM
	}
	switch model.kindOf() {
	case KindImage:
		vae, err := part("vae")
		if err != nil {
			return nil, err
		}
		args = append(args, "-m", path)
		if vae != "" {
			args = append(args, "--vae", vae)
		}
		args = append(args, "--cfg-scale", "6", "--steps", "25", "--sampling-method", "euler_a")
		if vram < 12*gb {
			args = append(args, "--vae-tiling")
		}
	case KindVideo:
		te, err := part("text_encoder")
		if err != nil {
			return nil, err
		}
		vae, err := part("vae")
		if err != nil {
			return nil, err
		}
		args = append(args, "--diffusion-model", path, "--t5xxl", te, "--vae", vae,
			"--cfg-scale", "6", "--steps", "20", "--sampling-method", "euler", "--flow-shift", "3",
			// The video decoder needs lots of memory; these keep it within
			// an 8 GB graphics card.
			"--vae-tiling", "--offload-to-cpu")
	default:
		return nil, errors.New("that model doesn't make pictures or videos")
	}
	if cpu {
		args = append(args, "--backend", "cpu")
	}
	return args, nil
}

// ensure starts sd-server with the model loaded and returns its address.
func (m *mediaRunner) ensure(ctx context.Context, model CatalogModel, path string, status func(string)) (string, string, error) {
	m.startMu.Lock()
	defer m.startMu.Unlock()
	a := m.a
	exe := a.toolPath(toolPictures)
	if exe == "" {
		_ = a.ensureTool(toolPictures)
		return "", "", errors.New("Phoenix is still downloading its picture and video engine. Try again when it's done (see the Models page)")
	}
	cpu := !a.getSettings().UseGPU
	key := model.ID + "|" + strconv.FormatBool(cpu)

	m.mu.Lock()
	if m.cmd != nil && m.key == key {
		base := m.base
		m.mu.Unlock()
		return base, "", nil
	}
	m.mu.Unlock()
	m.stop()
	// Free the graphics card for it.
	a.runner.stop()

	status("Loading " + model.Name + ". The first time takes a minute or two.")
	base, err := m.start(ctx, exe, model, path, cpu, key)
	note := ""
	if err != nil && !cpu && ctx.Err() == nil && !isSettingsError(m.lastLog()) {
		gpuErr := err
		status("Your graphics card couldn't load it. Trying the processor instead…")
		base, err = m.start(ctx, exe, model, path, true, key)
		if err == nil {
			note = "Your graphics card couldn't load " + model.Name + ", so it's running on the processor. This will be very slow."
		} else {
			err = gpuErr
		}
	}
	return base, note, err
}

func (m *mediaRunner) start(ctx context.Context, exe string, model CatalogModel, path string, cpu bool, key string) (string, error) {
	port, err := freePort()
	if err != nil {
		return "", err
	}
	args, err := m.a.mediaArgs(model, path, port, cpu)
	if err != nil {
		return "", err
	}
	logs := newRingLog(200)
	cmd := exec.Command(exe, args...)
	cmd.Dir = filepath.Dir(exe)
	cmd.Stdout, cmd.Stderr = logs, logs
	prepareCmd(cmd)
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("couldn't start the picture engine: %w", err)
	}
	bindToApp(cmd.Process)
	done := make(chan struct{})
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	m.mu.Lock()
	m.cmd, m.done, m.base, m.key, m.cpu, m.logs, m.stopping = cmd, done, base, key, cpu, logs, false
	m.mu.Unlock()
	go func() {
		cmd.Wait()
		close(done)
		m.mu.Lock()
		if m.cmd == cmd {
			m.cmd = nil
		}
		m.mu.Unlock()
	}()

	// sd-server loads the model before it starts listening.
	client := &http.Client{Timeout: 3 * time.Second}
	deadline := time.Now().Add(30 * time.Minute)
	for {
		select {
		case <-done:
			return "", errors.New(friendlyEngineError(logs.String()))
		case <-ctx.Done():
			m.stop()
			return "", ctx.Err()
		case <-time.After(400 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			m.stop()
			return "", errors.New("the model took too long to load")
		}
		resp, err := client.Get(base + "/sdcpp/v1/models/status")
		if err != nil {
			continue
		}
		var st struct {
			State string `json:"state"`
			Error string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&st)
		resp.Body.Close()
		switch st.State {
		case "ready":
			return base, nil
		case "error":
			m.stop()
			return "", errors.New("the model couldn't be loaded: " + st.Error)
		}
	}
}

func (m *mediaRunner) onCPU() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cpu
}

func (m *mediaRunner) lastLog() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.logs == nil {
		return ""
	}
	return m.logs.String()
}

// ---------- making pictures and videos ----------

// MakeMedia makes a picture or video from a description, with an image or
// video model. attachmentIDs may hold one picture to start from. Progress
// arrives as "media:progress" events and it finishes with "chat:done".
// Returns the chat ID.
func (a *App) MakeMedia(chatID, prompt, modelID string, opts MediaOptions, attachmentIDs []string) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", errors.New("describe what you'd like to see first")
	}
	model, path, err := a.mediaModel(modelID)
	if err != nil {
		return "", err
	}
	if a.generating() {
		return "", errors.New("wait for the current answer to finish, or stop it first")
	}
	if len(attachmentIDs) > 1 {
		return "", errors.New("add just one picture to start from")
	}
	var files []Attachment
	for _, id := range attachmentIDs {
		att, ok := a.pendingAttachment(id)
		if !ok {
			return "", errors.New("the picture is no longer there. Remove it and add it again")
		}
		if att.Kind != attachmentKindImage {
			return "", fmt.Errorf("%s can only start from a picture. Remove %s", model.Name, att.Name)
		}
		files = append(files, att)
	}
	_ = path

	var ch *Chat
	now := time.Now().UnixMilli()
	if chatID == "" {
		if len(a.ListChats()) >= MaxChats {
			return "", fmt.Errorf("you already have %d chats, the most Phoenix keeps. Delete one to start a new chat", MaxChats)
		}
		ch = &Chat{ID: newChatID(), Title: titleFrom(prompt), Created: now}
	} else if ch, err = a.chats.load(chatID); err != nil {
		return "", err
	}
	atts, err := a.takePending(ch.ID, attachmentIDs)
	if err != nil {
		return "", err
	}
	ch.Messages = append(ch.Messages, Message{Role: "user", Content: prompt, Attachments: atts, Time: now})
	ch.Updated = now
	if err := a.chats.save(ch); err != nil {
		return "", fmt.Errorf("couldn't save the chat: %w", err)
	}
	_ = a.updateSettings(func(s *Settings) { s.LastModel = modelID })
	a.emit("chats:changed", nil)
	a.startMedia(ch, model, modelID, opts, -1)
	return ch.ID, nil
}

// mediaModel finds a downloaded picture or video model by ID.
func (a *App) mediaModel(modelID string) (CatalogModel, string, error) {
	path, err := a.modelPath(modelID)
	if err != nil {
		return CatalogModel{}, "", err
	}
	parts := strings.SplitN(modelID, "/", 2)
	model, ok := catalogByFile(repoFromDir(parts[0]), parts[1])
	if !ok || (model.kindOf() != KindImage && model.kindOf() != KindVideo) {
		return CatalogModel{}, "", errors.New("that model doesn't make pictures or videos")
	}
	if miss := a.missingFor(model); miss != "" {
		if key := toolForKind(model.kindOf()); key != "" {
			_ = a.ensureTool(key)
		}
		a.startParts(model)
		return CatalogModel{}, "", fmt.Errorf("%s still needs %s. It's downloading now; try again when it's done", model.Name, strings.ToLower(miss[:1])+miss[1:])
	}
	return model, path, nil
}

func (a *App) startMedia(ch *Chat, model CatalogModel, modelID string, opts MediaOptions, seed int64) {
	a.startTask(ch, modelID, func(ctx context.Context) Message {
		return a.makeMedia(ctx, ch, model, modelID, opts, seed)
	})
}

func (a *App) makeMedia(ctx context.Context, ch *Chat, model CatalogModel, modelID string, opts MediaOptions, seed int64) Message {
	msg := Message{Role: "assistant"}
	started := time.Now()
	progress := func(p MediaProgress) {
		p.ChatID = ch.ID
		p.ElapsedMs = time.Since(started).Milliseconds()
		a.emit("media:progress", p)
	}
	path, err := a.modelPath(modelID)
	if err != nil {
		msg.Error = err.Error()
		return msg
	}
	base, note, err := a.media.ensure(ctx, model, path, func(s string) { progress(MediaProgress{Stage: "loading", Label: s}) })
	if err != nil {
		if ctx.Err() != nil {
			msg.Stopped = true
			return msg
		}
		msg.Error = err.Error()
		return msg
	}
	if note != "" {
		progress(MediaProgress{Stage: "loading", Label: note})
	}
	a.runner.setStatus(ModelStatus{State: "ready", ModelID: modelID, Name: model.Name, OnGPU: !a.media.onCPU(), Note: note})

	last := ch.Messages[len(ch.Messages)-1]
	kind := model.kindOf()
	shape := opts.Shape
	if shape != "wide" && shape != "tall" && shape != "square" {
		shape = "square"
		if kind == KindVideo {
			shape = "wide"
		}
	}
	var start string // data URL of the picture to start from
	for _, att := range last.Attachments {
		if att.Kind == attachmentKindImage {
			if u, ok := a.readAttachmentImage(ch.ID, att); ok {
				start = u
				// Match the picture's shape.
				switch r := float64(att.Width) / float64(max(att.Height, 1)); {
				case r > 1.2:
					shape = "wide"
				case r < 0.83:
					shape = "tall"
				default:
					shape = "square"
				}
			}
		}
	}
	w, h := mediaSize(kind, shape)
	if seed < 0 {
		seed = rand.Int63n(1 << 31)
	}
	body := map[string]any{
		"prompt": last.Content,
		"width":  w,
		"height": h,
		"seed":   seed,
	}
	endpoint := "/sdcpp/v1/img_gen"
	media := Media{Kind: KindImage, Width: w, Height: h, Seed: seed, Shape: shape}
	if kind == KindVideo {
		endpoint = "/sdcpp/v1/vid_gen"
		frames := videoFrames(opts.Length)
		body["negative_prompt"] = wanNegative
		body["video_frames"] = frames
		body["fps"] = videoFPS
		body["output_format"] = "webm"
		if start != "" {
			body["init_image"] = start
		}
		media.Kind, media.Frames, media.FPS = KindVideo, frames, videoFPS
	} else {
		body["negative_prompt"] = sdxlNegative
		body["batch_count"] = 1
		body["output_format"] = "png"
		body["preview"] = "proj"
		body["preview_interval"] = 2
		if start != "" {
			body["init_image"] = start
			body["strength"] = 0.65 // keep the picture's layout, change the details
		}
	}

	job, err := sdSubmit(ctx, base+endpoint, body)
	if err != nil {
		msg.Error = a.mediaError(err)
		return msg
	}
	result, err := a.sdWait(ctx, base, job, kind, progress)
	if err != nil {
		if ctx.Err() != nil || errors.Is(err, errCanceled) {
			msg.Stopped = true
			return msg
		}
		msg.Error = a.mediaError(err)
		return msg
	}
	progress(MediaProgress{Stage: "saving", Label: "Saving"})
	ext := ".png"
	if media.Kind == KindVideo {
		ext = ".webm"
	}
	media.ID = newAttachmentID(ext)
	dir := filepath.Join(a.paths.Chats, ch.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		msg.Error = err.Error()
		return msg
	}
	if err := os.WriteFile(filepath.Join(dir, media.ID), result, 0o644); err != nil {
		msg.Error = "Couldn't save it: " + err.Error()
		return msg
	}
	if media.Kind == KindImage {
		if cw, chh, ok := pngSize(result); ok {
			media.Width, media.Height = cw, chh
		}
	}
	media.Seconds = time.Since(started).Seconds()
	msg.Media = []Media{media}
	return msg
}

var errCanceled = errors.New("canceled")

type sdJob struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func sdSubmit(ctx context.Context, url string, body map[string]any) (string, error) {
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 202 && resp.StatusCode != 200 {
		var e struct {
			Error any `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		return "", fmt.Errorf("the engine refused the request (%d): %v", resp.StatusCode, e.Error)
	}
	var j sdJob
	if err := json.Unmarshal(raw, &j); err != nil || j.ID == "" {
		return "", errors.New("the engine sent an answer Phoenix didn't understand")
	}
	return j.ID, nil
}

// sdWait polls a job until it's done and returns the finished file.
func (a *App) sdWait(ctx context.Context, base, id, kind string, progress func(MediaProgress)) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	lastPreview := -1
	canceled := false
	for {
		select {
		case <-ctx.Done():
			if !canceled {
				canceled = true
				// Ask the engine to stop, then keep polling until it has.
				cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				req, _ := http.NewRequestWithContext(cctx, "POST", base+"/sdcpp/v1/jobs/"+id+"/cancel", nil)
				if resp, err := http.DefaultClient.Do(req); err == nil {
					resp.Body.Close()
				}
				cancel()
			}
		case <-time.After(500 * time.Millisecond):
		}
		if !a.media.running() {
			return nil, errors.New(friendlyEngineError(a.media.lastLog()))
		}
		resp, err := client.Get(base + "/sdcpp/v1/jobs/" + id)
		if err != nil {
			if canceled {
				return nil, errCanceled
			}
			continue
		}
		var j struct {
			Status   string `json:"status"`
			Progress struct {
				Stage     string `json:"stage"`
				Completed *int   `json:"completed"`
				Total     *int   `json:"total"`
			} `json:"progress"`
			Preview *struct {
				Step int    `json:"step"`
				Pass int    `json:"pass"`
				B64  string `json:"b64_json"`
			} `json:"preview"`
			Result *struct {
				B64    string `json:"b64_json"`
				Images []struct {
					B64 string `json:"b64_json"`
				} `json:"images"`
			} `json:"result"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 512<<20)).Decode(&j)
		resp.Body.Close()
		if err != nil {
			continue
		}
		switch j.Status {
		case "completed":
			b64 := ""
			if j.Result != nil {
				b64 = j.Result.B64
				if b64 == "" && len(j.Result.Images) > 0 {
					b64 = j.Result.Images[0].B64
				}
			}
			out, err := base64.StdEncoding.DecodeString(b64)
			if err != nil || len(out) == 0 {
				return nil, errors.New("the engine finished but sent back nothing")
			}
			return out, nil
		case "failed":
			msg := "something went wrong"
			if j.Error != nil && j.Error.Message != "" {
				msg = j.Error.Message
			}
			return nil, errors.New(msg)
		case "cancelled":
			return nil, errCanceled
		}
		p := MediaProgress{Stage: j.Progress.Stage}
		if j.Progress.Completed != nil {
			p.Completed = *j.Progress.Completed
		}
		if j.Progress.Total != nil {
			p.Total = *j.Progress.Total
		}
		p.Label = stageLabel(j.Progress.Stage, kind, j.Status)
		if j.Preview != nil && j.Preview.B64 != "" && j.Preview.Pass*10000+j.Preview.Step != lastPreview {
			lastPreview = j.Preview.Pass*10000 + j.Preview.Step
			p.Preview = "data:image/png;base64," + j.Preview.B64
		}
		progress(p)
	}
}

func stageLabel(stage, kind, status string) string {
	thing := "picture"
	if kind == KindVideo {
		thing = "video"
	}
	switch stage {
	case "queued", "waiting":
		return "Waiting to start"
	case "preparing", "conditioning":
		return "Reading your description"
	case "sampling":
		return "Making your " + thing
	case "vae_encode":
		return "Looking at your picture"
	case "vae_decode":
		return "Finishing the " + thing
	case "encoding":
		return "Saving the " + thing
	}
	if status == "queued" {
		return "Waiting to start"
	}
	return "Working"
}

func (a *App) mediaError(err error) string {
	s := err.Error()
	l := strings.ToLower(s + " " + a.media.lastLog())
	switch {
	case strings.Contains(l, "out of memory") || strings.Contains(l, "erroroutofdevicememory") || strings.Contains(l, "failed to allocate"):
		return "There wasn't enough memory to finish. Try the square shape or a shorter video, or close other programs and try again."
	}
	return "Couldn't make it: " + s
}

// pngSize reads a PNG's width and height from its header.
func pngSize(b []byte) (int, int, bool) {
	if len(b) < 24 || string(b[1:4]) != "PNG" {
		return 0, 0, false
	}
	w := int(b[16])<<24 | int(b[17])<<16 | int(b[18])<<8 | int(b[19])
	h := int(b[20])<<24 | int(b[21])<<16 | int(b[22])<<8 | int(b[23])
	return w, h, true
}

// SaveMedia asks where to save a picture or video and copies it there.
func (a *App) SaveMedia(chatID, mediaID string) (string, error) {
	if !chatIDPattern.MatchString(chatID) || !attIDPattern.MatchString(mediaID) {
		return "", errors.New("that file isn't there anymore")
	}
	src := filepath.Join(a.paths.Chats, chatID, mediaID)
	if !fileExists(src) {
		return "", errors.New("that file isn't there anymore")
	}
	if a.hooks.SaveFile == nil {
		return "", errors.New("saving isn't available here")
	}
	name := "Phoenix " + time.Now().Format("2006-01-02 15.04.05") + filepath.Ext(mediaID)
	dest, err := a.hooks.SaveFile(name)
	if err != nil || dest == "" {
		return "", err
	}
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	if err := writeFileFrom(dest, in, 0o644); err != nil {
		return "", fmt.Errorf("couldn't save it there: %w", err)
	}
	return dest, nil
}
