package backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Besides llama.cpp (engine.go), Phoenix uses three more open-source
// engines, each downloaded the first time a model needs it:
//
//   - stable-diffusion.cpp makes pictures and videos.
//   - NVIDIA's NeMo-Speech.cpp turns speech into text and tells speakers apart.
//   - sherpa-onnx speaks with the Kokoro voice.
//
// Their versions are pinned and checked against a SHA-256, because Phoenix
// talks to them through their command-line options and APIs, which can
// change between versions.

type toolAsset struct {
	URL    string
	Size   int64
	SHA256 string
}

type toolSpec struct {
	Key     string // sd, speech, voice
	Name    string // shown in the downloads list
	Version string
	Exe     string // the program to run, without .exe
	Env     string // a path in this variable is used instead (for testing)
	Assets  map[string]toolAsset
}

const (
	toolPictures = "sd"
	toolSpeech   = "speech"
	toolVoice    = "voice"
)

// toolsMu guards tools (tests swap in local download addresses).
var toolsMu sync.RWMutex

func mustTool(key string) toolSpec {
	t, _ := getTool(key)
	return t
}

func getTool(key string) (toolSpec, bool) {
	toolsMu.RLock()
	defer toolsMu.RUnlock()
	t, ok := tools[key]
	return t, ok
}

var tools = map[string]toolSpec{
	toolPictures: {
		Key: toolPictures, Name: "Picture and video engine", Version: "master-920-2f88688",
		Exe: "sd-server", Env: "PHOENIX_SD_SERVER",
		Assets: map[string]toolAsset{
			// Vulkan runs on NVIDIA, AMD and Intel graphics cards, and falls back to the processor.
			"windows/amd64": {"https://github.com/leejet/stable-diffusion.cpp/releases/download/master-920-2f88688/sd-master-2f88688-bin-win-vulkan-x64.zip", 31998883, "63e84439c20dde75487a933066318ae01353e9e80ee70e031acad48e857e1cb9"},
			"linux/amd64":   {"https://github.com/leejet/stable-diffusion.cpp/releases/download/master-920-2f88688/sd-master-2f88688-bin-Linux-Ubuntu-24.04-x86_64.zip", 25529878, "7367e4034a77a425aaa7d0795e02b6007329a58a893541c9fd6a4c79c196796b"},
		},
	},
	toolSpeech: {
		Key: toolSpeech, Name: "Speech engine", Version: "v0.3.0",
		Exe: "nemo-speech", Env: "PHOENIX_NEMO_SPEECH",
		Assets: map[string]toolAsset{
			// The processor build: speech models are small and quick on any PC.
			"windows/amd64": {"https://github.com/NVIDIA/NeMo-Speech.cpp/releases/download/v0.3.0/nemo-speech-0.3.0-windows-x86_64-cpu.zip", 8694509, "4ed9740e361e4653f56c3dce7d1e81883a1c8b831c1bd7d01567fff1130b5b05"},
			"linux/amd64":   {"https://github.com/NVIDIA/NeMo-Speech.cpp/releases/download/v0.3.0/nemo-speech-0.3.0-linux-x86_64-cpu.tar.gz", 9605101, "255f12b722ab15b2a38a51cf125d56e3fa97058b528f7e895b39d6a473deb06e"},
		},
	},
	toolVoice: {
		Key: toolVoice, Name: "Voice engine", Version: "v2.0.1",
		Exe: "sherpa-onnx-offline-tts", Env: "PHOENIX_SHERPA_TTS",
		Assets: map[string]toolAsset{
			"windows/amd64": {"https://github.com/k2-fsa/sherpa-onnx/releases/download/v2.0.1/sherpa-onnx-v2.0.1-win-x64-shared-MT-Release.tar.bz2", 26664192, "dc703575a68594435f53dec93df775ff4913af851a196c0537163f21d8d2364e"},
			"linux/amd64":   {"https://github.com/k2-fsa/sherpa-onnx/releases/download/v2.0.1/sherpa-onnx-v2.0.1-linux-x64-shared.tar.bz2", 28003506, "2e823376987c3b6960d50726b6560155e2ad92da0b8b3886325773225ec6bdec"},
		},
	},
}

// toolForKind says which engine runs a kind of model ("" for chat, which
// uses llama.cpp).
func toolForKind(kind string) string {
	switch kind {
	case KindImage, KindVideo:
		return toolPictures
	case KindListen, KindSpeakers:
		return toolSpeech
	case KindVoice:
		return toolVoice
	}
	return ""
}

func toolJobID(key string) string { return "tool:" + key }

func (t toolSpec) dir(a *App) string { return filepath.Join(a.paths.Tools, t.Key+"-"+t.Version) }

func (t toolSpec) exeName() string {
	if runtime.GOOS == "windows" {
		return t.Exe + ".exe"
	}
	return t.Exe
}

func (t toolSpec) asset() (toolAsset, bool) {
	as, ok := t.Assets[runtime.GOOS+"/"+runtime.GOARCH]
	return as, ok
}

// toolPath returns the installed program for an engine, or "".
func (a *App) toolPath(key string) string {
	t, ok := getTool(key)
	if !ok {
		return ""
	}
	if p := os.Getenv(t.Env); p != "" {
		return p
	}
	return findFile(t.dir(a), t.exeName())
}

// ensureTool starts downloading an engine if it isn't installed. It
// returns at once; the download shows in the downloads list.
func (a *App) ensureTool(key string) error {
	if a.toolPath(key) != "" {
		return nil
	}
	t, _ := getTool(key)
	if _, ok := t.asset(); !ok {
		return fmt.Errorf("the %s isn't available for this kind of PC yet", strings.ToLower(t.Name))
	}
	return a.dl.startTool(t)
}

// startTool downloads and unpacks an engine as a download job.
func (d *downloadManager) startTool(t toolSpec) error {
	id := toolJobID(t.Key)
	as, _ := t.asset()
	d.mu.Lock()
	if _, busy := d.active[id]; busy {
		d.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancelCause(d.a.ctx)
	j := &dlJob{cancel: cancel, state: DownloadState{ID: id, Name: t.Name, Status: "downloading", Total: as.Size, Tool: true}}
	d.active[id] = j
	d.mu.Unlock()
	d.a.emit("download:progress", j.state)

	go func() {
		err := d.installTool(ctx, j, t, as)
		d.mu.Lock()
		delete(d.active, id)
		st := j.state
		d.mu.Unlock()
		stopped := ctx.Err() != nil
		cancel(nil)
		switch {
		case err == nil:
			st.Status, st.Received = "done", st.Total
			d.a.emit("download:progress", st)
			d.a.emit("models:changed", nil)
		case stopped:
			st.Status = "canceled"
			d.a.emit("download:progress", st)
		default:
			st.Status, st.Error = "error", err.Error()
			d.a.emit("download:progress", st)
		}
	}()
	return nil
}

func (d *downloadManager) installTool(ctx context.Context, j *dlJob, t toolSpec, as toolAsset) error {
	a := d.a
	archive := filepath.Join(a.paths.Tools, t.Key+"-download.part")
	defer os.Remove(archive)
	h := sha256.New()
	th := throttle{interval: 250 * time.Millisecond}
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		h.Reset()
		os.Remove(archive)
		err = downloadFile(ctx, a.http, as.URL, archive, false, func(got, total int64) {
			if th.ok() {
				d.update(j, func(s *DownloadState) { s.Received = got })
			}
		}, h)
		if err == nil || ctx.Err() != nil || isPermanent(err) {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("the %s download failed: %w", strings.ToLower(t.Name), err)
	}
	d.update(j, func(s *DownloadState) { s.Status = "verifying"; s.Received = s.Total })
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, as.SHA256) {
		return errors.New("the engine download was damaged (checksum didn't match). Please try again")
	}
	dest := t.dir(a)
	tmp := dest + ".tmp"
	os.RemoveAll(tmp)
	if err := extractArchive(archive, tmp); err != nil {
		os.RemoveAll(tmp)
		return fmt.Errorf("couldn't unpack the %s: %w", strings.ToLower(t.Name), err)
	}
	if findFile(tmp, t.exeName()) == "" {
		os.RemoveAll(tmp)
		return fmt.Errorf("the %s download didn't contain %s", strings.ToLower(t.Name), t.exeName())
	}
	os.RemoveAll(dest)
	if err := os.Rename(tmp, dest); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	// Remove older versions of this engine.
	if entries, err := os.ReadDir(a.paths.Tools); err == nil {
		for _, en := range entries {
			if en.IsDir() && strings.HasPrefix(en.Name(), t.Key+"-") && en.Name() != filepath.Base(dest) {
				os.RemoveAll(filepath.Join(a.paths.Tools, en.Name()))
			}
		}
	}
	return nil
}
