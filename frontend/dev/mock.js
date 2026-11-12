// Development-only stand-in for the Go side, so the UI can be previewed in a
// normal browser. Not included in the app (it lives outside frontend/dist).
(() => {
  // Pick a state with ?scenario=noEngine,noModels,potato,downloading,slow,light,full,update,working
  const sc = window.__scenario || {};
  for (const k of (new URLSearchParams(location.search).get("scenario") || "").split(",").filter(Boolean)) {
    if (k === "light") sc.theme = "light";
    else sc[k] = true;
  }
  const listeners = {};
  const emit = (n, d) => (listeners[n] || []).forEach((cb) => cb(d));
  window.__emit = emit;
  let maximised = !sc.restored;
  window.runtime = {
    EventsOn(n, cb) { (listeners[n] ||= []).push(cb); return () => {}; },
    BrowserOpenURL() {},
    ClipboardSetText: async () => true,
    WindowMinimise() {},
    WindowMaximise() { maximised = true; dispatchEvent(new Event("resize")); },
    WindowToggleMaximise() { maximised = !maximised; dispatchEvent(new Event("resize")); },
    WindowIsMaximised: async () => maximised,
    Quit() {},
    OnFileDrop() {},
  };

  // Files "added" in the mock live here as data URLs or text.
  const files = {};
  let fileSeq = 0;
  const newId = (ext) => (++fileSeq).toString(16).padStart(16, "a") + ext;
  window.__attURL = (id) => files[id] || "";
  window.__attText = (id) => files[id] || "";
  const drawPicture = (w, h, paint) => {
    const c = document.createElement("canvas");
    c.width = w; c.height = h;
    paint(c.getContext("2d"), w, h);
    return c.toDataURL("image/jpeg", 0.88);
  };
  const sketch = () => drawPicture(1280, 853, (g, w, h) => {
    const sky = g.createLinearGradient(0, 0, 0, h);
    sky.addColorStop(0, "#2a1530"); sky.addColorStop(0.6, "#e8492e"); sky.addColorStop(1, "#f5b23a");
    g.fillStyle = sky; g.fillRect(0, 0, w, h);
    g.fillStyle = "#fff3d6"; g.beginPath(); g.arc(w * 0.62, h * 0.58, 120, 0, 7); g.fill();
    g.fillStyle = "#1d1012";
    g.beginPath(); g.moveTo(0, h); g.lineTo(0, h * 0.72); g.lineTo(w * 0.25, h * 0.55); g.lineTo(w * 0.45, h * 0.75); g.lineTo(w * 0.7, h * 0.6); g.lineTo(w, h * 0.78); g.lineTo(w, h); g.fill();
  });
  const GB = 2 ** 30;
  const sys = sc.potato
    ? { ram: 8 * GB, gpus: [], bestVram: 0, cpuThreads: 4, diskFree: 61 * GB, gpuChecked: true }
    : { ram: 32 * GB, gpus: [{ name: "NVIDIA GeForce RTX 4070", vram: 16311 * 2 ** 20 }], bestVram: 16311 * 2 ** 20, cpuThreads: 16, diskFree: 412 * GB, gpuChecked: true };
  const fit = (size) => {
    const need = size + GB, ram = Math.max(0, sys.ram - 3 * GB);
    if (sys.bestVram >= need) return { level: "great", label: "Runs great on your graphics card" };
    if (sys.bestVram + ram >= need && size <= 3 * GB) return { level: "ok", label: "Runs well on this PC" };
    if (sys.bestVram + ram >= need) return { level: "slow", label: "Runs, but slowly on this PC" };
    return { level: "toobig", label: "Too big for this PC" };
  };
  const cat = [
    ["qwen3.5-0.8b", "Qwen 3.5 Mini", "Alibaba", "0.8B", "Tiny and quick. Runs on almost any PC, even without a graphics card.", 532517120, ["Fastest", "Can think", "Sees images"]],
    ["qwen3.5-2b", "Qwen 3.5 Small", "Alibaba", "2B", "Small and fast. A good pick for laptops and older PCs.", 1280835840, ["Fast", "Can think", "Sees images"]],
    ["qwen3.5-4b", "Qwen 3.5", "Alibaba", "4B", "The best all-rounder for most PCs. Start here if you're not sure.", 2740937888, ["Recommended", "Can think", "Sees images"]],
    ["gemma-4-e4b", "Gemma 4", "Google", "E4B", "Google's efficient model. Strong at writing and explaining things clearly.", 4977171584, ["Writing", "Sees images"]],
    ["qwen3.5-9b", "Qwen 3.5 Plus", "Alibaba", "9B", "Noticeably smarter. Best with a graphics card that has 8 GB or more.", 5680522464, ["Smart", "Can think", "Sees images"]],
    ["gemma-4-12b", "Gemma 4 Large", "Google", "12B", "Thoughtful, well-written answers. Wants a graphics card with 10 GB or more.", 7121861440, ["Smart", "Writing", "Sees images"]],
    ["gpt-oss-20b", "gpt-oss", "OpenAI", "20B", "OpenAI's open model. Always reasons step by step before it answers.", 11624759488, ["Smart", "Always thinks"]],
    ["qwen3-coder-30b", "Qwen 3 Coder", "Alibaba", "30B", "Built for writing and fixing code. Quick for its size because only part of it runs at once.", 17665334432, ["Coding"]],
    ["qwen3.8-27b", "Qwen 3.8", "Alibaba", "27B", "Very capable for hard questions. Needs a high-end graphics card or lots of memory.", 16464440224, ["Smartest", "Can think", "Sees images"]],
    ["qwen3-coder-next", "Qwen 3 Coder Next", "Alibaba", "80B", "The best open coding model that can run at home. Writes, fixes and explains code across whole projects. Needs about 40 GB of memory (graphics card plus system memory).", 38429272064, ["Best for coding", "Big"]],
    ["sdxl", "Stable Diffusion XL", "Stability AI", "3.5B", "Makes pictures from a description: photos, art, logos, anything. Can also redo a picture you add.", 6938078334, ["Makes pictures", "Big"], "image", [334641162]],
    ["wan2.2-5b", "Wan 2.2 Video", "Alibaba", "5B", "Makes short videos from a description, or brings a picture you add to life. Takes a few minutes per video.", 3433116000, ["Makes videos", "Big"], "video", [3655145312, 1409400960]],
    ["nemotron-asr", "Speech recognition", "NVIDIA", "0.6B", "Lets Phoenix understand what you say and what's in audio files. Works in 40+ languages.", 741548352, ["Understands speech"], "listen"],
    ["nemotron-diarization", "Speaker detection", "NVIDIA", "0.1B", "Tells apart who is talking in audio files, up to 8 people, so transcripts say who said what.", 107012128, ["Who's talking"], "speakers"],
    ["kokoro", "Kokoro voice", "hexgrad", "82M", "A natural-sounding voice, so Phoenix can read answers aloud and talk back to you.", 132353410, ["Talks back"], "voice"],
  ];
  const installedIds = sc.noModels ? [] : sc.full ? ["qwen3.5-0.8b", "qwen3.5-2b", "qwen3.5-4b", "gemma-4-e4b", "gpt-oss-20b"] : ["qwen3.5-4b", "gpt-oss-20b", "sdxl", "wan2.2-5b", "nemotron-asr", "kokoro"];
  const fitKind = (kind, size) => {
    if (kind === "listen" || kind === "speakers" || kind === "voice") return { level: "great", label: "Runs on your processor" };
    const f = fit(size);
    if (kind === "image" || kind === "video") return f.level === "great" ? { level: "great", label: "Runs well on your graphics card" } : f.level === "toobig" ? f : { level: "slow", label: "Works, but takes a while on this PC" };
    return f;
  };
  const catalog = () => cat.map(([id, name, maker, params, description, size, tags, kind, parts]) => {
    const totalSize = size + (parts || []).reduce((a, b) => a + b, 0);
    return {
      id, name, maker, params, description, size, tags, repo: "unsloth/" + id, file: id + ".gguf", license: "Apache 2.0", kind: kind || "chat",
      parts: (parts || []).map((sz, i) => ({ role: "part", name: "Part " + (i + 1), size: sz })), totalSize,
      installed: installedIds.includes(id), installedId: installedIds.includes(id) ? "unsloth__" + id + "/" + id + ".gguf" : "", fit: fitKind(kind, totalSize),
    };
  });
  const seesImages = ["qwen3.5-0.8b", "qwen3.5-2b", "qwen3.5-4b", "gemma-4-e4b", "qwen3.5-9b", "gemma-4-12b", "qwen3.8-27b"];
  const visionOn = new Set(["qwen3.5-4b", "qwen3.5-0.8b", "qwen3.5-2b"]);
  const installed = () => catalog().filter((c) => c.installed).map((c) => ({
    id: c.installedId, name: c.name, maker: c.maker, params: c.params, description: c.description, repo: c.repo, file: c.file,
    quant: c.kind === "chat" ? "Q4_K_M" : "", size: c.totalSize, catalogId: c.id, fit: c.fit, kind: c.kind,
    ready: !(sc.unready && c.kind === "video"), missing: sc.unready && c.kind === "video" ? "Video decoder for Wan 2.2" : "",
    vision: visionOn.has(c.id), visionAvailable: seesImages.includes(c.id) && !visionOn.has(c.id), visionSize: 990372672,
  }));
  const voices = [["af_heart", "Heart", "American"], ["af_bella", "Bella", "American"], ["am_michael", "Michael", "American"], ["bf_emma", "Emma", "British"], ["bm_george", "George", "British"]].map(([id, name, accent]) => ({ id, name, accent }));
  const now = Date.now(), day = 86400000;
  const chats = sc.noModels ? [] : [
    { id: "a7", title: "Cozy cabin in the snow", updated: now - 300e3 },
    { id: "a0", title: "Turn my sketch into a web page", updated: now - 600e3 },
    { id: "a1", title: "Paper plugin for a parkour timer", updated: now - 3600e3 },
    { id: "a2", title: "Why is the sky orange at sunset?", updated: now - 5 * 3600e3, unread: true },
    { id: "a3", title: "Best way to learn Rust", updated: now - day - 1000 },
    { id: "a4", title: "Gradle build keeps failing on shadowJar", updated: now - 3 * day },
    { id: "a5", title: "Ideas for a birthday party", updated: now - 12 * day },
    { id: "a6", title: "Explain Vulkan vs CUDA", updated: now - 40 * day },
  ];
  if (sc.full) {
    chats.splice(4, 0,
      { id: "b1", title: "Best seeds for a speedrun practice world", updated: now - 4 * day },
      { id: "b2", title: "Rewrite my server rules to sound friendlier", updated: now - 5 * day },
      { id: "b3", title: "How does a pressure plate redstone clock work", updated: now - 6 * day },
      { id: "b4", title: "Good settings for recording gameplay", updated: now - 8 * day });
  }
  const viewing = { id: "" };
  let update = sc.update
    ? { current: "0.2.0", latest: "0.2.1", state: "ready", notes: "- Faster model loading\n- Fixed the chat list jumping when an answer finishes", received: 1, total: 1, error: "", checked: now - 120e3 }
    : { current: "0.2.0", latest: "0.2.0", state: "uptodate", notes: "", received: 0, total: 0, error: "", checked: now - 40 * 60e3 };
  const convo = {
    id: "a1", title: "Paper plugin for a parkour timer", created: now - 4000e3, updated: now - 3600e3,
    messages: [
      { role: "user", content: "How do I start a timer when a player steps on a gold pressure plate in a Paper plugin?", time: now - 3700e3 },
      {
        role: "assistant", modelName: "Qwen 3.5", tps: 61.4, time: now - 3650e3,
        thinking: "The user wants a Paper plugin. Listen for PlayerInteractEvent with Action.PHYSICAL on a LIGHT_WEIGHTED_PRESSURE_PLATE (gold). Store start time in a map keyed by UUID.",
        content: "Listen for `PlayerInteractEvent` with the **physical** action, then check the block type. Gold plates are `LIGHT_WEIGHTED_PRESSURE_PLATE`.\n\n```java\n@EventHandler\npublic void onPlate(PlayerInteractEvent e) {\n    if (e.getAction() != Action.PHYSICAL) return;\n    Block b = e.getClickedBlock();\n    if (b == null || b.getType() != Material.LIGHT_WEIGHTED_PRESSURE_PLATE) return;\n    starts.putIfAbsent(e.getPlayer().getUniqueId(), System.nanoTime());\n    e.getPlayer().sendActionBar(Component.text(\"Timer started!\"));\n}\n```\n\nA few things to keep in mind:\n\n1. Use `putIfAbsent` so standing on the plate doesn't restart the timer every tick.\n2. Clear the entry when the player quits, or the map grows forever.\n3. Use `System.nanoTime()` for timing, not the system clock.",
      },
      { role: "user", content: "Nice. How do I show the time on the scoreboard while they run?", time: now - 3620e3 },
      {
        role: "assistant", modelName: "Qwen 3.5", tps: 58.9, time: now - 3600e3,
        content: "Run a repeating task every 2 ticks and update an action bar or sidebar line for everyone who has a running timer:\n\n```java\nBukkit.getScheduler().runTaskTimer(this, () -> {\n    starts.forEach((id, t0) -> {\n        Player p = Bukkit.getPlayer(id);\n        if (p == null) return;\n        double secs = (System.nanoTime() - t0) / 1e9;\n        p.sendActionBar(Component.text(String.format(\"%.2f s\", secs)));\n    });\n}, 0L, 2L);\n```\n\nThe action bar is the simplest option. A sidebar scoreboard works too, but you'd create one `Scoreboard` per player.",
      },
    ],
  };
  const sketchId = newId(".jpg");
  files[sketchId] = sketch();
  const notesId = newId(".txt");
  files[notesId] = "Sunset page notes\n\n- Big sun in the middle, slowly glowing\n- Mountains along the bottom\n- A short line of text that fades in\n- Should work on phones too\n";
  const pageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Sunset</title>
<style>
  html, body { margin: 0; height: 100%; }
  body {
    display: grid; place-items: center; overflow: hidden;
    background: linear-gradient(#2a1530, #e8492e 62%, #f5b23a);
    font-family: Georgia, serif; color: #fff3d6;
  }
  .sun {
    position: absolute; top: 42%; left: 62%;
    width: 220px; height: 220px; margin: -110px;
    border-radius: 50%; background: #fff3d6;
    box-shadow: 0 0 80px 30px rgba(255, 220, 160, .55);
    animation: glow 4s ease-in-out infinite alternate;
  }
  @keyframes glow { to { box-shadow: 0 0 140px 60px rgba(255, 220, 160, .7); } }
  svg { position: absolute; bottom: 0; width: 100%; height: 42%; }
  h1 { position: relative; align-self: start; margin-top: 9vh; font-size: 44px; font-weight: normal; letter-spacing: .02em; animation: fade 2s ease-out both; }
  @keyframes fade { from { opacity: 0; transform: translateY(12px); } }
</style>
</head>
<body>
  <div class="sun"></div>
  <svg viewBox="0 0 100 40" preserveAspectRatio="none"><path d="M0 40V22L25 6l20 20L70 12l30 24v4z" fill="#1d1012"/></svg>
  <h1>See you at sunset</h1>
</body>
</html>`;
  const sketchConvo = {
    id: "a0", title: "Turn my sketch into a web page", created: now - 700e3, updated: now - 600e3,
    messages: [
      { role: "user", content: "Can you turn this sketch into a web page? My notes are attached too.", time: now - 700e3,
        attachments: [
          { id: sketchId, name: "sunset-sketch.png", kind: "image", size: 812004, width: 1280, height: 853 },
          { id: notesId, name: "page-notes.md", kind: "text", size: 141, chars: 141 },
        ] },
      { role: "assistant", modelName: "Qwen 3.5", tps: 58.2, time: now - 600e3,
        content: "Here's a page that matches your sketch: a glowing sun, dark mountains along the bottom, and a line of text that fades in.\n\n```html\n" + pageHTML + "\n```\n\nSave it as `index.html` and open it in your browser. The sun's glow is the `glow` animation, so change `4s` to make it pulse faster or slower." },
    ],
  };
  const cabinId = newId(".png");
  files[cabinId] = drawPicture(1024, 1024, (g, w, h) => {
    const sky = g.createLinearGradient(0, 0, 0, h);
    sky.addColorStop(0, "#1b2340"); sky.addColorStop(0.55, "#d9654b"); sky.addColorStop(1, "#f4c27a");
    g.fillStyle = sky; g.fillRect(0, 0, w, h);
    g.fillStyle = "#eef2f7";
    g.beginPath(); g.moveTo(0, h * 0.62); g.lineTo(w * 0.3, h * 0.35); g.lineTo(w * 0.55, h * 0.6); g.lineTo(w * 0.78, h * 0.4); g.lineTo(w, h * 0.6); g.lineTo(w, h); g.lineTo(0, h); g.fill();
    g.fillStyle = "#f7f9fc"; g.fillRect(0, h * 0.78, w, h * 0.22);
    g.fillStyle = "#5a3424"; g.fillRect(w * 0.36, h * 0.6, w * 0.28, h * 0.2);
    g.fillStyle = "#3a2018"; g.beginPath(); g.moveTo(w * 0.33, h * 0.61); g.lineTo(w * 0.5, h * 0.48); g.lineTo(w * 0.67, h * 0.61); g.fill();
    g.fillStyle = "#ffcf6e"; g.fillRect(w * 0.41, h * 0.66, w * 0.06, h * 0.06); g.fillRect(w * 0.53, h * 0.66, w * 0.06, h * 0.06);
  });
  const cabinConvo = {
    id: "a7", title: "Cozy cabin in the snow", created: now - 360e3, updated: now - 300e3,
    messages: [
      { role: "user", content: "A cozy wooden cabin in snowy mountains at sunset, warm light in the windows, oil painting", time: now - 360e3 },
      { role: "assistant", modelName: "Stable Diffusion XL", time: now - 300e3, content: "", media: [{ id: cabinId, kind: "image", width: 1024, height: 1024, seed: 42, shape: "square", seconds: 38.4 }] },
    ],
  };
  let engine = sc.noEngine ? { installed: false, version: "", busy: false, stage: "", received: 0, total: 0, error: "" } : { installed: true, version: "b11669", busy: false };
  const downloads = sc.downloading ? [
    { id: "unsloth__qwen3.5-9b/qwen3.5-9b.gguf", catalogId: "qwen3.5-9b", name: "Qwen 3.5 Plus", repo: "unsloth/Qwen3.5-9B-GGUF", file: "x", received: 2.1 * GB, total: 5680522464, speed: 38 * 2 ** 20, status: "downloading" },
  ] : [];
  const later = (ms, fn) => setTimeout(fn, ms);
  let settings = { useGPU: true, contextSize: 8192, systemPrompt: "", thinking: false, lastModel: installedIds.length ? "unsloth__qwen3.5-4b/qwen3.5-4b.gguf" : "", theme: sc.theme || "dark", autoUpdate: true, notifications: true };
  const P = (v) => Promise.resolve(v);
  window.go = { backend: { App: {
    GetAppInfo: () => P({ version: "0.3.0", dataDir: "C:\\Users\\you\\AppData\\Local\\PhoenixAI", platform: "windows", maxChats: 10, maxModels: 5, maxFiles: 10 }),
    GetSettings: () => P(settings),
    SaveSettings: (s) => { settings = s; return P(s); },
    GetEngineStatus: () => P(engine),
    InstallEngine: () => {
      engine = { ...engine, busy: true, stage: "Downloading the engine", received: 0, total: 61 * 2 ** 20 };
      emit("engine:status", engine);
      let got = 0;
      const t = setInterval(() => {
        got += 9 * 2 ** 20;
        if (got >= engine.total) { clearInterval(t); engine = { installed: true, version: "b11669", busy: false }; emit("engine:status", engine); return; }
        engine = { ...engine, received: got }; emit("engine:status", engine);
      }, 300);
      return P();
    },
    CheckEngineUpdate: () => P({ current: "b11669", latest: "b11702", available: true }),
    GetSystemInfo: () => P(sys),
    GetCatalog: () => P(catalog()),
    GetInstalledModels: () => P(installed()),
    GetDownloads: () => P(downloads),
    DownloadModel: () => P(),
    DownloadFromHub: () => P(),
    CancelDownload: () => P(),
    DeleteModel: () => P(),
    SearchHub: () => P([
      { repo: "bartowski/Llama-3.2-3B-Instruct-GGUF", downloads: 812345, likes: 210, updated: "" },
      { repo: "unsloth/Llama-3.2-3B-Instruct-GGUF", downloads: 402118, likes: 90, updated: "" },
      { repo: "lmstudio-community/Llama-3.2-3B-Instruct-GGUF", downloads: 120044, likes: 40, updated: "" },
    ]),
    GetHubFiles: () => P([
      { file: "Llama-3.2-3B-Instruct-Q3_K_L.gguf", size: 1.82 * GB, quant: "Q3_K_L", fit: fit(1.82 * GB), installed: false, recommended: false },
      { file: "Llama-3.2-3B-Instruct-Q4_K_M.gguf", size: 2.02 * GB, quant: "Q4_K_M", fit: fit(2.02 * GB), installed: false, recommended: true, vision: false },
      { file: "Llama-3.2-3B-Instruct-Q6_K.gguf", size: 2.64 * GB, quant: "Q6_K", fit: fit(2.64 * GB), installed: false, recommended: false },
      { file: "Llama-3.2-3B-Instruct-f16.gguf", size: 6.43 * GB, quant: "F16", fit: fit(6.43 * GB), installed: false, recommended: false },
    ]),
    ListChats: () => P(chats.map((c) => ({ ...c, unread: !!c.unread && c.id !== viewing.id }))),
    GetChat: (id) => P(id === "a7" ? cabinConvo : id === "a0" ? sketchConvo : id === "a1" ? convo : { id, title: (chats.find((c) => c.id === id) || {}).title || "x", messages: [{ role: "user", content: "Hi!" }, { role: "assistant", content: "Hello! What can I help with?", modelName: "Qwen 3.5" }] }),
    SetViewingChat: (id) => {
      viewing.id = id;
      const c = chats.find((x) => x.id === id);
      if (c && c.unread) { c.unread = false; later(5, () => emit("chats:changed")); }
      return P();
    },
    DeleteChat: (id) => { const i = chats.findIndex((c) => c.id === id); if (i >= 0) chats.splice(i, 1); later(5, () => emit("chats:changed")); return P(); },
    RenameChat: (id, title) => { const c = chats.find((x) => x.id === id); if (c) c.title = title; return P(); },
    GetUpdateStatus: () => P(update),
    CheckForUpdates: () => {
      update = { ...update, state: "checking" }; emit("update:status", update);
      return new Promise((res) => later(700, () => { update = { ...update, state: "uptodate", checked: Date.now() }; emit("update:status", update); res(update); }));
    },
    InstallUpdate: () => { update = { ...update, state: "installing" }; emit("update:status", update); return P(); },
    GetModelStatus: () => P(sc.noModels ? { state: "idle" } : { state: "ready", modelId: "unsloth__qwen3.5-4b/qwen3.5-4b.gguf", name: "Qwen 3.5", onGpu: true, note: "" }),
    LoadModel: () => P(),
    UnloadModel: () => P(),
    IsGenerating: () => P(""),
    StopGeneration: () => P(),
    OpenDataFolder: () => P(),
    Regenerate: () => P(),
    SetWindowActive: () => P(),
    GetSpeechStatus: () => P({ listen: !sc.noVoice, speakers: false, voice: !sc.noVoice, listenMb: 707, voiceMb: 126 }),
    GetVoices: () => P(voices),
    GetVoiceReady: () => P(),
    Speak: () => P(""),
    ForgetSpeech: () => P(),
    Transcribe: () => new Promise((res) => later(600, () => res("What's a good name for a pet dragon?"))),
    TranscribeAttachment: (id) => new Promise((res) => later(900, () => {
      files[id] = "[0:00] Speaker 1: Okay, let's start the meeting.\n\n[0:04] Speaker 2: Sounds good.";
      res({ id, name: "team-meeting.mp3", kind: "audio", audio: "x.mp3", size: 3.2e6, chars: 74, duration: 204, speakers: 2 });
    })),
    SaveMedia: () => P("C:\\Users\\you\\Pictures\\Phoenix 2026-11-11 19.02.10.png"),
    SendVoiceMessage: (chatId, text, model) => window.go.backend.App.SendMessage(chatId, text, model, false, []),
    MakeMedia: (chatId, prompt, model, opts) => {
      const id = chatId || "m" + Date.now();
      if (!chatId) chats.unshift({ id, title: prompt.slice(0, 40), updated: Date.now() });
      const video = /wan/.test(model);
      later(10, () => emit("chat:start", { chatId: id, modelId: model, modelName: video ? "Wan 2.2 Video" : "Stable Diffusion XL", kind: video ? "video" : "image" }));
      let t = 50;
      const total = 20;
      later(t, () => emit("media:progress", { chatId: id, stage: "loading", label: "Loading Stable Diffusion XL. The first time takes a minute or two." }));
      for (let i = 1; i <= total; i++) {
        later((t += sc.slow ? 700 : 90), () => emit("media:progress", { chatId: id, stage: "sampling", label: video ? "Making your video" : "Making your picture", completed: i, total, elapsedMs: i * 1900,
          preview: i % 2 ? files[cabinId] : undefined }));
      }
      later(t + 100, () => {
        emit("chat:done", { chatId: id, message: { role: "assistant", content: "", modelName: video ? "Wan 2.2 Video" : "Stable Diffusion XL", media: [{ id: cabinId, kind: "image", width: 1024, height: 1024, seed: 7, shape: opts.shape || "square", seconds: 41.2 }] } });
        emit("chats:changed");
      });
      later(5, () => emit("chats:changed"));
      return P(id);
    },
    AddImageSupport: () => P(),
    PickFiles: () => P(["C:\\Users\\you\\Documents\\Lab report.pdf", "C:\\Users\\you\\Projects\\site\\script.js"]),
    AddAttachmentFromPath: (path) => new Promise((res) => later(500, () => {
      const name = path.split("\\").pop();
      if (name.endsWith(".pdf")) {
        const id = newId(".txt");
        files[id] = "[Page 1]\nLab report: how plants grow toward light\n\n[Page 2]\nMethods: three bean plants, one lamp, two weeks.";
        return res({ id, name, kind: "pdf", size: 1824400, chars: files[id].length, pages: 12 });
      }
      const id = newId(".txt");
      files[id] = "const btn = document.querySelector('button');\nbtn.addEventListener('click', () => alert('Hi!'));\n";
      res({ id, name, kind: "text", size: 2400, chars: 2391 });
    })),
    AddAttachmentData: (name, data) => {
      const id = newId(data.startsWith("data:image") ? ".jpg" : ".txt");
      files[id] = data;
      if (/\.(mp3|wav|m4a)$/i.test(name)) return P({ id, name, kind: "audio", audio: id, size: 3.2e6, transcribing: false, duration: 204, speakers: 2, chars: 74 });
      return P(data.startsWith("data:image")
        ? { id, name, kind: "image", size: data.length * 0.75, width: 1280, height: 720 }
        : { id, name, kind: "text", size: data.length * 0.75, chars: Math.round(data.length * 0.75) });
    },
    RemovePendingAttachment: () => P(),
    SendMessage: (chatId, text, model, think, attachmentIds) => {
      const id = chatId || "new1";
      later(10, () => emit("chat:start", { chatId: id, modelId: model, modelName: "Qwen 3.5" }));
      const thoughts = think ? ["The user is asking ", "about ", "something. ", "Let me think step by step."] : [];
      const reply = /page|html|site/i.test(text)
        ? "Here you go:\n\n```html\n<!doctype html>\n<html>\n<body style=\"margin:0;display:grid;place-items:center;height:100vh;background:#1d1012;color:#ff6a4d;font:48px Georgia\">\n  <h1>Hello from Phoenix</h1>\n</body>\n</html>\n```\n\nSwitch between **Preview** and **Code** above."
        : "Sure! Here's a **short** answer to that. Local models run entirely on your graphics card, so nothing you type leaves this PC.";
      const words = reply.split(" ");
      let t = 30;
      thoughts.forEach((w) => later((t += 60), () => emit("chat:delta", { chatId: id, thinking: w })));
      words.forEach((w, i) => later((t += sc.slow ? 400 : 45), () => emit("chat:delta", { chatId: id, content: (i ? " " : "") + w })));
      if (!sc.slow) later(t + 50, () => {
        const c = chats.find((x) => x.id === id);
        if (c && viewing.id !== id) c.unread = true;
        emit("chat:done", { chatId: id, message: { role: "assistant", content: words.join(" "), thinking: thoughts.join(""), modelName: "Qwen 3.5", tps: 63.2 } });
        emit("chats:changed");
      });
      if (!chatId) chats.unshift({ id, title: text.slice(0, 40) || "New chat", updated: Date.now() });
      later(5, () => emit("chats:changed"));
      return P(id);
    },
  } } };
})();
