// A stand-in for llama-server used by the tests. It speaks the same
// small part of the API Phoenix uses: /health and streaming chat.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	args := os.Args[1:]
	port := ""
	gpu := true
	mmproj := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--list-devices":
			fmt.Println("Available devices:")
			fmt.Println("  Vulkan0: NVIDIA GeForce RTX 4070 (16311 MiB, 15112 MiB free)")
			return
		case "--port":
			port = args[i+1]
		case "--mmproj":
			mmproj = args[i+1]
		case "-ngl":
			if args[i+1] == "0" {
				gpu = false
			}
		}
	}
	if os.Getenv("FAKE_FAIL_GPU") == "1" && gpu {
		fmt.Fprintln(os.Stderr, "ggml_vulkan: ErrorOutOfDeviceMemory")
		fmt.Fprintln(os.Stderr, "failed to allocate buffer")
		os.Exit(1)
	}
	if os.Getenv("FAKE_FAIL_MMPROJ") == "1" && mmproj != "" {
		fmt.Fprintln(os.Stderr, "clip_init: failed to load model")
		os.Exit(1)
	}
	if os.Getenv("FAKE_FAIL_ALL") == "1" {
		fmt.Fprintln(os.Stderr, "llama_model_load: error loading model: unknown model architecture: 'qwen9'")
		os.Exit(1)
	}
	if gpu {
		fmt.Fprintln(os.Stderr, "load_tensors: offloaded 20/33 layers to GPU")
	} else {
		fmt.Fprintln(os.Stderr, "load_tensors: offloaded 0/33 layers to GPU")
	}
	start := time.Now()
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if time.Since(start) < 600*time.Millisecond {
			w.WriteHeader(503)
			return
		}
		w.Write([]byte(`{"status":"ok"}`))
	})
	http.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if key := os.Getenv("LLAMA_API_KEY"); key == "" || r.Header.Get("Authorization") != "Bearer "+key {
			w.WriteHeader(401)
			w.Write([]byte(`{"error":{"code":401,"message":"Invalid API Key","type":"authentication_error"}}`))
			return
		}
		var body struct {
			Messages []struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
			Kwargs map[string]any `json:"chat_template_kwargs"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		// Content is a string, or a list of text and image parts.
		raw := body.Messages[len(body.Messages)-1].Content
		last, pictures := "", 0
		if json.Unmarshal(raw, &last) != nil {
			var parts []struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				ImageURL struct {
					URL string `json:"url"`
				} `json:"image_url"`
			}
			json.Unmarshal(raw, &parts)
			for _, p := range parts {
				switch p.Type {
				case "text":
					last += p.Text
				case "image_url":
					if mmproj == "" {
						w.WriteHeader(500)
						w.Write([]byte(`{"error":{"code":500,"message":"image input is not supported - hint: if this is unexpected, you may need to provide the mmproj","type":"server_error"}}`))
						return
					}
					if strings.HasPrefix(p.ImageURL.URL, "data:image/jpeg;base64,") {
						pictures++
					}
				}
			}
		}
		if i := strings.LastIndex(last, "</file>"); i >= 0 {
			files := strings.Count(last, "<file name=")
			last = fmt.Sprintf("%d files then %s", files, strings.TrimSpace(last[i+len("</file>"):]))
		}
		if pictures > 0 {
			last = fmt.Sprintf("%d pictures then %s", pictures, last)
		}
		if strings.Contains(last, "too long") {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":{"code":400,"message":"the request exceeds the available context size, try increasing it","type":"exceed_context_size_error"}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fl := w.(http.Flusher)
		send := func(v any) {
			b, _ := json.Marshal(v)
			fmt.Fprintf(w, "data: %s\n\n", b)
			fl.Flush()
		}
		delta := func(k, v string) map[string]any {
			return map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{k: v}}}}
		}
		think, _ := body.Kwargs["enable_thinking"].(bool)
		if think {
			for _, t := range []string{"The user ", "said: ", last} {
				send(delta("reasoning_content", t))
			}
		}
		if strings.Contains(last, "tags") {
			for _, t := range []string{"<thi", "nk>raw tho", "ughts</th", "ink>\n\nTagged answer."} {
				send(delta("content", t))
				time.Sleep(5 * time.Millisecond)
			}
		} else {
			words := strings.Fields(fmt.Sprintf("You said **%s** and there are %d messages.", last, len(body.Messages)))
			slow := strings.Contains(last, "slow")
			for i, wd := range words {
				if i > 0 {
					wd = " " + wd
				}
				send(delta("content", wd))
				if slow {
					time.Sleep(150 * time.Millisecond)
				}
			}
		}
		send(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}},
			"timings": map[string]any{"predicted_per_second": 42.5}})
		fmt.Fprint(w, "data: [DONE]\n\n")
		fl.Flush()
	})
	if err := http.ListenAndServe("127.0.0.1:"+port, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
