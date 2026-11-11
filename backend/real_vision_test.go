package backend

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Real engine + real vision model. Run with
// PHOENIX_REAL_DIR=<folder with Qwen3.5-0.8B-Q4_K_M.gguf and mmproj-F16.gguf>
// PHOENIX_REAL_SERVER=<llama-server>.
func TestRealVision(t *testing.T) {
	dir, server := os.Getenv("PHOENIX_REAL_DIR"), os.Getenv("PHOENIX_REAL_SERVER")
	if dir == "" || server == "" {
		t.Skip("set PHOENIX_REAL_DIR and PHOENIX_REAL_SERVER")
	}
	t.Setenv("PHOENIX_HOME", t.TempDir())
	t.Setenv("PHOENIX_LLAMA_SERVER", server)
	t.Setenv("PHOENIX_NO_UPDATE", "1")
	rec := &recorder{}
	a := NewApp()
	if err := Start(a, context.Background(), rec.emit); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { Stop(a) })
	id := modelID("unsloth/Qwen3.5-0.8B-GGUF", "Qwen3.5-0.8B-Q4_K_M.gguf")
	mdir := filepath.Join(a.paths.Models, repoDir("unsloth/Qwen3.5-0.8B-GGUF"))
	os.MkdirAll(mdir, 0o755)
	for _, f := range []string{"Qwen3.5-0.8B-Q4_K_M.gguf", "mmproj-F16.gguf"} {
		if err := os.Symlink(filepath.Join(dir, f), filepath.Join(mdir, f)); err != nil {
			t.Fatal(err)
		}
	}
	s := a.GetSettings()
	s.UseGPU = false
	a.SaveSettings(s)

	// A big photo-sized picture: a red square on a blue background.
	img := image.NewRGBA(image.Rect(0, 0, 2400, 1800))
	for y := 0; y < 1800; y++ {
		for x := 0; x < 2400; x++ {
			c := color.RGBA{20, 60, 200, 255}
			if x > 800 && x < 1600 && y > 500 && y < 1300 {
				c = color.RGBA{220, 20, 20, 255}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	pic, err := a.AddAttachmentData("shapes.png", base64.StdEncoding.EncodeToString(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("picture stored as %dx%d", pic.Width, pic.Height)
	note, _ := a.AddAttachmentData("note.txt", base64.StdEncoding.EncodeToString([]byte("The secret word is PINEAPPLE.")))

	n := rec.count()
	chatID, err := a.SendMessage("", "What color is the square in the middle of the picture? One word.", id, false, []string{pic.ID})
	if err != nil {
		t.Fatal(err)
	}
	d := rec.waitDone(t, n)
	t.Logf("status: %+v", a.GetModelStatus())
	t.Logf("answer: %q (%.1f tok/s) err=%q", d.Message.Content, d.Message.TokensPerSec, d.Message.Error)
	if d.Message.Error != "" || !strings.Contains(strings.ToLower(d.Message.Content), "red") {
		t.Fatalf("bad picture answer")
	}

	n = rec.count()
	if _, err := a.SendMessage(chatID, "What is the secret word in note.txt?", id, false, []string{note.ID}); err != nil {
		t.Fatal(err)
	}
	d = rec.waitDone(t, n)
	t.Logf("answer: %q err=%q", d.Message.Content, d.Message.Error)
	if d.Message.Error != "" || !strings.Contains(strings.ToLower(d.Message.Content), "pineapple") {
		t.Fatalf("bad file answer")
	}
}
