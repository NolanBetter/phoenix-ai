package backend

import (
	"encoding/base64"
	"encoding/binary"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeWAV builds a 16 kHz mono 16-bit WAV of silence.
func makeWAV(seconds float64) []byte {
	const rate = 16000
	n := int(seconds * rate)
	b := make([]byte, 44+n*2)
	copy(b[0:], "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(36+n*2))
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1)
	binary.LittleEndian.PutUint16(b[22:], 1)
	binary.LittleEndian.PutUint32(b[24:], rate)
	binary.LittleEndian.PutUint32(b[28:], rate*2)
	binary.LittleEndian.PutUint16(b[32:], 2)
	binary.LittleEndian.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], uint32(n*2))
	return b
}

func wavB64(seconds float64) string { return base64.StdEncoding.EncodeToString(makeWAV(seconds)) }

func installSpeech(t *testing.T, a *App, speakers bool) {
	t.Helper()
	asr, _ := findCatalog("nemotron-asr")
	placeModel(t, a, asr.Repo, asr.File, fakeGGUF())
	if speakers {
		d, _ := findCatalog("nemotron-diarization")
		placeModel(t, a, d.Repo, d.File, fakeGGUF())
	}
}

func installVoice(t *testing.T, a *App) string {
	t.Helper()
	k, _ := findCatalog("kokoro")
	dir := filepath.Join(a.paths.Models, repoDir(k.Repo), k.Archive.Folder)
	os.MkdirAll(dir, 0o755)
	for _, f := range []string{"model.int8.onnx", "voices.bin", "tokens.txt", "lexicon-us-en.txt", "lexicon-gb-en.txt"} {
		os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644)
	}
	return dir
}

func TestWavSeconds(t *testing.T) {
	if s, ok := wavSeconds(makeWAV(2.5)); !ok || s != 2.5 {
		t.Errorf("got %v %v", s, ok)
	}
	if _, ok := wavSeconds([]byte("RIFF....WAVEjunk")); ok {
		t.Error("junk should fail")
	}
	if _, _, err := decodeWAVUpload("data:audio/wav;base64," + wavB64(1)); err != nil {
		t.Error(err)
	}
	if _, _, err := decodeWAVUpload(base64.StdEncoding.EncodeToString([]byte("not audio"))); err == nil {
		t.Error("not a WAV")
	}
}

func TestTranscribe(t *testing.T) {
	a, _, _ := setupApp(t)
	argsFile := filepath.Join(t.TempDir(), "args")
	t.Setenv("PHOENIX_NEMO_SPEECH", buildFake(t, "fakespeech"))
	t.Setenv("FAKE_SPEECH_ARGS", argsFile)

	if _, err := a.Transcribe(wavB64(1)); err == nil || !strings.Contains(err.Error(), "speech recognition") {
		t.Errorf("without the model: %v", err)
	}
	if st := a.GetSpeechStatus(); st.Listen || st.Voice || st.ListenMB < 600 || st.VoiceMB < 100 {
		t.Errorf("status: %+v", st)
	}
	installSpeech(t, a, false)
	if !a.GetSpeechStatus().Listen {
		t.Fatal("speech recognition should be ready")
	}
	text, err := a.Transcribe(wavB64(1.5))
	if err != nil || text != "Hello there. Hi Phoenix." {
		t.Fatalf("got %q, %v", text, err)
	}
	args := readLines(t, argsFile)
	if args[0] != "serve" || !strings.HasSuffix(argAfter(args, "--asr-model"), ".q8_0.gguf") || argAfter(args, "--host") != "127.0.0.1" || argAfter(args, "--diar-model") != "" {
		t.Errorf("args: %v", args)
	}
	// Too short to hold words: nothing to send.
	if text, err := a.Transcribe(wavB64(0.1)); err != nil || text != "" {
		t.Errorf("short: %q %v", text, err)
	}
}

func TestAudioAttachment(t *testing.T) {
	a, rec, model := setupApp(t)
	t.Setenv("PHOENIX_NEMO_SPEECH", buildFake(t, "fakespeech"))
	mp3 := base64.StdEncoding.EncodeToString(append([]byte("ID3\x04\x00\x00\x00\x00\x00\x00"), make([]byte, 200)...))

	// Audio needs speech recognition.
	if _, err := a.AddAttachmentData("meeting.mp3", mp3); err == nil || !strings.Contains(err.Error(), "speech recognition") {
		t.Errorf("expected a speech recognition error, got %v", err)
	}
	installSpeech(t, a, true)
	att, err := a.AddAttachmentData("meeting.mp3", mp3)
	if err != nil {
		t.Fatal(err)
	}
	if att.Kind != "audio" || !att.Transcribing || !strings.HasSuffix(att.ID, ".txt") || !strings.HasSuffix(att.Audio, ".mp3") {
		t.Fatalf("audio: %+v", att)
	}
	// The sound can be played from the message box.
	w := httptest.NewRecorder()
	AttachmentHandler(a).ServeHTTP(w, httptest.NewRequest("GET", "/att/pending/"+att.Audio, nil))
	if w.Code != 200 || w.Header().Get("Content-Type") != "audio/mpeg" {
		t.Errorf("serve audio: %d %s", w.Code, w.Header().Get("Content-Type"))
	}
	if _, err := a.SendMessage("", "summarize", model, false, []string{att.ID}); err == nil || !strings.Contains(err.Error(), "still being written down") {
		t.Errorf("sending before the transcript: %v", err)
	}
	if len(a.ListChats()) != 0 {
		t.Error("no chat should be made")
	}
	att, err = a.TranscribeAttachment(att.ID, wavB64(2.1))
	if err != nil {
		t.Fatal(err)
	}
	if att.Transcribing || att.Speakers != 2 || att.Duration != 2.1 || att.Chars == 0 {
		t.Errorf("after: %+v", att)
	}
	text, _ := os.ReadFile(filepath.Join(a.paths.Pending, att.ID))
	want := "[0:00] Speaker 1: Hello there.\n\n[0:01] Speaker 2: Hi Phoenix."
	if string(text) != want {
		t.Errorf("transcript %q", text)
	}

	n := rec.count()
	chatID, err := a.SendMessage("", "summarize", model, false, []string{att.ID})
	if err != nil {
		t.Fatal(err)
	}
	if d := rec.waitDone(t, n); !strings.Contains(d.Message.Content, "1 files then summarize") {
		t.Errorf("answer: %+v", d.Message)
	}
	for _, f := range []string{att.ID, att.Audio} {
		if !fileExists(filepath.Join(a.paths.Chats, chatID, f)) {
			t.Errorf("%s should be in the chat folder", f)
		}
	}
	ch, _ := a.GetChat(chatID)
	p := buildPrompt("", ch.Messages, 8192, false, chatFiles{a, chatID})
	if s, _ := p[0].Content.(string); !strings.Contains(s, "Transcript of this audio recording (0:02 long)") || !strings.Contains(s, "Speaker 2: Hi Phoenix.") {
		t.Errorf("prompt: %q", p[0].Content)
	}
	// Removing an audio file from the message box deletes both files.
	b, _ := a.AddAttachmentData("b.wav", base64.StdEncoding.EncodeToString(makeWAV(0.5)))
	a.RemovePendingAttachment(b.ID)
	if fileExists(filepath.Join(a.paths.Pending, b.Audio)) {
		t.Error("the sound should be deleted too")
	}
	if got, n := speakerTranscript(nil, "plain"); got != "plain" || n != 0 {
		t.Errorf("no speakers: %q", got)
	}
}

func TestSpeak(t *testing.T) {
	a, _, _ := setupApp(t)
	argsFile := filepath.Join(t.TempDir(), "args")
	t.Setenv("PHOENIX_SHERPA_TTS", buildFake(t, "faketts"))
	t.Setenv("FAKE_TTS_ARGS", argsFile)

	if _, err := a.Speak("hello"); err == nil || !strings.Contains(err.Error(), "voice") {
		t.Errorf("without the voice: %v", err)
	}
	dir := installVoice(t, a)
	url, err := a.Speak("## Hello\n\nHere is **bold** and `code`:\n```go\nfmt.Println(1)\n```\nSee [the docs](https://x.y) or https://a.b/c.")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "/att/voice/") || !strings.HasSuffix(url, ".wav") {
		t.Fatalf("url %q", url)
	}
	args := readLines(t, argsFile)
	joined := strings.Join(args, "\n")
	text := args[len(args)-1]
	if strings.ContainsAny(text, "#*`[]") || !strings.Contains(text, "The code is on screen") || strings.Contains(text, "Println") || !strings.Contains(text, "the docs") || strings.Contains(text, "https") {
		t.Errorf("spoken text %q", text)
	}
	if !strings.Contains(joined, "--kokoro-model="+filepath.Join(dir, "model.int8.onnx")) || !strings.Contains(joined, "--sid=3") || !strings.Contains(joined, "lexicon-us-en.txt") || !strings.Contains(joined, "--kokoro-length-scale=1.00") {
		t.Errorf("args: %v", args)
	}
	w := httptest.NewRecorder()
	AttachmentHandler(a).ServeHTTP(w, httptest.NewRequest("GET", url, nil))
	if w.Code != 200 || w.Header().Get("Content-Type") != "audio/wav" {
		t.Errorf("serve: %d %s", w.Code, w.Header().Get("Content-Type"))
	}
	a.ForgetSpeech(url)
	if fileExists(filepath.Join(a.paths.Voice, strings.TrimPrefix(url, "/att/voice/"))) {
		t.Error("ForgetSpeech should delete it")
	}

	// A British voice uses the British word list; speed changes the pace.
	s := a.GetSettings()
	s.Voice, s.VoiceSpeed = "bm_george", 1.25
	a.SaveSettings(s)
	if _, err := a.Speak("-- leading dashes"); err != nil {
		t.Fatal(err)
	}
	args = readLines(t, argsFile)
	joined = strings.Join(args, "\n")
	if !strings.Contains(joined, "--sid=26") || !strings.Contains(joined, "lexicon-gb-en.txt") || !strings.Contains(joined, "--kokoro-length-scale=0.80") || strings.HasPrefix(args[len(args)-1], "-") {
		t.Errorf("args: %v", args)
	}
	// Unknown voices and silly speeds are ignored.
	s.Voice, s.VoiceSpeed = "nope", 9
	got, _ := a.SaveSettings(s)
	if got.Voice != "bm_george" || got.VoiceSpeed != 1.25 {
		t.Errorf("settings: %+v", got)
	}
	if len(a.GetVoices()) != len(kokoroVoices) {
		t.Error("every voice should be listed")
	}
	if _, err := a.Speak("```\nonly code\n```"); err != nil {
		t.Errorf("code only: %v", err)
	}
	if _, err := a.Speak("   "); err == nil {
		t.Error("nothing to say should fail")
	}
}

func TestSplitSentences(t *testing.T) {
	got := splitSentences("Hi there! How are you? Fine. ")
	if len(got) != 3 || got[0] != "Hi there!" || got[2] != "Fine." {
		t.Errorf("%q", got)
	}
}
