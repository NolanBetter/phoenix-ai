package backend

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Speech recognition and speaker detection run in NVIDIA's NeMo-Speech.cpp
// ("nemo-speech serve"), started the first time they're needed and kept
// running on the processor. The window records your voice or decodes an
// audio file into 16 kHz mono WAV, and Phoenix sends that to the engine.

const maxAudioSeconds = 30 * 60

type speechRunner struct {
	a       *App
	startMu sync.Mutex

	mu   sync.Mutex
	cmd  *exec.Cmd
	done chan struct{}
	base string
	key  string // API key for this run
	cfg  string // models it was started with
	logs *ringLog
}

func newSpeechRunner(a *App) *speechRunner { return &speechRunner{a: a} }

func (s *speechRunner) stop() {
	s.mu.Lock()
	cmd, done := s.cmd, s.done
	s.mu.Unlock()
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
	s.mu.Lock()
	if s.cmd == cmd {
		s.cmd = nil
	}
	s.mu.Unlock()
}

// SpeechStatus tells the window which voice features are ready.
type SpeechStatus struct {
	Listen   bool `json:"listen"`   // speech recognition is downloaded
	Speakers bool `json:"speakers"` // speaker detection is downloaded
	Voice    bool `json:"voice"`    // the Kokoro voice is downloaded
	ListenMB int  `json:"listenMb"` // download sizes, for the offer
	VoiceMB  int  `json:"voiceMb"`
}

// GetSpeechStatus says which voice features can be used.
func (a *App) GetSpeechStatus() SpeechStatus {
	st := SpeechStatus{}
	p, _ := a.installedOfKind(KindListen)
	st.Listen = p != ""
	p, _ = a.installedOfKind(KindSpeakers)
	st.Speakers = p != ""
	p, _ = a.installedOfKind(KindVoice)
	st.Voice = p != ""
	if m, ok := catalogByKind(KindListen); ok {
		st.ListenMB = int(m.totalSize() >> 20)
	}
	if m, ok := catalogByKind(KindVoice); ok {
		st.VoiceMB = int(m.totalSize() >> 20)
	}
	return st
}

// GetVoiceReady downloads what talking needs: speech recognition and the
// Kokoro voice, with their engines.
func (a *App) GetVoiceReady() error {
	for _, kind := range []string{KindListen, KindVoice} {
		m, ok := catalogByKind(kind)
		if !ok {
			continue
		}
		if a.modelComplete(m) {
			if p, ok := safeJoin(a.paths.Models, modelID(m.Repo, m.File)); ok && fileExists(p) {
				continue
			}
		}
		if err := a.DownloadModel(m.ID); err != nil && !strings.Contains(err.Error(), "already") {
			return err
		}
	}
	return nil
}

// ensure starts the speech engine with speech recognition (and speaker
// detection, if downloaded) and returns its address and key.
func (s *speechRunner) ensure(ctx context.Context) (string, string, bool, error) {
	s.startMu.Lock()
	defer s.startMu.Unlock()
	a := s.a
	asr, asrModel := a.installedOfKind(KindListen)
	if asr == "" {
		if m, ok := catalogByKind(KindListen); ok {
			if p, ok := safeJoin(a.paths.Models, modelID(m.Repo, m.File)); ok && fileExists(p) {
				_ = a.ensureTool(toolSpeech)
				return "", "", false, errors.New("Phoenix is still downloading its speech engine. Try again when it's done")
			}
		}
		return "", "", false, errNoSpeech
	}
	_ = asrModel
	diar, _ := a.installedOfKind(KindSpeakers)
	exe := a.toolPath(toolSpeech)
	cfg := asr + "|" + diar
	s.mu.Lock()
	if s.cmd != nil && s.cfg == cfg {
		base, key := s.base, s.key
		s.mu.Unlock()
		return base, key, diar != "", nil
	}
	s.mu.Unlock()
	s.stop()

	port, err := freePort()
	if err != nil {
		return "", "", false, err
	}
	args := []string{"serve", "--host", "127.0.0.1", "--port", strconv.Itoa(port), "--no-ui", "--asr-model", asr}
	if diar != "" {
		args = append(args, "--diar-model", diar)
	}
	kb := make([]byte, 24)
	_, _ = rand.Read(kb)
	key := hex.EncodeToString(kb)
	logs := newRingLog(120)
	cmd := exec.Command(exe, args...)
	cmd.Dir = filepath.Dir(exe)
	cmd.Env = append(os.Environ(), "NEMO_SPEECH_HTTP_API_KEY="+key)
	cmd.Stdout, cmd.Stderr = logs, logs
	prepareCmd(cmd)
	if err := cmd.Start(); err != nil {
		return "", "", false, fmt.Errorf("couldn't start the speech engine: %w", err)
	}
	bindToApp(cmd.Process)
	done := make(chan struct{})
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	s.mu.Lock()
	s.cmd, s.done, s.base, s.key, s.cfg, s.logs = cmd, done, base, key, cfg, logs
	s.mu.Unlock()
	go func() {
		cmd.Wait()
		close(done)
		s.mu.Lock()
		if s.cmd == cmd {
			s.cmd = nil
		}
		s.mu.Unlock()
	}()

	client := &http.Client{Timeout: 3 * time.Second}
	deadline := time.Now().Add(5 * time.Minute)
	for {
		select {
		case <-done:
			return "", "", false, errors.New("the speech engine stopped: " + lastLine(logs.String()))
		case <-ctx.Done():
			s.stop()
			return "", "", false, ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			s.stop()
			return "", "", false, errors.New("the speech engine took too long to start")
		}
		resp, err := client.Get(base + "/ready")
		if err != nil {
			continue
		}
		var r struct {
			Ready bool `json:"ready"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&r)
		resp.Body.Close()
		if resp.StatusCode == 200 && r.Ready {
			return base, key, diar != "", nil
		}
	}
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

var errNoSpeech = errors.New("speech recognition isn't downloaded yet. Get it on the Models page, under Voice")

// transcript is what the speech engine heard.
type transcript struct {
	Text     string
	Duration float64
	Speakers int
}

type asrWord struct {
	Word    string  `json:"word"`
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
	Speaker int     `json:"speaker"`
}

// transcribe sends 16-bit WAV audio to the speech engine.
func (a *App) transcribe(ctx context.Context, wav []byte, speakers bool) (transcript, error) {
	base, key, haveDiar, err := a.speech.ensure(ctx)
	if err != nil {
		return transcript{}, err
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "audio.wav")
	_, _ = fw.Write(wav)
	_ = mw.WriteField("response_format", "verbose_json")
	_ = mw.WriteField("language", "auto")
	diarize := speakers && haveDiar
	if diarize {
		_ = mw.WriteField("diarization", "true")
	}
	mw.Close()
	req, err := http.NewRequestWithContext(ctx, "POST", base+"/v1/audio/transcriptions", &body)
	if err != nil {
		return transcript{}, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+key)
	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return transcript{}, fmt.Errorf("couldn't reach the speech engine: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if resp.StatusCode != 200 {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		if e.Error.Message == "" {
			e.Error.Message = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return transcript{}, errors.New("the speech engine couldn't understand the audio: " + e.Error.Message)
	}
	var r struct {
		Text     string    `json:"text"`
		Duration float64   `json:"duration"`
		Words    []asrWord `json:"words"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return transcript{}, errors.New("the speech engine sent an answer Phoenix didn't understand")
	}
	t := transcript{Text: strings.TrimSpace(r.Text), Duration: r.Duration}
	if diarize {
		t.Text, t.Speakers = speakerTranscript(r.Words, t.Text)
	}
	return t, nil
}

// speakerTranscript turns words tagged with speakers into lines like
// "[0:42] Speaker 2: ...". Without speaker tags it returns the plain text.
func speakerTranscript(words []asrWord, plain string) (string, int) {
	seen := map[int]bool{}
	var b strings.Builder
	cur := -1
	for _, w := range words {
		if w.Speaker <= 0 {
			if cur < 0 {
				continue
			}
			w.Speaker = cur
		}
		seen[w.Speaker] = true
		if w.Speaker != cur {
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			fmt.Fprintf(&b, "[%s] Speaker %d:", clock(w.Start), w.Speaker)
			cur = w.Speaker
		}
		b.WriteString(" ")
		b.WriteString(strings.TrimSpace(w.Word))
	}
	if len(seen) == 0 {
		return plain, 0
	}
	return b.String(), len(seen)
}

func clock(sec float64) string {
	s := int(sec)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// decodeWAVUpload accepts the base64 WAV the window made and checks it.
func decodeWAVUpload(b64 string) ([]byte, float64, error) {
	if i := strings.Index(b64, ","); i >= 0 && strings.HasPrefix(b64, "data:") {
		b64 = b64[i+1:]
	}
	wav, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, 0, errors.New("the recording couldn't be read")
	}
	secs, ok := wavSeconds(wav)
	if !ok {
		return nil, 0, errors.New("the recording isn't a WAV file Phoenix can use")
	}
	if secs > maxAudioSeconds+5 {
		return nil, 0, errors.New("audio can be up to 30 minutes long")
	}
	return wav, secs, nil
}

// wavSeconds reads a PCM WAV header and returns its length in seconds.
func wavSeconds(b []byte) (float64, bool) {
	if len(b) < 44 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return 0, false
	}
	var rate, byteRate uint32
	for off := 12; off+8 <= len(b); {
		id := string(b[off : off+4])
		size := int(binary.LittleEndian.Uint32(b[off+4 : off+8]))
		body := off + 8
		switch id {
		case "fmt ":
			if body+16 > len(b) {
				return 0, false
			}
			rate = binary.LittleEndian.Uint32(b[body+4:])
			byteRate = binary.LittleEndian.Uint32(b[body+8:])
		case "data":
			if rate == 0 || byteRate == 0 {
				return 0, false
			}
			if body+size > len(b) {
				size = len(b) - body
			}
			return float64(size) / float64(byteRate), true
		}
		off = body + size + size%2
	}
	return 0, false
}

// Transcribe turns a recording from the microphone (base64 16 kHz WAV)
// into text.
func (a *App) Transcribe(wavB64 string) (string, error) {
	wav, secs, err := decodeWAVUpload(wavB64)
	if err != nil {
		return "", err
	}
	if secs < 0.3 {
		return "", nil
	}
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Minute)
	defer cancel()
	t, err := a.transcribe(ctx, wav, false)
	if err != nil {
		return "", err
	}
	return t.Text, nil
}

// TranscribeAttachment finishes an audio file added to the message box:
// the window decodes it to WAV, and Phoenix writes down what's said, with
// speaker names when speaker detection is downloaded.
func (a *App) TranscribeAttachment(id, wavB64 string) (Attachment, error) {
	att, ok := a.pendingAttachment(id)
	if !ok || att.Kind != attachmentKindAudio {
		return Attachment{}, errors.New("that audio file is no longer there. Add it again")
	}
	wav, secs, err := decodeWAVUpload(wavB64)
	if err != nil {
		return Attachment{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Minute)
	defer cancel()
	t, err := a.transcribe(ctx, wav, true)
	if err != nil {
		return Attachment{}, err
	}
	text := t.Text
	if strings.TrimSpace(text) == "" {
		text = "(No speech was heard in this audio.)"
	}
	if t.Duration == 0 {
		t.Duration = secs
	}
	if err := os.WriteFile(filepath.Join(a.paths.Pending, att.ID), []byte(text), 0o644); err != nil {
		return Attachment{}, err
	}
	att.Chars = len([]rune(text))
	att.Duration = t.Duration
	att.Speakers = t.Speakers
	att.Transcribing = false
	a.pending.mu.Lock()
	a.pending.items[att.ID] = att
	a.pending.mu.Unlock()
	return att, nil
}
