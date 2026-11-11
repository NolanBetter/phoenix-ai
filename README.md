# Phoenix AI

Run open-source AI models on your own Windows PC. Pick a model, Phoenix downloads it, and you chat with it. Nothing you type leaves the computer.

**[Download the latest installer](https://github.com/NolanBetter/phoenix-ai/releases/latest)** (`PhoenixAI-Setup-<version>.exe`). It installs just for you, with no administrator prompt, and updates itself after that.

> Windows may say "Windows protected your PC" the first time, because the installer isn't code-signed yet. Click **More info**, then **Run anyway**. Updates after that don't show the warning.

## How it works

- **The engine** is [llama.cpp](https://github.com/ggml-org/llama.cpp)'s `llama-server`. On first launch Phoenix downloads the official Windows Vulkan build from GitHub. Vulkan runs on NVIDIA, AMD and Intel graphics cards, and falls back to the processor when there isn't one.
- **Models** are GGUF files from Hugging Face. There's a curated list (Qwen 3.5, Gemma 4, gpt-oss and more), plus search for anything else on Hugging Face. Downloads resume if interrupted and are checked against Hugging Face's SHA-256 checksum. Phoenix keeps up to 5 models and 10 chats (`MaxModels` and `MaxChats` in `backend/app.go`).
- **Running a model**: Phoenix starts `llama-server` hidden in the background on a random local port with a random per-run key, and llama.cpp fits the model to your graphics card's memory automatically. If the graphics card can't load it, Phoenix retries on the processor. The engine is tied to Phoenix with a Windows job object, so it closes when Phoenix closes, even after a crash.
- **The app** installs to `%LOCALAPPDATA%\Programs\Phoenix AI`, with "Phoenix AI" shortcuts in the Start menu and on the desktop, and an entry in Settings > Apps for uninstalling.
- **Your data** lives separately in `%LOCALAPPDATA%\PhoenixAI`, so installing, updating or uninstalling never touches it (the uninstaller asks before deleting it):

```
PhoenixAI\
  engine\b11669\llama-server.exe (+ DLLs)
  models\unsloth__Qwen3.5-4B-GGUF\Qwen3.5-4B-Q4_K_M.gguf
  chats\<id>.json
  updates\            (a downloaded update waiting to install)
  webview\            (the window's browser cache)
  settings.json
```

## Updates

Phoenix checks this repository's [Releases](https://github.com/NolanBetter/phoenix-ai/releases) about 20 seconds after it opens and every 6 hours after that. When there's a newer version it downloads `PhoenixAI-Setup-<version>.exe`, checks it against its SHA-256 checksum, and shows **Update ready** in the sidebar. Clicking that restarts Phoenix into the new version. If you just close Phoenix instead, the update installs quietly in the background. You can turn automatic updates off in Settings.

Phoenix downloads the installer itself rather than through a browser, so Windows doesn't flag it as a file from the internet and the "Windows protected your PC" warning doesn't appear for updates.

## Releasing a new version

1. Change the number in `VERSION` (for example `0.2.1`).
2. Add a `## 0.2.1` section at the top of `CHANGELOG.md`. It becomes the release notes and the "What's new" text inside Phoenix.
3. Commit, then tag and push:

```
git commit -am "Phoenix AI 0.2.1"
git tag v0.2.1
git push origin main v0.2.1
```

GitHub Actions (`.github/workflows/release.yml`) runs the tests, builds the app and installer, and publishes the release. Everyone's Phoenix picks it up within a few hours.

## Project layout

```
main.go                 Window setup (Wails v2)
VERSION                 The version number, used everywhere
CHANGELOG.md            Release notes
backend/                Everything behind the window, no Wails imports
  app.go                App struct bound to JavaScript, start/stop, limits
  engine.go             Finds and installs llama.cpp from GitHub releases
  runner.go             Starts/stops llama-server, health checks, GPU fallback
  chat.go               Chats on disk, streaming answers, thinking, unread
  downloads.go          Model downloads (resume, checksum), Hugging Face search
  models.go             Installed models, delete, model limit
  update.go             Self-updates from GitHub Releases
  catalog.go            Recommended models and the "will it run" estimate
  sysinfo.go            RAM, graphics cards (via llama-server --list-devices)
  platform_windows.go   Hidden console window, job object, RAM, disk, installer launch
  *_test.go             Tests (a fake llama-server and fake GitHub)
frontend/dist/          The UI: plain HTML, CSS and JavaScript, no build step
frontend/dev/           Browser preview with a fake backend
build/installer/        The NSIS installer script and its picture
tools/mkres/            Writes the Windows icon, manifest and version info
build/appicon.*         App icon
```

## Building

Needs [Go](https://go.dev/dl/) 1.26 or newer, plus [NSIS](https://nsis.sourceforge.io) for the installer.

On Windows, in PowerShell:

```powershell
.\build.ps1
```

On Linux or macOS (cross-compiles the Windows app):

```
./build.sh
```

Both write `build\PhoenixAI.exe` and `build\PhoenixAI-Setup-<version>.exe`. The window uses Microsoft Edge WebView2, which is built into Windows 10 and 11.

## Working on the UI

Open `frontend/dist/index.html` straight from disk in Chrome or Edge to see the UI with fake data (from `frontend/dev/mock.js`), no Go needed. Add `?scenario=` with any of `noEngine`, `noModels`, `potato`, `downloading`, `slow`, `light`, `full` (10 chats and 5 models) or `update` (an update is ready) to the address. Combine them with commas.

## Tests

```
go test ./backend/
PHOENIX_NET_TEST=1 go test -run Real ./backend/    # real Hugging Face download, about 500 MB
```

## Calling the backend from JavaScript

Every exported method on `backend.App` is available as `window.go.backend.App.<Method>()` and returns a promise. Events come in through `window.runtime.EventsOn(name, callback)`:

| Event | When |
|---|---|
| `engine:status` | Engine setup progress |
| `download:progress` | Model download progress, done, paused or error |
| `models:changed` | A model was added or deleted |
| `model:status` | A model is loading, ready, or failed |
| `chat:start`, `chat:delta`, `chat:done` | An answer starts, streams in, finishes |
| `chats:changed` | The chat list changed (including unread dots) |
| `update:status` | Checking for, downloading, or ready to install an update |

## Licenses

llama.cpp (MIT) is downloaded at runtime. Bundled: marked (MIT), DOMPurify (Apache 2.0 / MPL 2.0), Bricolage Grotesque (SIL Open Font License), Wails (MIT). Their license files are in `frontend/dist/vendor/`. Each model keeps its own license; the recommended ones are all Apache 2.0.
