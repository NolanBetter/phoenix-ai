// A stand-in for NeMo-Speech.cpp's "nemo-speech serve", used by the tests.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 || args[0] != "serve" {
		fmt.Fprintln(os.Stderr, "usage: nemo-speech serve ...")
		os.Exit(2)
	}
	port, diar := "", ""
	for i := 0; i < len(args)-1; i++ {
		switch args[i] {
		case "--port":
			port = args[i+1]
		case "--diar-model":
			diar = args[i+1]
		}
	}
	if f := os.Getenv("FAKE_SPEECH_ARGS"); f != "" {
		os.WriteFile(f, []byte(strings.Join(args, "\n")), 0o644)
	}
	key := os.Getenv("NEMO_SPEECH_HTTP_API_KEY")
	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ready":true}`))
	})
	http.HandleFunc("/v1/audio/transcriptions", func(w http.ResponseWriter, r *http.Request) {
		if key == "" || r.Header.Get("Authorization") != "Bearer "+key {
			w.WriteHeader(401)
			w.Write([]byte(`{"error":{"message":"missing or invalid API key","type":"invalid_request_error"}}`))
			return
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":{"message":"file is required","type":"invalid_request_error"}}`))
			return
		}
		b, _ := io.ReadAll(f)
		if len(b) < 44 || string(b[:4]) != "RIFF" {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":{"message":"not a WAV file","type":"invalid_request_error"}}`))
			return
		}
		words := []map[string]any{
			{"word": "Hello", "start": 0.1, "end": 0.4, "confidence": 0.9},
			{"word": "there.", "start": 0.5, "end": 0.8, "confidence": 0.9},
			{"word": "Hi", "start": 1.2, "end": 1.4, "confidence": 0.9},
			{"word": "Phoenix.", "start": 1.5, "end": 2.0, "confidence": 0.9},
		}
		diarize := r.FormValue("diarization") == "true"
		if diarize && diar == "" {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":{"message":"diarization requires a diarizer model","type":"invalid_request_error"}}`))
			return
		}
		if diarize {
			for i, wd := range words {
				wd["speaker"] = 1 + i/2
			}
		}
		json.NewEncoder(w).Encode(map[string]any{
			"text": "Hello there. Hi Phoenix.", "task": "transcribe", "duration": 2.1, "language": "en-US", "words": words,
		})
	})
	http.ListenAndServe("127.0.0.1:"+port, nil)
}
