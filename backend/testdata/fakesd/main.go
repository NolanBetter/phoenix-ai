// A stand-in for stable-diffusion.cpp's sd-server, used by the tests.
// It follows the async job API: submit, poll progress, get the result.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type job struct {
	kind     string
	body     map[string]any
	step     int
	total    int
	canceled bool
}

// The options Phoenix uses, as sd-server master-1088-32d516c defines them
// (examples/common/common.cpp and examples/server/runtime.cpp): true if the
// option takes a value. Like the real one, anything else prints an error and
// the help text, then exits. Check this list against the source of the
// pinned version whenever Phoenix passes a new option or the version changes.
var options = map[string]bool{
	"--listen-ip": true, "--listen-port": true, "--model-dir": true,
	"-m": true, "--vae": true, "--diffusion-model": true, "--t5xxl": true,
	"--cfg-scale": true, "--steps": true, "--sampling-method": true, "--flow-shift": true,
	"--backend": true, "--vae-tiling": false, "--offload-to-cpu": false,
}

func checkArgs(args []string) {
	if f := os.Getenv("FAKE_SD_LAUNCHES"); f != "" {
		fh, _ := os.OpenFile(f, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		fmt.Fprintln(fh, "start")
		fh.Close()
	}
	for i := 0; i < len(args); i++ {
		takesValue, ok := options[args[i]]
		if !ok || args[i] == os.Getenv("FAKE_SD_REJECT") {
			fmt.Fprintf(os.Stderr, "[E] common.cpp:353 - error: unknown argument: %s\n", args[i])
			fmt.Println("Usage: sd-server [options]\n\nSvr Options:\n  -l, --listen-ip <string>   server listen ip (default: 127.0.0.1)")
			for j := 0; j < 150; j++ {
				fmt.Printf("  --option-%d                 an option\n", j)
			}
			fmt.Println("  --negative-prompt-file     path to the file containing the negative prompt")
			os.Exit(1)
		}
		if takesValue {
			i++
		}
	}
}

func main() {
	port := ""
	args := os.Args[1:]
	checkArgs(args)
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--listen-port" {
			port = args[i+1]
		}
	}
	if f := os.Getenv("FAKE_SD_ARGS"); f != "" {
		os.WriteFile(f, []byte(strings.Join(args, "\n")), 0o644)
	}
	cpu := strings.Contains(strings.Join(args, " "), "--backend cpu")
	if os.Getenv("FAKE_SD_FAIL_GPU") == "1" && !cpu {
		fmt.Fprintln(os.Stderr, "ggml_vulkan: ErrorOutOfDeviceMemory")
		os.Exit(1)
	}
	time.Sleep(200 * time.Millisecond) // "loading the model"

	var mu sync.Mutex
	jobs := map[string]*job{}
	n := 0
	steps := 4
	if os.Getenv("FAKE_SD_SLOW") == "1" {
		steps = 400
	}
	http.HandleFunc("/sdcpp/v1/models/status", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"state":"ready","origin":"command_line"}`))
	})
	submit := func(kind string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(400)
				return
			}
			if f := os.Getenv("FAKE_SD_BODY"); f != "" {
				b, _ := json.Marshal(body)
				os.WriteFile(f, b, 0o644)
			}
			mu.Lock()
			n++
			id := fmt.Sprintf("job_%d", n)
			jobs[id] = &job{kind: kind, body: body, total: steps}
			mu.Unlock()
			w.WriteHeader(202)
			fmt.Fprintf(w, `{"id":%q,"kind":%q,"status":"queued","poll_url":"/sdcpp/v1/jobs/%s"}`, id, kind, id)
		}
	}
	http.HandleFunc("/sdcpp/v1/img_gen", submit("img_gen"))
	http.HandleFunc("/sdcpp/v1/vid_gen", submit("vid_gen"))
	http.HandleFunc("/sdcpp/v1/jobs/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/sdcpp/v1/jobs/")
		id, cancel := strings.CutSuffix(rest, "/cancel")
		mu.Lock()
		j := jobs[id]
		if j == nil {
			mu.Unlock()
			w.WriteHeader(404)
			return
		}
		if cancel {
			j.canceled = true
			mu.Unlock()
			w.Write([]byte(`{"cancel_requested":true}`))
			return
		}
		out := map[string]any{"id": id, "kind": j.kind}
		switch {
		case j.canceled:
			out["status"] = "cancelled"
			out["error"] = map[string]any{"code": "cancelled", "message": "job cancelled by client"}
		case os.Getenv("FAKE_SD_FAIL_JOB") == "1":
			out["status"] = "failed"
			out["error"] = map[string]any{"code": "generation_failed", "message": "generate_image returned empty results"}
		case j.step < j.total:
			j.step++
			out["status"] = "generating"
			out["progress"] = map[string]any{"stage": "sampling", "completed": j.step, "total": j.total, "unit": "steps"}
			if j.kind == "img_gen" {
				out["preview"] = map[string]any{"pass": 1, "step": j.step, "total_steps": j.total, "b64_json": pngB64(8, 8)}
			}
		default:
			out["status"] = "completed"
			if j.kind == "img_gen" {
				w, _ := j.body["width"].(float64)
				h, _ := j.body["height"].(float64)
				out["result"] = map[string]any{"output_format": "png", "images": []any{map[string]any{"index": 0, "b64_json": pngB64(int(w)/16, int(h)/16)}}}
			} else {
				out["result"] = map[string]any{"output_format": "webm", "mime_type": "video/webm", "b64_json": base64.StdEncoding.EncodeToString([]byte("\x1aE\xdf\xa3fake webm")), "fps": j.body["fps"], "frame_count": j.body["video_frames"]}
			}
		}
		mu.Unlock()
		json.NewEncoder(w).Encode(out)
	})
	http.ListenAndServe("127.0.0.1:"+port, nil)
}

func pngB64(w, h int) string {
	img := image.NewRGBA(image.Rect(0, 0, max(w, 1), max(h, 1)))
	for i := range img.Pix {
		img.Pix[i] = 200
	}
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var b bytes.Buffer
	png.Encode(&b, img)
	return base64.StdEncoding.EncodeToString(b.Bytes())
}
