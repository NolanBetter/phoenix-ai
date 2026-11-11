package backend

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// GPU is a graphics card the engine can use.
type GPU struct {
	Name string `json:"name"`
	VRAM int64  `json:"vram"`
}

// SystemInfo describes this PC, used to say which models will run well.
type SystemInfo struct {
	RAM        int64 `json:"ram"`
	GPUs       []GPU `json:"gpus"`
	BestVRAM   int64 `json:"bestVram"`
	CPUThreads int   `json:"cpuThreads"`
	DiskFree   int64 `json:"diskFree"`
	GPUChecked bool  `json:"gpuChecked"`
}

// GetSystemInfo returns memory, graphics cards and free disk space.
// Graphics cards are found by asking the engine, so they show up after setup.
func (a *App) GetSystemInfo() SystemInfo {
	a.sysMu.Lock()
	if a.sys == nil {
		a.sys = &SystemInfo{RAM: totalRAM(), CPUThreads: runtime.NumCPU()}
	}
	needGPU := !a.sys.GPUChecked
	a.sysMu.Unlock()

	if needGPU {
		if exe := a.engine.exePath(); exe != "" {
			gpus, err := listDevices(exe)
			a.sysMu.Lock()
			if err == nil {
				a.sys.GPUs = gpus
				a.sys.BestVRAM = 0
				for _, g := range gpus {
					if g.VRAM > a.sys.BestVRAM {
						a.sys.BestVRAM = g.VRAM
					}
				}
			}
			a.sys.GPUChecked = true
			a.sysMu.Unlock()
		}
	}

	a.sysMu.Lock()
	defer a.sysMu.Unlock()
	info := *a.sys
	info.DiskFree = diskFree(a.paths.Root)
	return info
}

func (a *App) sysSnapshot() *SystemInfo {
	info := a.GetSystemInfo()
	return &info
}

func (a *App) resetGPUInfo() {
	a.sysMu.Lock()
	if a.sys != nil {
		a.sys.GPUChecked = false
	}
	a.sysMu.Unlock()
}

// listDevices runs `llama-server --list-devices` and reads lines like
//
//	Vulkan0: NVIDIA GeForce RTX 4070 (16311 MiB, 15112 MiB free)
func listDevices(exe string) ([]GPU, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "--list-devices")
	prepareCmd(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		return nil, err
	}
	return parseDevices(string(out)), nil
}

var deviceLine = regexp.MustCompile(`^\s*([^\s:]+):\s+(.+?)\s+\((\d+) MiB,\s*(\d+) MiB free\)\s*$`)

func parseDevices(out string) []GPU {
	var gpus []GPU
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		m := deviceLine.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		mib, _ := strconv.ParseInt(m[3], 10, 64)
		gpus = append(gpus, GPU{Name: strings.TrimSpace(m[2]), VRAM: mib << 20})
	}
	return gpus
}

func humanBytes(n int64) string {
	switch {
	case n >= gb:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(gb))
	case n >= 1<<20:
		return fmt.Sprintf("%.0f MB", float64(n)/float64(1<<20))
	default:
		return fmt.Sprintf("%d KB", n>>10)
	}
}
