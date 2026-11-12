package backend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Phoenix reads answers aloud with the Kokoro voice, run by sherpa-onnx's
// sherpa-onnx-offline-tts. Each call writes a WAV file into the voice
// folder, which the window plays from /att/voice/<file>.

// kokoroVoices are the English voices in Kokoro v1.0, by their speaker
// number in voices.bin.
var kokoroVoices = map[string]int{
	"af_heart": 3, "af_bella": 2, "af_nicole": 6, "af_sky": 10,
	"am_michael": 16, "am_adam": 11, "am_fenrir": 13,
	"bf_emma": 21, "bf_isabella": 22, "bm_george": 26, "bm_lewis": 27,
}

// VoiceOption is a voice for the Settings list.
type VoiceOption struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Accent string `json:"accent"`
}

// GetVoices lists the voices you can pick in Settings.
func (a *App) GetVoices() []VoiceOption {
	return []VoiceOption{
		{"af_heart", "Heart", "American"}, {"af_bella", "Bella", "American"}, {"af_nicole", "Nicole", "American"},
		{"af_sky", "Sky", "American"}, {"am_michael", "Michael", "American"}, {"am_adam", "Adam", "American"},
		{"am_fenrir", "Fenrir", "American"}, {"bf_emma", "Emma", "British"}, {"bf_isabella", "Isabella", "British"},
		{"bm_george", "George", "British"}, {"bm_lewis", "Lewis", "British"},
	}
}

var (
	codeBlock   = regexp.MustCompile("(?s)```.*?(```|$)")
	inlineCode  = regexp.MustCompile("`([^`]*)`")
	mdLink      = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	mdNoise     = regexp.MustCompile(`(?m)^\s{0,3}(#{1,6}\s+|>\s?|[-*+]\s+|\d+[.)]\s+)|[*_~|]`)
	bareURL     = regexp.MustCompile(`https?://\S+`)
	manySpaces  = regexp.MustCompile(`[ \t]+`)
	thinkBlock  = regexp.MustCompile(`(?s)<think>.*?</think>`)
	sentenceEnd = regexp.MustCompile(`([.!?…])\s+`)
)

// speakableText turns a markdown answer into what should be read aloud:
// no code, no symbols, no links.
func speakableText(s string) string {
	s = thinkBlock.ReplaceAllString(s, "")
	s = codeBlock.ReplaceAllString(s, " (The code is on screen.) ")
	s = inlineCode.ReplaceAllString(s, "$1")
	s = mdLink.ReplaceAllString(s, "$1")
	s = bareURL.ReplaceAllString(s, "a link")
	s = mdNoise.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\r", "")
	// Paragraphs become pauses.
	s = regexp.MustCompile(`\n{2,}`).ReplaceAllString(s, ". ")
	s = strings.ReplaceAll(s, "\n", ", ")
	s = manySpaces.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, ". .", ".")
	s = strings.ReplaceAll(s, ":.", ".")
	return strings.TrimSpace(s)
}

const maxSpeakChars = 4000

var voiceMu sync.Mutex // one voice at a time keeps the processor free for the chat model

// Speak reads text aloud with the Kokoro voice and returns the address of
// the WAV file to play.
func (a *App) Speak(text string) (string, error) {
	// A leading dash would be taken as an option by the voice engine.
	text = strings.TrimLeft(speakableText(text), "-–— ")
	if text == "" {
		return "", errors.New("there's nothing to read aloud")
	}
	if len([]rune(text)) > maxSpeakChars {
		r := []rune(text)
		cut := string(r[:maxSpeakChars])
		if i := strings.LastIndexAny(cut, ".!?"); i > maxSpeakChars/2 {
			cut = cut[:i+1]
		}
		text = cut
	}
	model, cat := a.installedOfKind(KindVoice)
	if model == "" {
		if m, ok := catalogByKind(KindVoice); ok {
			if p, ok := safeJoin(a.paths.Models, modelID(m.Repo, m.File)); ok && fileExists(p) {
				_ = a.ensureTool(toolVoice)
				return "", errors.New("Phoenix is still downloading its voice engine. Try again when it's done")
			}
		}
		return "", errors.New("the voice isn't downloaded yet. Get Kokoro voice on the Models page, under Voice")
	}
	_ = cat
	exe := a.toolPath(toolVoice)
	dir := filepath.Dir(model)
	s := a.getSettings()
	sid, ok := kokoroVoices[s.Voice]
	if !ok {
		sid = kokoroVoices["af_heart"]
	}
	lexicon := filepath.Join(dir, "lexicon-us-en.txt")
	if strings.HasPrefix(s.Voice, "b") && fileExists(filepath.Join(dir, "lexicon-gb-en.txt")) {
		lexicon = filepath.Join(dir, "lexicon-gb-en.txt")
	}
	speed := s.VoiceSpeed
	if speed < 0.7 || speed > 1.5 {
		speed = 1
	}
	out := filepath.Join(a.paths.Voice, newAttachmentID(".wav"))
	threads := runtime.NumCPU() / 2
	if threads < 1 {
		threads = 1
	}
	if threads > 4 {
		threads = 4
	}
	args := []string{
		"--kokoro-model=" + model,
		"--kokoro-voices=" + filepath.Join(dir, "voices.bin"),
		"--kokoro-tokens=" + filepath.Join(dir, "tokens.txt"),
		"--kokoro-lexicon=" + lexicon,
		"--kokoro-length-scale=" + strconv.FormatFloat(1/speed, 'f', 2, 64),
		"--num-threads=" + strconv.Itoa(threads),
		"--sid=" + strconv.Itoa(sid),
		"--output-filename=" + out,
		text,
	}

	voiceMu.Lock()
	defer voiceMu.Unlock()
	ctx, cancel := context.WithTimeout(a.ctx, 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = filepath.Dir(exe)
	logs := newRingLog(60)
	cmd.Stdout, cmd.Stderr = logs, logs
	prepareCmd(cmd)
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("couldn't start the voice engine: %w", err)
	}
	bindToApp(cmd.Process)
	err := cmd.Wait()
	if err != nil || !fileExists(out) {
		os.Remove(out)
		if ctx.Err() != nil {
			return "", errors.New("reading aloud took too long")
		}
		return "", errors.New("the voice engine couldn't read that: " + lastLine(logs.String()))
	}
	return "/att/voice/" + filepath.Base(out), nil
}

// ForgetSpeech deletes a spoken answer once it has been played.
func (a *App) ForgetSpeech(url string) {
	name := strings.TrimPrefix(url, "/att/voice/")
	if attIDPattern.MatchString(name) {
		os.Remove(filepath.Join(a.paths.Voice, name))
	}
}

// cleanupVoice clears spoken answers left from an earlier run.
func cleanupVoice(dir string) { cleanupPending(dir) }

// splitSentences is used by tests and by the window's voice chat to speak
// the start of an answer while the rest is still being written.
func splitSentences(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := sentenceEnd.Split(s, -1)
	ends := sentenceEnd.FindAllStringSubmatch(s, -1)
	out := make([]string, 0, len(parts))
	for i, p := range parts {
		if i < len(ends) {
			p += ends[i][1]
		}
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
