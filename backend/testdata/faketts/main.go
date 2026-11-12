// A stand-in for sherpa-onnx-offline-tts, used by the tests. It writes a
// short silent WAV and records its options.
package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

func main() {
	out := "./generated.wav"
	var text []string
	for _, a := range os.Args[1:] {
		if strings.HasPrefix(a, "--output-filename=") {
			out = strings.TrimPrefix(a, "--output-filename=")
		} else if !strings.HasPrefix(a, "--") {
			text = append(text, a)
		}
	}
	if f := os.Getenv("FAKE_TTS_ARGS"); f != "" {
		os.WriteFile(f, []byte(strings.Join(os.Args[1:], "\n")), 0o644)
	}
	if len(text) != 1 {
		fmt.Fprintln(os.Stderr, "Error: Accept only one positional argument.")
		os.Exit(1)
	}
	const rate = 24000
	samples := rate / 2
	b := make([]byte, 44+samples*2)
	copy(b[0:], "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(36+samples*2))
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1)
	binary.LittleEndian.PutUint16(b[22:], 1)
	binary.LittleEndian.PutUint32(b[24:], rate)
	binary.LittleEndian.PutUint32(b[28:], rate*2)
	binary.LittleEndian.PutUint16(b[32:], 2)
	binary.LittleEndian.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], uint32(samples*2))
	if err := os.WriteFile(out, b, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
