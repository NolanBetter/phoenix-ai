"use strict";

/* Phoenix AI window. Talks to the Go side through window.go.backend.App
   and listens for events through window.runtime.EventsOn. */

const $ = (sel, root = document) => root.querySelector(sel);
const $$ = (sel, root = document) => [...root.querySelectorAll(sel)];
const api = () => window.go.backend.App;

// ---------- icons ----------

let gradientSeq = 0;
function phoenixSVG() {
  const id = "pg" + ++gradientSeq;
  return `<svg viewBox="4 4 56 56" aria-hidden="true"><defs><linearGradient id="${id}" x1="0" y1="1" x2="0" y2="0">
<stop offset="0" stop-color="#E8492E"/><stop offset=".55" stop-color="#F27A26"/><stop offset="1" stop-color="#F5B23A"/></linearGradient></defs>
<g fill="url(#${id})"><path d="M32 57C27 49 24.2 41 24.8 32.5 25.4 24 28.6 16.6 33.4 10.2 35.2 8 38.4 7.4 40.6 8.8L45.6 10.6 40.4 12.6C39 15.4 39 20.6 39.8 27.4 41.2 38.4 37.4 48.4 32 57Z"/>
<path d="M29.8 55.6C18.6 52 10.4 43.4 8.8 31.6 8 25.6 8.8 19.4 11 13.8 13.2 22.8 18 29.6 24.4 34.2 25.2 41.6 27 48.8 29.8 55.6Z"/>
<path d="M34.6 55.6C45.4 52 53.6 43.4 55.2 31.6 56 25.6 55.2 19.4 53 13.8 50.8 22.8 46 29.6 40.2 34.4 39.8 42 37.6 49 34.6 55.6Z"/></g></svg>`;
}

const STROKE = (d, extra = "") =>
  `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${extra}<path d="${d}"/></svg>`;

const ICONS = {
  plus: () => STROKE("M12 5v14M5 12h14"),
  chevron: () => STROKE("M6 9l6 6 6-6"),
  chevronRight: () => STROKE("M9 6l6 6-6 6"),
  models: () => STROKE("M12 3l8 4.5v9L12 21l-8-4.5v-9L12 3zM4 7.5l8 4.5 8-4.5M12 12v9"),
  gear: () => STROKE("M4 7h9M17 7h3M4 17h3M11 17h9", '<circle cx="15" cy="7" r="2"/><circle cx="9" cy="17" r="2"/>'),
  send: () => STROKE("M12 19V5M5 12l7-7 7 7"),
  stop: () => `<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="7" y="7" width="10" height="10" rx="2" fill="currentColor"/></svg>`,
  copy: () => STROKE("M5 15V6a2 2 0 0 1 2-2h9", '<rect x="9" y="8" width="11" height="12" rx="2"/>'),
  retry: () => STROKE("M4 12a8 8 0 1 0 2.4-5.7M4 4v4.5h4.5"),
  trash: () => STROKE("M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3"),
  pencil: () => STROKE("M4 20h4L19.5 8.5a2.1 2.1 0 0 0-3-3L5 17v3z"),
  search: () => STROKE("M20 20l-4.2-4.2", '<circle cx="11" cy="11" r="7"/>'),
  folder: () => STROKE("M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z"),
  spark: () => STROKE("M12 3c.8 4.6 2.4 6.2 7 7-4.6.8-6.2 2.4-7 7-.8-4.6-2.4-6.2-7-7 4.6-.8 6.2-2.4 7-7z"),
  download: () => STROKE("M12 4v11M7 10l5 5 5-5M5 20h14"),
  check: () => STROKE("M5 12.5l4.5 4.5L19 7.5"),
  x: () => STROKE("M6 6l12 12M18 6L6 18"),
  more: () => `<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="5.5" cy="12" r="1.6" fill="currentColor"/><circle cx="12" cy="12" r="1.6" fill="currentColor"/><circle cx="18.5" cy="12" r="1.6" fill="currentColor"/></svg>`,
  update: () => STROKE("M12 16V8M8.5 11.5L12 8l3.5 3.5", '<circle cx="12" cy="12" r="9"/>'),
  attach: () => STROKE("M20 11.5l-8.3 8.3a5 5 0 0 1-7.1-7.1l8.6-8.6a3.4 3.4 0 0 1 4.8 4.8l-8.6 8.6a1.7 1.7 0 0 1-2.4-2.4l7.9-7.9"),
  expand: () => STROKE("M14 4h6v6M10 20H4v-6M20 4l-6.5 6.5M4 20l6.5-6.5"),
  image: () => STROKE("M4 17l4.5-4.5 3.5 3.5 2.5-2.5L20 19", '<rect x="3" y="4" width="18" height="16" rx="2"/><circle cx="9" cy="9" r="1.6"/>'),
  film: () => STROKE("M7 4v16M17 4v16M3 9h4M3 15h4M17 9h4M17 15h4", '<rect x="3" y="4" width="18" height="16" rx="2"/>'),
  mic: () => STROKE("M12 18v3M8 21h8M5 11a7 7 0 0 0 14 0", '<rect x="9" y="3" width="6" height="11" rx="3"/>'),
  volume: () => STROKE("M4 9h4l5-4v14l-5-4H4zM16.5 8.5a5 5 0 0 1 0 7M19 6a8.5 8.5 0 0 1 0 12"),
  waves: () => STROKE("M4 10v4M8 7v10M12 4v16M16 7v10M20 10v4"),
  save: () => STROKE("M12 4v10M7.5 9.5L12 14l4.5-4.5M5 16v3h14v-3"),
  audio: () => STROKE("M9 18V6l10-2v12", '<circle cx="6.5" cy="18" r="2.5"/><circle cx="16.5" cy="16" r="2.5"/>'),
  phoenix: phoenixSVG,
};

function icon(name) { return (ICONS[name] || ICONS.x)(); }
function hydrateIcons(root = document) {
  $$("[data-icon]", root).forEach((el) => { if (!el.firstElementChild) el.innerHTML = icon(el.dataset.icon); });
}

// ---------- helpers ----------

function h(tag, attrs = {}, ...children) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (v == null || v === false) continue;
    if (k === "class") el.className = v;
    else if (k.startsWith("on")) el.addEventListener(k.slice(2), v);
    else if (k === "html") el.innerHTML = v;
    else el.setAttribute(k, v === true ? "" : v);
  }
  for (const c of children.flat()) {
    if (c == null || c === false) continue;
    el.append(c instanceof Node ? c : document.createTextNode(String(c)));
  }
  return el;
}
const ic = (name) => h("span", { "data-icon": name, html: icon(name) });

// "Video decoder for Wan 2.2" → "video decoder for Wan 2.2"
const lowerFirst = (t) => (t ? t[0].toLowerCase() + t.slice(1) : "");

function fmtBytes(n) {
  if (!n || n < 0) return "0 MB";
  const gbv = n / 2 ** 30;
  if (gbv >= 1) return (gbv >= 10 ? gbv.toFixed(0) : gbv.toFixed(1)) + " GB";
  return Math.max(1, Math.round(n / 2 ** 20)) + " MB";
}
function fmtCount(n) {
  if (n >= 1e6) return (n / 1e6).toFixed(1).replace(/\.0$/, "") + "M";
  if (n >= 1e3) return (n / 1e3).toFixed(1).replace(/\.0$/, "") + "K";
  return String(n);
}
function fmtEta(remaining, speed) {
  if (!speed || speed < 1) return "";
  const s = remaining / speed;
  if (s < 60) return "less than a minute left";
  if (s < 3600) return Math.round(s / 60) + " min left";
  return (s / 3600).toFixed(1) + " hr left";
}
function errText(e) {
  const s = typeof e === "string" ? e : e && e.message ? e.message : String(e);
  return s.charAt(0).toUpperCase() + s.slice(1);
}

function toast(msg, kind = "error", ms = 5500, onClick = null) {
  const t = h(onClick ? "button" : "div", { class: "toast " + kind + (onClick ? " clickable" : ""), role: kind === "error" ? "alert" : "status" }, errText(msg));
  if (onClick) t.addEventListener("click", () => { t.remove(); onClick(); });
  $("#toasts").append(t);
  setTimeout(() => t.remove(), ms);
}

// A small popup menu (the ⋯ menu on chats, and right-click).
function openMenu(items, anchor, x, y) {
  closeMenu();
  const menu = h("div", { class: "menu", role: "menu", id: "ctxMenu" },
    items.map((it) => h("button", {
      class: "menu-item" + (it.danger ? " danger" : ""), role: "menuitem",
      onclick: (e) => { e.stopPropagation(); closeMenu(); it.onClick(); },
    }, ic(it.icon), it.label)));
  document.body.append(menu);
  const r = anchor ? anchor.getBoundingClientRect() : { left: x, right: x, top: y, bottom: y };
  const mw = menu.offsetWidth;
  const mh = menu.offsetHeight;
  let left = anchor ? r.left : x;
  let top = anchor ? r.bottom + 4 : y;
  if (left + mw > innerWidth - 8) left = innerWidth - mw - 8;
  if (top + mh > innerHeight - 8) top = (anchor ? r.top - 4 : y) - mh;
  menu.style.left = Math.max(8, left) + "px";
  menu.style.top = Math.max(8, top) + "px";
  if (anchor) anchor.setAttribute("aria-expanded", "true");
  menu._anchor = anchor;
  const buttons = $$(".menu-item", menu);
  buttons[0].focus();
  menu.addEventListener("keydown", (e) => {
    const i = buttons.indexOf(document.activeElement);
    if (e.key === "ArrowDown") { e.preventDefault(); buttons[(i + 1) % buttons.length].focus(); }
    if (e.key === "ArrowUp") { e.preventDefault(); buttons[(i - 1 + buttons.length) % buttons.length].focus(); }
    if (e.key === "Escape" || e.key === "Tab") { e.preventDefault(); closeMenu(true); }
  });
}
function closeMenu(refocus) {
  const m = $("#ctxMenu");
  if (!m) return;
  if (m._anchor) {
    m._anchor.setAttribute("aria-expanded", "false");
    if (refocus) m._anchor.focus();
  }
  m.remove();
}

async function call(name, ...args) {
  try {
    return await api()[name](...args);
  } catch (e) {
    toast(e);
    throw e;
  }
}

function confirmDialog({ title, body, ok = "Delete", danger = true }) {
  return new Promise((resolve) => {
    const d = $("#dialog");
    $("#dialogTitle").textContent = title;
    $("#dialogBody").textContent = body;
    $("#dialogOk").textContent = ok;
    $("#dialogOk").className = "btn " + (danger ? "btn-danger" : "btn-primary");
    d.hidden = false;
    $("#dialogCancel").focus();
    const done = (v) => {
      d.hidden = true;
      $("#dialogOk").onclick = $("#dialogCancel").onclick = d.onkeydown = d.onclick = null;
      resolve(v);
    };
    $("#dialogOk").onclick = () => done(true);
    $("#dialogCancel").onclick = () => done(false);
    d.onclick = (e) => { if (e.target === d) done(false); };
    d.onkeydown = (e) => { if (e.key === "Escape") done(false); };
  });
}

// ---------- state ----------

const state = {
  info: null,
  settings: null,
  sys: null,
  engine: null,
  catalog: [],
  installed: [],
  downloads: {},
  chats: [],
  view: "chat",
  chatId: null,
  messages: [],
  pendingNew: false,
  gen: null, // {chatId, content, thinking, modelName}
  modelStatus: { state: "idle" },
  selected: null,
  think: false,
  hub: { results: [], open: {}, files: {}, loading: false, query: "" },
  update: null,
  renaming: null,
  viewingSent: null,
  pending: [], // files added to the message box: {key, loading, ...Attachment}
  codeModes: new Map(), // "chat:msg:block" -> "preview" | "code"
  speech: { listen: false, speakers: false, voice: false, listenMb: 0, voiceMb: 0 },
  mediaOpts: { shape: "", length: 2 },
  voices: [],
};

// What a model does: chat, image, video, listen, speakers or voice.
const kindOf = (m) => (m && m.kind) || "chat";
const isMediaKind = (k) => k === "image" || k === "video";
const pickable = (m) => ["chat", "image", "video"].includes(kindOf(m));
const KIND_LABEL = { image: "Makes pictures", video: "Makes videos", listen: "Understands speech", speakers: "Who's talking", voice: "Talks back" };

// ---------- limits ----------

const maxChats = () => state.info?.maxChats || 10;
const maxModels = () => state.info?.maxModels || 5;
const chatsFull = () => state.chats.length >= maxChats();
const isVisionFile = (id) => /mmproj/i.test(id.split("/").pop());
// Chat, picture and video models count toward the limit; image support,
// parts, engines and the small voice helpers don't.
function modelSlotsUsed() {
  const counts = (kind) => ["chat", "image", "video"].includes(kind || "chat");
  const ids = new Set(state.installed.filter((m) => counts(m.kind)).map((m) => m.id));
  for (const d of Object.values(state.downloads)) {
    if (d.tool || d.part || isVisionFile(d.id) || d.status === "error") continue;
    const c = state.catalog.find((m) => m.id === d.catalogId);
    if (c ? counts(c.kind) : true) ids.add(d.id);
  }
  return ids.size;
}
const modelsFull = () => modelSlotsUsed() >= maxModels();
const fullModelsText = () => `You have ${maxModels()} models, the most Phoenix keeps. Delete one to download another.`;

// Tell the backend which chat is on screen, so answers that finish
// elsewhere get an unread dot. Nothing counts as seen while minimized.
function syncViewing() {
  const id = state.view === "chat" && document.visibilityState === "visible" ? state.chatId || "" : "";
  if (id === state.viewingSent) return;
  state.viewingSent = id;
  const c = id && state.chats.find((x) => x.id === id);
  if (c && c.unread) {
    c.unread = false;
    renderChatList();
  }
  api().SetViewingChat(id).catch(() => {});
}

// ---------- theme ----------

const darkQuery = matchMedia("(prefers-color-scheme: dark)");
function applyTheme() {
  const t = state.settings?.theme || "system";
  const dark = t === "dark" || (t === "system" && darkQuery.matches);
  document.documentElement.dataset.theme = dark ? "dark" : "light";
}
darkQuery.addEventListener("change", applyTheme);

// ---------- loading data ----------

async function refreshModels() {
  const [catalog, installed, speech] = await Promise.all([api().GetCatalog(), api().GetInstalledModels(), api().GetSpeechStatus()]);
  state.catalog = catalog || [];
  state.installed = installed || [];
  state.speech = speech || state.speech;
  const before = state.selected;
  const choices = state.installed.filter(pickable);
  if (!choices.some((m) => m.id === state.selected)) {
    state.selected = (choices.find((m) => kindOf(m) === "chat") || choices[0] || {}).id || null;
  }
  // First model just arrived: load it so the first answer is quick.
  if (state.selected && state.selected !== before && !state.gen && state.modelStatus.state !== "loading" && document.body.dataset.booted) {
    api().LoadModel(state.selected).catch(() => {});
  }
  renderPicker();
  renderStarter();
  if (state.view === "models") renderModelsPage();
  updateComposer();
}

async function refreshChats() {
  state.chats = (await api().ListChats()) || [];
  renderChatList();
}

async function refreshSystem() {
  state.sys = await api().GetSystemInfo();
  if (state.view === "models") renderPcSummary();
}

async function refreshDownloads() {
  const list = (await api().GetDownloads()) || [];
  state.downloads = {};
  for (const d of list) state.downloads[d.id] = d;
  renderDownloadBits();
}

// ---------- sidebar ----------

function groupLabel(ts) {
  const d = new Date(ts);
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const diff = (today - new Date(d.getFullYear(), d.getMonth(), d.getDate())) / 86400000;
  if (diff <= 0) return "Today";
  if (diff === 1) return "Yesterday";
  if (diff < 7) return "Previous 7 days";
  if (diff < 30) return "Previous 30 days";
  return "Older";
}

function renderChatList() {
  if (state.renaming) return; // don't wipe out the rename box; redrawn when it closes
  const list = $("#chatList");
  list.textContent = "";
  const count = $("#chatCount");
  count.textContent = `${state.chats.length}/${maxChats()}`;
  count.classList.toggle("full", chatsFull());
  $("#newChat").title = chatsFull()
    ? `You have ${maxChats()} chats, the most Phoenix keeps. Delete one to start a new chat.`
    : `${state.chats.length} of ${maxChats()} chats used`;
  if (!state.chats.length) {
    list.append(h("div", { class: "chat-empty" }, "Your chats will show up here."));
    return;
  }
  let last = "";
  for (const c of state.chats) {
    const g = groupLabel(c.updated);
    if (g !== last) {
      list.append(h("div", { class: "chat-group" }, g));
      last = g;
    }
    const active = c.id === state.chatId && state.view === "chat";
    const working = !!state.gen && state.gen.chatId === c.id;
    const unread = c.unread && !active && !working;
    let status = null;
    if (working) status = h("span", { class: "chat-state", title: "Writing an answer", role: "img", "aria-label": "Writing an answer" }, h("span", { class: "spinner" }));
    else if (unread) status = h("span", { class: "chat-state", title: "New answer", role: "img", "aria-label": "New answer" }, h("span", { class: "unread-dot" }));
    const item = h("div", { class: "chat-item" + (active ? " active" : "") + (unread ? " unread" : "") },
      h("button", { class: "open", title: c.title, onclick: () => openChat(c.id) }, c.title),
      status,
      h("button", {
        class: "icon-btn more", title: "Rename or delete", "aria-label": `Options for “${c.title}”`,
        "aria-haspopup": "menu", "aria-expanded": "false",
        onclick: (e) => { e.stopPropagation(); chatMenu(c, item, e.currentTarget); },
      }, ic("more")),
    );
    item.addEventListener("contextmenu", (e) => { e.preventDefault(); chatMenu(c, item, null, e.clientX, e.clientY); });
    list.append(item);
  }
}

function chatMenu(c, item, anchor, x, y) {
  openMenu([
    { label: "Rename", icon: "pencil", onClick: () => startRename(item, c) },
    { label: "Delete", icon: "trash", danger: true, onClick: () => deleteChat(c) },
  ], anchor, x, y);
}

function startRename(item, c) {
  const input = h("input", { value: c.title, "aria-label": "Chat name", maxlength: "80" });
  state.renaming = c.id;
  item.textContent = "";
  item.classList.add("renaming");
  item.append(input);
  input.focus();
  input.select();
  let finished = false;
  const finish = async (save) => {
    if (finished) return;
    finished = true;
    const v = input.value.trim();
    state.renaming = null;
    if (save && v && v !== c.title) {
      try { await call("RenameChat", c.id, v); } catch {}
    }
    refreshChats();
  };
  input.addEventListener("keydown", (e) => {
    if (e.key === "Enter") finish(true);
    if (e.key === "Escape") finish(false);
  });
  input.addEventListener("blur", () => finish(true));
}

async function deleteChat(c) {
  const ok = await confirmDialog({ title: "Delete this chat?", body: `“${c.title}” will be deleted from this PC. This can't be undone.` });
  if (!ok) return;
  await call("DeleteChat", c.id);
  if (state.chatId === c.id) newChat();
  refreshChats();
}

// ---------- views ----------

function setView(v) {
  state.view = v;
  $("#viewChat").hidden = v !== "chat";
  $("#viewModels").hidden = v !== "models";
  $("#viewSettings").hidden = v !== "settings";
  $$(".nav-item").forEach((b) => b.classList.toggle("active", b.dataset.view === v));
  if (v === "models") {
    renderModelsPage();
    refreshSystem();
  }
  if (v === "settings") renderSettings();
  if (v === "chat") setTimeout(() => $("#input").focus(), 0);
  closeMenu();
  syncViewing();
  renderChatList();
}

// ---------- chat ----------

function greetingText() {
  const hr = new Date().getHours();
  if (hr >= 5 && hr < 12) return "Good morning";
  if (hr >= 12 && hr < 17) return "Good afternoon";
  if (hr >= 17 && hr < 22) return "Good evening";
  return "Up late?";
}

function newChat() {
  closeViewer();
  state.chatId = null;
  state.messages = [];
  state.pendingNew = false;
  setView("chat");
  $("#greeting").textContent = greetingText();
  renderThread();
}

async function openChat(id) {
  try {
    const c = await api().GetChat(id);
    closeViewer();
    state.chatId = c.id;
    state.messages = c.messages || [];
    state.pendingNew = false;
    setView("chat");
    renderThread(true);
  } catch (e) {
    toast(e);
    refreshChats();
  }
}

function suggestedModel() {
  const fits = state.catalog.filter((m) => m.fit.level === "great" || m.fit.level === "ok");
  const rec = fits.find((m) => m.tags.includes("Recommended"));
  if (rec) return rec;
  const small = fits.filter((m) => m.size < 8 * 2 ** 30).sort((a, b) => b.size - a.size);
  if (small.length) return small[0];
  return [...state.catalog].sort((a, b) => a.size - b.size)[0];
}

const SUGGESTIONS = [
  ["Explain something", "Explain how a graphics card works, in simple terms."],
  ["Write an email", "Write a short, polite email asking to move our meeting to Friday."],
  ["Brainstorm", "Give me 10 fun ideas for a weekend coding project."],
  ["Help with code", "Help me find the bug in this code:\n\n"],
];

const MEDIA_SUGGESTIONS = {
  image: [
    ["Cozy cabin", "A cozy wooden cabin in snowy mountains at sunset, warm light in the windows, oil painting"],
    ["Product photo", "A glass bottle of orange soda on a wet rock, studio lighting, photo"],
    ["Game art", "A tiny pixel-art dragon guarding a pile of gold coins, 16-bit style"],
    ["Logo idea", "A simple flat logo of a phoenix made of flames, orange and red, white background"],
  ],
  video: [
    ["Ocean waves", "Slow waves rolling onto a sandy beach at golden hour, gentle camera pan"],
    ["City at night", "A rainy city street at night with neon signs reflecting in puddles, cars driving by"],
    ["Cute cat", "A fluffy orange cat stretching and yawning on a sunny windowsill"],
    ["Campfire", "A crackling campfire in a dark forest, sparks drifting up into the night sky"],
  ],
};

function renderStarter() {
  const box = $("#starter");
  box.textContent = "";
  if (!state.installed.filter(pickable).length) {
    $("#subline").textContent = "Everything you ask stays on this PC.";
    const m = suggestedModel();
    if (!m) return;
    const dl = Object.values(state.downloads).find((d) => d.catalogId === m.id);
    const card = h("div", { class: "first-model" },
      h("h2", {}, "Download your first model"),
      h("p", {}, `Based on your PC, ${m.name} (${m.params}) is a good place to start. ${m.description}`),
    );
    if (dl && dl.status !== "error") {
      card.append(progressEl(dl));
    } else {
      card.append(h("div", { class: "actions" },
        h("button", { class: "btn btn-primary", onclick: () => downloadCatalog(m.id) }, ic("download"), `Download ${m.name} (${fmtBytes(m.size)})`),
        h("button", { class: "btn btn-quiet", onclick: () => setView("models") }, "See all models"),
      ));
    }
    box.append(card);
    return;
  }
  $("#subline").textContent = "Everything you ask stays on this PC.";
  if (!state.chatId && chatsFull()) {
    const oldest = state.chats[state.chats.length - 1];
    box.append(h("div", { class: "first-model limit" },
      h("h2", {}, `You've used all ${maxChats()} chats`),
      h("p", {}, `Phoenix keeps up to ${maxChats()} chats. Delete one to start a new chat. Your oldest is “${oldest.title}”.`),
      h("div", { class: "actions" },
        h("button", { class: "btn btn-primary", onclick: () => deleteChat(oldest) }, ic("trash"), "Delete oldest chat"),
        h("button", { class: "btn btn-quiet", onclick: () => openChat(oldest.id) }, "Open it first"),
      ),
    ));
    return;
  }
  const kind = kindOf(selectedModel());
  $("#subline").textContent = kind === "image" ? "Describe a picture and Phoenix will make it, right on this PC."
    : kind === "video" ? "Describe a short video, or add a picture to bring to life. Videos take a few minutes."
    : "Everything you ask stays on this PC.";
  box.append(h("div", { class: "suggestions" },
    (MEDIA_SUGGESTIONS[kind] || SUGGESTIONS).map(([label, text]) => h("button", { class: "suggestion", onclick: () => {
      const inp = $("#input");
      inp.value = text;
      autosize();
      inp.focus();
      inp.setSelectionRange(text.length, text.length);
    } }, label)),
  ));
}

function renderThread(scrollToEnd = false) {
  const thread = $("#thread");
  const live = state.gen && state.gen.chatId === state.chatId && state.chatId;
  const empty = !state.messages.length && !live;
  $("#empty").hidden = !empty;
  thread.hidden = empty;
  thread.textContent = "";
  if (empty) {
    $("#greeting").textContent = greetingText();
    renderStarter();
  } else {
    const lastAssistant = state.messages.map((m) => m.role).lastIndexOf("assistant");
    state.messages.forEach((m, i) => thread.append(messageEl(m, i === lastAssistant && !live && i === state.messages.length - 1, i)));
    if (live) thread.append(liveEl());
  }
  updateComposer();
  if (scrollToEnd) scrollEnd(true);
}

function markdownEl(text, codeKey = null) {
  const div = h("div", { class: "answer" });
  div.innerHTML = DOMPurify.sanitize(marked.parse(text || ""), { USE_PROFILES: { html: true } });
  enhanceCode(div, codeKey);
  return div;
}

// ---------- code blocks ----------

const LANG_NAMES = { js: "JavaScript", javascript: "JavaScript", ts: "TypeScript", typescript: "TypeScript", py: "Python", python: "Python",
  html: "HTML", htm: "HTML", css: "CSS", svg: "SVG", json: "JSON", java: "Java", go: "Go", rust: "Rust", rs: "Rust", cpp: "C++", c: "C",
  cs: "C#", csharp: "C#", sh: "Shell", bash: "Shell", powershell: "PowerShell", ps1: "PowerShell", sql: "SQL", xml: "XML", yaml: "YAML",
  yml: "YAML", md: "Markdown", markdown: "Markdown", jsx: "JSX", tsx: "TSX", php: "PHP", ruby: "Ruby", kotlin: "Kotlin", lua: "Lua" };

// What a finished code block can show as a live preview, if anything.
function previewKind(lang, src) {
  const l = lang.toLowerCase();
  const head = src.trimStart().slice(0, 400).toLowerCase();
  if (l === "svg" || ((l === "xml" || l === "") && /^(<\?xml[^>]*>\s*)?<svg[\s>]/.test(head))) return "svg";
  if (l === "html" || l === "htm" || l === "xhtml") return "html";
  if (l === "" && (head.startsWith("<!doctype html") || head.startsWith("<html"))) return "html";
  return null;
}

// Turns a code block into a page for the preview. If the page links to
// style.css or script.js and the same answer has CSS or JavaScript blocks,
// those are put into the page so it works on its own.
function previewDoc(kind, src, siblings) {
  if (kind === "svg") {
    return `<!doctype html><meta charset="utf-8"><style>html,body{margin:0;height:100%}body{display:grid;place-items:center;background:#fff}svg{max-width:100%;max-height:100vh;height:auto}</style>${src}`;
  }
  const css = siblings.filter((b) => b.lang === "css").map((b) => b.src).join("\n");
  const js = siblings.filter((b) => b.lang === "js" || b.lang === "javascript").map((b) => b.src).join("\n").replace(/<\/script/gi, "<\\/script");
  const local = (url) => url && !/^(https?:)?\/\//i.test(url) && !/^data:/i.test(url);
  let doc = src;
  if (css) {
    let used = false;
    doc = doc.replace(/<link\b[^>]*>/gi, (tag) => {
      const href = (tag.match(/\bhref\s*=\s*["']?([^"'\s>]+)/i) || [])[1];
      if (!/\bstylesheet\b/i.test(tag) || !local(href) || !/\.css$/i.test(href)) return tag;
      if (used) return "";
      used = true;
      return `<style>\n${css}\n</style>`;
    });
  }
  if (js) {
    let used = false;
    doc = doc.replace(/<script\b[^>]*\bsrc\s*=\s*["']?([^"'\s>]+)[^>]*>\s*<\/script>/gi, (tag, src) => {
      if (!local(src) || !/\.m?js$/i.test(src)) return tag;
      if (used) return "";
      used = true;
      const mod = /type\s*=\s*["']?module/i.test(tag) ? ' type="module"' : "";
      return `<script${mod}>\n${js}\n</script>`;
    });
  }
  return doc;
}

const PREVIEW_SANDBOX = "allow-scripts allow-forms allow-modals allow-pointer-lock";
function previewFrame(doc, title) {
  // Sandboxed: the page runs in its own locked box and can't reach Phoenix,
  // your files or the chat.
  const f = h("iframe", { class: "preview-frame", sandbox: PREVIEW_SANDBOX, referrerpolicy: "no-referrer", title });
  f.srcdoc = doc;
  return f;
}

// Previews load when they scroll into view, so a chat full of them stays fast.
const previewObserver = "IntersectionObserver" in window ? new IntersectionObserver((entries) => {
  for (const e of entries) {
    if (!e.isIntersecting) continue;
    previewObserver.unobserve(e.target);
    e.target._load && e.target._load();
  }
}, { rootMargin: "300px" }) : null;

function segmented(options, value, onPick, label) {
  const g = h("div", { class: "seg-mini", role: "group", "aria-label": label });
  for (const [v, text] of options) {
    g.append(h("button", { type: "button", "aria-pressed": String(v === value), onclick: (e) => {
      $$("button", g).forEach((b) => b.setAttribute("aria-pressed", String(b === e.currentTarget)));
      onPick(v);
    } }, text));
  }
  return g;
}

// enhanceCode adds the header to each code block: the language, Copy, and
// for finished HTML or SVG, a Preview/Code switch and a full-screen button.
// codeKey is null while the answer is still being written.
function enhanceCode(root, codeKey) {
  const blocks = $$("pre > code", root).map((code) => {
    const lang = ([...code.classList].find((c) => c.startsWith("language-")) || "").slice(9).toLowerCase();
    return { code, pre: code.parentElement, lang, src: code.textContent };
  });
  blocks.forEach((b, i) => {
    if (b.pre.parentElement.classList.contains("code-block")) return;
    const kind = codeKey ? previewKind(b.lang, b.src) : null;
    const label = LANG_NAMES[b.lang] || b.lang || "Code";
    const wrap = h("div", { class: "code-block" });
    b.pre.replaceWith(wrap);
    const copy = h("button", { type: "button", class: "copy-code", title: "Copy code" }, ic("copy"), h("span", {}, "Copy"));
    copy.addEventListener("click", () => copyText(b.src, copy.lastChild));
    const tools = h("div", { class: "code-tools" });
    const head = h("div", { class: "code-head" }, h("span", { class: "code-lang" }, label), tools);
    wrap.append(head, b.pre);

    if (kind) {
      const key = codeKey + ":" + i;
      const doc = () => previewDoc(kind, b.src, blocks.filter((x) => x !== b));
      const pane = h("div", { class: "code-preview" });
      pane._load = () => { if (!pane.firstChild) pane.append(previewFrame(doc(), `Preview of the ${label} code`)); };
      const show = (mode) => {
        state.codeModes.set(key, mode);
        wrap.classList.toggle("previewing", mode === "preview");
        if (mode === "preview") previewObserver ? previewObserver.observe(pane) : pane._load();
      };
      const mode = state.codeModes.get(key) || "preview";
      tools.append(
        segmented([["preview", "Preview"], ["code", "Code"]], mode, show, "Show the preview or the code"),
        h("button", { type: "button", class: "code-icon", title: "Open full screen", "aria-label": "Open full screen",
          onclick: () => openCodeViewer({ label, src: b.src, doc: doc(), mode: state.codeModes.get(key) || "preview" }) }, ic("expand")),
      );
      wrap.append(pane);
      show(mode);
    }
    tools.append(copy);
  });
}

async function copyText(text, btn) {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    try { await window.runtime.ClipboardSetText(text); } catch { return toast("Couldn't copy to the clipboard."); }
  }
  if (btn) {
    const old = btn.innerHTML;
    btn.textContent = "Copied";
    setTimeout(() => (btn.innerHTML = old), 1200);
  }
  return true;
}

function thinkingEl(text, live) {
  const d = h("details", { class: "thinking" + (live ? " live" : "") },
    h("summary", {}, ic("chevronRight"), live ? "Thinking…" : "Thought process"),
    h("div", { class: "thought" }, text),
  );
  if (live) d.open = true;
  return d;
}

function messageEl(m, isLast, index) {
  if (m.role === "user") {
    const wrap = h("div", { class: "msg user" });
    if (m.attachments && m.attachments.length) wrap.append(sentFilesEl(m));
    if (m.content) wrap.append(h("div", { class: "bubble" }, m.content));
    return wrap;
  }
  const wrap = h("div", { class: "msg assistant" });
  if (m.media && m.media.length) return mediaMessageEl(m, isLast);
  if (m.thinking) wrap.append(thinkingEl(m.thinking, false));
  if (m.content) wrap.append(markdownEl(m.content, `${state.chatId}:${index}`));
  if (m.error) {
    wrap.append(h("div", { class: "msg-error" },
      h("p", {}, m.error),
      isLast ? h("button", { class: "btn", onclick: retry }, ic("retry"), "Try again") : null,
    ));
  }
  if (m.content || m.stopped) {
    const meta = h("div", { class: "msg-meta" },
      m.stopped ? h("span", { class: "stopped" }, "Stopped") : null,
      m.modelName ? h("span", { class: "who" }, m.modelName) : null,
      m.tps ? h("span", { class: "who", title: "How fast the model wrote this answer" }, `${Math.round(m.tps)} tokens/s`) : null,
      m.content ? h("button", { class: "icon-btn", title: "Copy", "aria-label": "Copy answer", onclick: (e) => copyText(m.content, null) && flashCheck(e.currentTarget) }, ic("copy")) : null,
      m.content ? readAloudBtn(m.content) : null,
      isLast ? h("button", { class: "icon-btn", title: "Write a new answer", "aria-label": "Retry", onclick: retry }, ic("retry")) : null,
    );
    wrap.append(meta);
  }
  return wrap;
}

function flashCheck(btn) {
  btn.innerHTML = "";
  btn.append(ic("check"));
  setTimeout(() => { btn.innerHTML = ""; btn.append(ic("copy")); }, 1200);
  return true;
}

function liveEl() {
  const g = state.gen;
  const wrap = h("div", { class: "msg assistant", id: "live" });
  fillLive(wrap, g);
  return wrap;
}

function fillLive(wrap, g) {
  if (isMediaKind(g.kind)) return fillMediaLive(wrap, g);
  const prev = $(".thinking", wrap);
  const prevOpen = prev ? prev.open : null;
  wrap.textContent = "";
  if (g.thinking) {
    const t = thinkingEl(g.thinking, !g.content);
    if (prevOpen !== null && !g.content) t.open = prevOpen;
    wrap.append(t);
    const thought = $(".thought", t);
    thought.scrollTop = thought.scrollHeight;
  }
  if (g.content) {
    const md = markdownEl(g.content);
    const last = md.lastElementChild || md;
    const host = last.classList.contains("code-block") ? md : last.tagName === "UL" || last.tagName === "OL" ? last.lastElementChild || last : last;
    host.append(h("span", { class: "caret", "aria-hidden": "true" }));
    wrap.append(md);
  } else if (!g.thinking) {
    const ms = state.modelStatus;
    if (ms.state === "loading") {
      wrap.append(h("div", { class: "status-line" }, h("span", { class: "spinner" }), `Loading ${ms.name || "the model"} into memory. The first time can take a minute.`));
    } else {
      wrap.append(h("div", { class: "status-line" }, h("span", { class: "caret" })));
    }
  }
}

let liveFrame = 0;
function scheduleLive() {
  if (liveFrame) return;
  liveFrame = requestAnimationFrame(() => {
    liveFrame = 0;
    const wrap = $("#live");
    if (!wrap || !state.gen) return;
    const stick = nearBottom();
    fillLive(wrap, state.gen);
    if (stick) scrollEnd();
  });
}

function nearBottom() {
  const s = $("#scroller");
  return s.scrollHeight - s.scrollTop - s.clientHeight < 120;
}
function scrollEnd(force) {
  const s = $("#scroller");
  if (force || nearBottom()) s.scrollTop = s.scrollHeight;
}

// ---------- pictures and videos ----------

const SHAPES = [["square", "Square"], ["wide", "Wide"], ["tall", "Tall"]];
const LENGTHS = [[2, "2 s"], [3, "3 s"], [5, "5 s"]];

function fmtSeconds(sec) {
  sec = Math.round(sec);
  return sec < 60 ? `${sec} s` : `${Math.floor(sec / 60)} min ${String(sec % 60).padStart(2, "0")} s`;
}

function mediaMessageEl(m, isLast) {
  const media = m.media[0];
  const url = attURL(state.chatId, media.id);
  const wrap = h("div", { class: "msg assistant media-msg" });
  const frame = h("div", { class: "media-frame " + media.kind, style: `aspect-ratio:${media.width} / ${media.height}` });
  if (media.kind === "video") {
    frame.append(h("video", { src: url, controls: true, loop: true, playsinline: true, preload: "metadata", "aria-label": "Video Phoenix made" }));
  } else {
    frame.append(h("button", { class: "media-open", title: "View full size", "aria-label": "View full size",
      onclick: () => openViewer(["Picture", h("small", {}, `${media.width}×${media.height}`)], [saveBtn(media)], h("div", { class: "viewer-image" }, h("img", { src: url, alt: "Picture Phoenix made" }))) },
      h("img", { src: url, alt: "Picture Phoenix made", loading: "lazy" })));
  }
  wrap.append(frame);
  const what = media.kind === "video" ? `${Math.round((media.frames - 1) / (media.fps || 24))} s video` : "Picture";
  wrap.append(h("div", { class: "msg-meta" },
    m.modelName ? h("span", { class: "who" }, m.modelName) : null,
    h("span", { class: "who" }, `${what}, made in ${fmtSeconds(media.seconds)}`),
    saveBtn(media, true),
    isLast ? h("button", { class: "btn btn-quiet media-again", onclick: retry }, ic("retry"), media.kind === "video" ? "Make another" : "Make another") : null,
  ));
  return wrap;
}

function saveBtn(media, small) {
  const b = h("button", { class: small ? "icon-btn" : "btn", title: "Save to your PC", "aria-label": "Save to your PC" }, ic("save"), small ? null : "Save");
  b.addEventListener("click", async () => {
    try {
      const where = await api().SaveMedia(state.chatId, media.id);
      if (where) toast(`Saved to ${where}`, "info", 4000);
    } catch (e) { toast(e); }
  });
  return b;
}

function fillMediaLive(wrap, g) {
  const p = g.progress || {};
  const video = g.kind === "video";
  wrap.textContent = "";
  wrap.classList.add("media-msg");
  const ms = state.modelStatus;
  let label = p.label || (ms.state === "loading" ? `Loading ${ms.name || "the model"}…` : "Getting started");
  const pct = p.total ? Math.min(100, (p.completed / p.total) * 100) : null;
  const shape = g.shape || (video ? "wide" : "square");
  const ratio = { square: "1 / 1", wide: video ? "832 / 480" : "1216 / 832", tall: video ? "480 / 832" : "832 / 1216" }[shape];
  const card = h("div", { class: "media-frame making", style: `aspect-ratio:${ratio}` });
  if (p.preview) card.append(h("img", { src: p.preview, alt: "", class: "preview" }));
  card.append(h("div", { class: "making-info" },
    h("div", { class: "making-label" }, h("span", { class: "spinner" }), label),
    pct !== null ? h("div", { class: "bar", role: "progressbar", "aria-valuemin": "0", "aria-valuemax": "100", "aria-valuenow": String(Math.round(pct)) }, h("span", { style: `width:${pct}%` })) : null,
    h("div", { class: "making-time" }, [p.total ? `Step ${p.completed} of ${p.total}` : null, p.elapsedMs ? fmtSeconds(p.elapsedMs / 1000) : null].filter(Boolean).join(", ")),
  ));
  wrap.append(card);
}

function renderMediaOpts() {
  const box = $("#mediaOpts");
  const kind = kindOf(selectedModel());
  box.hidden = !isMediaKind(kind);
  $("#thinkToggle").hidden = isMediaKind(kind);
  box.textContent = "";
  if (!isMediaKind(kind)) return;
  const shape = state.mediaOpts.shape || (kind === "video" ? "wide" : "square");
  box.append(segmented(SHAPES, shape, (v) => { state.mediaOpts.shape = v; }, "Shape"));
  if (kind === "video") box.append(segmented(LENGTHS, state.mediaOpts.length, (v) => { state.mediaOpts.length = v; }, "Length"));
}

// ---------- read aloud ----------

let speaking = null; // {audio, btn, url}
function stopSpeaking() {
  if (!speaking) return;
  speaking.audio.pause();
  if (speaking.btn) speaking.btn.classList.remove("on");
  api().ForgetSpeech(speaking.url).catch(() => {});
  speaking = null;
}

function readAloudBtn(text) {
  const b = h("button", { class: "icon-btn read-aloud", title: "Read aloud", "aria-label": "Read aloud" }, ic("volume"));
  b.addEventListener("click", async () => {
    if (speaking && speaking.btn === b) return stopSpeaking();
    stopSpeaking();
    if (!state.speech.voice) return offerVoice("read");
    b.classList.add("on", "busy");
    try {
      const url = await api().Speak(text);
      b.classList.remove("busy");
      if (!b.classList.contains("on")) return api().ForgetSpeech(url);
      const audio = new Audio(url);
      speaking = { audio, btn: b, url };
      audio.onended = () => { if (speaking && speaking.audio === audio) stopSpeaking(); };
      await audio.play();
    } catch (e) {
      b.classList.remove("on", "busy");
      toast(e);
    }
  });
  return b;
}

// Offer to download what voice features need.
async function offerVoice(why) {
  const sp = state.speech;
  const parts = [];
  if (why !== "read" && !sp.listen) parts.push(`speech recognition (${fmtBytes(sp.listenMb * 2 ** 20)})`);
  if (why !== "listen" && !sp.voice) parts.push(`the Kokoro voice (${fmtBytes(sp.voiceMb * 2 ** 20)})`);
  const title = why === "read" ? "Get a voice for Phoenix?" : why === "talk" ? "Get voice chat?" : "Get speech recognition?";
  const body = `Phoenix needs ${parts.join(" and ")} for this. They run on your processor and everything stays on this PC.`;
  const ok = await confirmDialog({ title, body, ok: "Download", danger: false });
  if (!ok) return;
  try {
    if (why === "read") await api().DownloadModel(state.catalog.find((m) => m.kind === "voice").id);
    else if (why === "listen") await api().DownloadModel(state.catalog.find((m) => m.kind === "listen").id);
    else await api().GetVoiceReady();
    toast("Downloading. You'll find it on the Models page.", "info", 4000);
    refreshDownloads();
  } catch (e) { toast(e); }
}

// ---------- audio: recording, decoding, voice chat ----------

// 16 kHz mono 16-bit WAV, as base64 (what the speech engine wants).
function wavBase64(samples, rate) {
  const buf = new ArrayBuffer(44 + samples.length * 2);
  const v = new DataView(buf);
  const str = (o, t) => [...t].forEach((c, i) => v.setUint8(o + i, c.charCodeAt(0)));
  str(0, "RIFF"); v.setUint32(4, 36 + samples.length * 2, true); str(8, "WAVEfmt ");
  v.setUint32(16, 16, true); v.setUint16(20, 1, true); v.setUint16(22, 1, true);
  v.setUint32(24, rate, true); v.setUint32(28, rate * 2, true); v.setUint16(32, 2, true); v.setUint16(34, 16, true);
  str(36, "data"); v.setUint32(40, samples.length * 2, true);
  for (let i = 0; i < samples.length; i++) {
    const x = Math.max(-1, Math.min(1, samples[i]));
    v.setInt16(44 + i * 2, x < 0 ? x * 0x8000 : x * 0x7fff, true);
  }
  return readAsDataURL(new Blob([buf], { type: "audio/wav" }));
}

// Any audio file the window can play (MP3, M4A, OGG, FLAC, WAV...) to 16 kHz WAV.
async function audioFileToWav(arrayBuffer) {
  const ctx = new OfflineAudioContext(1, 1, 16000);
  let buf;
  try { buf = await ctx.decodeAudioData(arrayBuffer); } catch { throw new Error("Phoenix couldn't play that audio file. Try MP3, WAV, M4A, OGG or FLAC."); }
  if (buf.duration > 30 * 60 + 5) throw new Error("Audio can be up to 30 minutes long.");
  const mono = new Float32Array(buf.length);
  for (let c = 0; c < buf.numberOfChannels; c++) {
    const d = buf.getChannelData(c);
    for (let i = 0; i < d.length; i++) mono[i] += d[i] / buf.numberOfChannels;
  }
  return wavBase64(mono, buf.sampleRate);
}

function to16k(chunks, rate) {
  const total = chunks.reduce((a, c) => a + c.length, 0);
  const all = new Float32Array(total);
  let o = 0;
  for (const c of chunks) { all.set(c, o); o += c.length; }
  if (rate === 16000) return all;
  const ratio = rate / 16000;
  const out = new Float32Array(Math.floor(total / ratio));
  for (let i = 0; i < out.length; i++) {
    // Average the samples that fall into this one (a simple low-pass).
    const a = Math.floor(i * ratio), b = Math.min(total, Math.floor((i + 1) * ratio));
    let sum = 0;
    for (let j = a; j < b; j++) sum += all[j];
    out[i] = sum / Math.max(1, b - a);
  }
  return out;
}

// openMic starts listening. onFrame(level, chunk) gets each ~85 ms of sound.
async function openMic(onFrame) {
  const stream = await navigator.mediaDevices.getUserMedia({ audio: { echoCancellation: true, noiseSuppression: true, autoGainControl: true, channelCount: 1 } });
  const ctx = new AudioContext();
  const src = ctx.createMediaStreamSource(stream);
  const node = ctx.createScriptProcessor(4096, 1, 1);
  node.onaudioprocess = (e) => {
    const d = new Float32Array(e.inputBuffer.getChannelData(0));
    let sum = 0;
    for (let i = 0; i < d.length; i++) sum += d[i] * d[i];
    onFrame(Math.sqrt(sum / d.length), d);
  };
  src.connect(node);
  node.connect(ctx.destination); // needed for it to run; it outputs silence
  return {
    rate: ctx.sampleRate,
    close() {
      node.onaudioprocess = null;
      try { src.disconnect(); node.disconnect(); } catch {}
      stream.getTracks().forEach((t) => t.stop());
      ctx.close().catch(() => {});
    },
  };
}

// A small voice detector: speech starts when the sound is well above the
// room's background noise, and ends after `quietMs` of quiet.
function makeVAD(quietMs) {
  let floor = 0.01, speaking = false, loud = 0, quiet = 0, frames = 0;
  return (level, frameMs) => {
    frames++;
    if (!speaking) floor = frames < 4 ? Math.max(floor, level) : floor * 0.95 + Math.min(level, floor * 2) * 0.05;
    const threshold = Math.max(0.015, floor * 2.5);
    if (level > threshold) { loud += frameMs; quiet = 0; } else { quiet += frameMs; loud = Math.max(0, loud - frameMs / 2); }
    if (!speaking && loud >= 200) { speaking = true; return "start"; }
    if (speaking && quiet >= quietMs) { speaking = false; return "end"; }
    return speaking ? "speaking" : "quiet";
  };
}

// ----- dictation: the microphone button -----

const dict = { mic: null, chunks: [], started: 0, timer: 0, heard: false };

async function toggleDictation() {
  if (dict.mic) return stopDictation(true);
  if (!state.speech.listen) return offerVoice("listen");
  const vad = makeVAD(2000);
  dict.chunks = [];
  dict.heard = false;
  try {
    dict.mic = await openMic((level, chunk) => {
      dict.chunks.push(chunk);
      const ev = vad(level, (chunk.length / (dict.mic ? dict.mic.rate : 48000)) * 1000);
      if (ev === "start") dict.heard = true;
      if (ev === "end" && dict.heard) stopDictation(true);
      $("#micBtn").style.setProperty("--level", Math.min(1, level * 12).toFixed(2));
      if (Date.now() - dict.started > 120000) stopDictation(true);
    });
  } catch {
    dict.mic = null;
    return toast("Phoenix couldn't use your microphone. Check that one is plugged in, and that Windows lets apps use it (Settings, Privacy, Microphone).");
  }
  dict.started = Date.now();
  const btn = $("#micBtn");
  btn.classList.add("rec");
  btn.title = "Stop and write it down";
  dict.timer = setInterval(() => { $("#micTime").textContent = fmtClock((Date.now() - dict.started) / 1000); }, 250);
}

function fmtClock(sec) {
  sec = Math.floor(sec);
  return `${Math.floor(sec / 60)}:${String(sec % 60).padStart(2, "0")}`;
}

async function stopDictation(keep) {
  const mic = dict.mic;
  if (!mic) return;
  dict.mic = null;
  mic.close();
  clearInterval(dict.timer);
  const btn = $("#micBtn");
  btn.classList.remove("rec");
  $("#micTime").textContent = "";
  btn.title = "Speak your message";
  if (!keep) return;
  btn.classList.add("busy");
  btn.disabled = true;
  try {
    const wav = await wavBase64(to16k(dict.chunks, mic.rate), 16000);
    const text = await api().Transcribe(wav);
    if (text) {
      const input = $("#input");
      input.value = (input.value.trim() ? input.value.trimEnd() + " " : "") + text;
      autosize();
      input.focus();
      input.setSelectionRange(input.value.length, input.value.length);
    } else {
      toast("Phoenix didn't hear anything. Try again a little closer to the microphone.", "info", 3500);
    }
  } catch (e) { toast(e); }
  btn.classList.remove("busy");
  btn.disabled = false;
}

// ----- voice chat -----

const vc = { on: false, phase: "", mic: null, chunks: [], vad: null, heard: false, queue: [], playing: false, audio: null, spoken: 0, done: false, muted: false, level: 0 };

function voicePhase(p, text) {
  vc.phase = p;
  $("#voice").dataset.phase = p;
  $("#voiceState").textContent = text;
  $("#voiceOrb").setAttribute("aria-label", p === "speaking" ? "Tap to interrupt" : p === "listening" ? "Listening" : "Working");
}

async function startVoiceChat() {
  const m = selectedModel();
  if (!m || kindOf(m) !== "chat") return toast("Pick a chat model to talk with.", "info", 3500);
  if (!state.speech.listen || !state.speech.voice) return offerVoice("talk");
  if (state.gen) return toast("Wait for the current answer to finish first.", "info", 3500);
  if (!state.chatId && chatsFull()) return toast(`You have ${maxChats()} chats already. Delete one, or open a chat to talk in it.`);
  stopSpeaking();
  if (dict.mic) stopDictation(false);
  vc.on = true;
  vc.muted = false;
  $("#voiceSaid").textContent = "";
  $("#voiceReply").textContent = "";
  $("#voiceModel").textContent = `Talking with ${m.name}`;
  $("#voiceMute").setAttribute("aria-pressed", "false");
  $("#voice").hidden = false;
  voicePhase("starting", "Getting ready…");
  try {
    vc.mic = await openMic(onVoiceFrame);
  } catch {
    endVoiceChat();
    return toast("Phoenix couldn't use your microphone. Check that one is plugged in, and that Windows lets apps use it (Settings, Privacy, Microphone).");
  }
  voiceListen();
}

function voiceListen() {
  if (!vc.on) return;
  vc.chunks = [];
  vc.heard = false;
  vc.vad = makeVAD(1100);
  voicePhase(vc.muted ? "muted" : "listening", vc.muted ? "Muted" : "Listening…");
}

function onVoiceFrame(level, chunk) {
  vc.level = level;
  $("#voice").style.setProperty("--level", Math.min(1, level * 10).toFixed(3));
  if (vc.phase !== "listening") return;
  vc.chunks.push(chunk);
  // Keep a little sound from just before you started talking.
  if (!vc.heard && vc.chunks.length > 6) vc.chunks.shift();
  const ev = vc.vad(level, (chunk.length / vc.mic.rate) * 1000);
  if (ev === "start") { vc.heard = true; voicePhase("listening", "Listening…"); }
  if (ev === "end" && vc.heard) voiceHeard();
}

async function voiceHeard() {
  voicePhase("thinking", "Thinking…");
  const wav = await wavBase64(to16k(vc.chunks, vc.mic.rate), 16000);
  vc.chunks = [];
  let text = "";
  try { text = await api().Transcribe(wav); } catch (e) { toast(e); }
  if (!vc.on) return;
  if (!text) return voiceListen();
  $("#voiceSaid").textContent = text;
  $("#voiceReply").textContent = "";
  vc.spoken = 0;
  vc.done = false;
  vc.queue = [];
  const msg = { role: "user", content: text, time: Date.now() };
  state.messages.push(msg);
  state.pendingNew = !state.chatId;
  renderThread(true);
  try {
    const id = await api().SendVoiceMessage(state.chatId || "", text, state.selected);
    if (!state.chatId) state.chatId = id;
    state.pendingNew = false;
    syncViewing();
    renderChatList();
  } catch (e) {
    state.messages.splice(state.messages.indexOf(msg), 1);
    state.pendingNew = false;
    renderThread();
    toast(e);
    voiceListen();
  }
}

// Called as the answer streams in: speak each finished stretch of sentences.
function voiceFeed(content, done) {
  if (!vc.on || vc.phase === "listening") return;
  const text = content.slice(vc.spoken);
  $("#voiceReply").textContent = previewLine(content);
  let cut = -1;
  const re = /[.!?…](?=\s)|\n\n/g;
  let m;
  const min = vc.spoken === 0 ? 30 : 140; // start talking quickly, then speak in longer pieces
  while ((m = re.exec(text))) if (m.index + 1 >= min) { cut = m.index + 1; break; }
  if (done) cut = text.length;
  if (cut > 0) {
    const piece = text.slice(0, cut).trim();
    vc.spoken += cut;
    if (piece) voiceSay(piece);
  }
  if (done) {
    vc.done = true;
    if (!vc.playing && !vc.queue.length) voiceListen();
  }
}

function previewLine(md) {
  const t = md.replace(/```[\s\S]*?(```|$)/g, " ").replace(/[#*_`>]/g, "").replace(/\s+/g, " ").trim();
  return t.length > 220 ? "…" + t.slice(-220) : t;
}

function voiceSay(text) {
  vc.queue.push(api().Speak(text).catch((e) => { toast(e); return null; }));
  if (!vc.playing) voicePlay();
}

async function voicePlay() {
  vc.playing = true;
  voicePhase("speaking", "Speaking… Tap the circle to interrupt");
  while (vc.on && vc.queue.length) {
    const url = await vc.queue.shift();
    if (!url) continue;
    if (!vc.on || vc.phase !== "speaking") { api().ForgetSpeech(url).catch(() => {}); continue; }
    await new Promise((resolve) => {
      const a = new Audio(url);
      vc.audio = a;
      a.onended = a.onerror = resolve;
      a.play().catch(resolve);
    });
    vc.audio = null;
    api().ForgetSpeech(url).catch(() => {});
  }
  vc.playing = false;
  if (vc.on && vc.done && vc.phase === "speaking") voiceListen();
}

// Tap the circle while it talks to cut in.
function voiceInterrupt() {
  if (!vc.on) return;
  if (vc.phase === "speaking" || vc.phase === "thinking") {
    if (vc.audio) vc.audio.pause();
    vc.queue.forEach((p) => p.then((u) => u && api().ForgetSpeech(u).catch(() => {})));
    vc.queue = [];
    vc.playing = false;
    if (state.gen) api().StopGeneration();
    vc.done = true;
    voiceListen();
  }
}

function endVoiceChat() {
  vc.on = false;
  if (vc.mic) vc.mic.close();
  vc.mic = null;
  if (vc.audio) vc.audio.pause();
  vc.queue = [];
  vc.playing = false;
  $("#voice").hidden = true;
  $("#talkBtn").focus();
}

// ---------- files in messages ----------

const maxFiles = () => state.info?.maxFiles || 10;
const MAX_FILE_BYTES = 25 * 2 ** 20;
const CODE_FILE = /\.(m?js|jsx|tsx?|py|ipynb|go|rs|java|kt|c|h|cpp|hpp|cs|php|rb|swift|lua|r|sql|sh|bat|ps1|html?|css|scss|vue|svelte|json|xml|ya?ml|toml|ini|svg)$/i;
let attachSeq = 0;

// Where the window loads a file from: the message box ("pending") or a chat.
function attURL(scope, id) {
  return window.__attURL ? window.__attURL(id) : `/att/${scope}/${encodeURIComponent(id)}`;
}

function attMeta(a) {
  if (a.kind === "image") return a.width ? `Picture, ${a.width}×${a.height}` : "Picture";
  if (a.kind === "audio") {
    if (a.transcribing) return "Audio, writing down what's said…";
    return ["Audio", a.duration ? fmtClock(a.duration) : null, a.speakers > 1 ? `${a.speakers} people talking` : null].filter(Boolean).join(", ");
  }
  let t;
  if (a.kind === "pdf") t = `PDF, ${a.pages} page${a.pages === 1 ? "" : "s"}`;
  else if (a.kind === "doc") t = "Word document";
  else t = `${CODE_FILE.test(a.name) ? "Code" : "Text"}, ${fmtCount(a.chars || 0)} characters`;
  return a.truncated ? t + ", only the start kept" : t;
}

function fileTile(name) {
  const dot = name.lastIndexOf(".");
  const ext = dot > 0 ? name.slice(dot + 1).slice(0, 4).toUpperCase() : "TXT";
  return h("span", { class: "file-tile", "aria-hidden": "true" }, ext);
}

function chipText(a, meta) {
  return h("span", { class: "chip-text" }, h("span", { class: "chip-name" }, a.name), h("span", { class: "chip-meta" }, meta || attMeta(a)));
}

function renderAttachments() {
  const box = $("#attachments");
  box.textContent = "";
  box.hidden = !state.pending.length;
  for (const p of state.pending) {
    const remove = h("button", { type: "button", class: "chip-x", title: "Remove", "aria-label": `Remove ${p.name}`, onclick: () => removePending(p) }, ic("x"));
    if (p.kind === "audio") {
      box.append(h("div", { class: "chip" + (p.transcribing ? " loading" : ""), title: p.name },
        p.transcribing ? h("span", { class: "file-tile" }, h("span", { class: "spinner" })) : h("span", { class: "file-tile audio" }, ic("audio")),
        chipText(p),
        remove));
    } else if (p.kind === "image" && !p.loading) {
      box.append(h("div", { class: "chip chip-img" },
        h("button", { type: "button", class: "chip-open", title: p.name, "aria-label": `View ${p.name}`, onclick: () => openImageViewer(p, "pending") },
          h("img", { src: attURL("pending", p.id), alt: "" })),
        remove));
    } else {
      box.append(h("div", { class: "chip" + (p.loading ? " loading" : ""), title: p.name },
        p.loading ? h("span", { class: "file-tile" }, h("span", { class: "spinner" })) : fileTile(p.name),
        chipText(p, p.loading ? "Reading…" : null),
        remove));
    }
  }
  updateComposer();
}

function removePending(p) {
  state.pending = state.pending.filter((x) => x !== p);
  if (p.id) api().RemovePendingAttachment(p.id).catch(() => {});
  renderAttachments();
  $("#input").focus();
}

// addFiles takes [{path}] (from the picker or a drop) or [{name, data}] (pasted).
async function addFiles(sources) {
  if (!sources.length) return;
  const room = maxFiles() - state.pending.length;
  if (room <= 0) return toast(`You can add up to ${maxFiles()} files to one message.`);
  if (sources.length > room) {
    toast(`You can add up to ${maxFiles()} files to one message, so only the first ${room} were added.`);
    sources = sources.slice(0, room);
  }
  if (state.view !== "chat") setView("chat");
  await Promise.all(sources.map(async (src) => {
    const p = { key: ++attachSeq, name: src.name || src.path.split(/[\\/]/).pop(), loading: true };
    state.pending.push(p);
    renderAttachments();
    try {
      const att = src.path ? await api().AddAttachmentFromPath(src.path) : await api().AddAttachmentData(src.name, src.data);
      if (!state.pending.includes(p)) return api().RemovePendingAttachment(att.id).catch(() => {}); // removed while it was loading
      Object.assign(p, att, { loading: false });
      if (att.kind === "audio" && att.transcribing) transcribePending(p);
    } catch (e) {
      state.pending = state.pending.filter((x) => x !== p);
      if (/speech recognition/i.test(errText(e))) offerVoice("listen");
      else toast(e);
    }
    renderAttachments();
  }));
}

// The window turns the audio into WAV; Phoenix writes down what's said.
async function transcribePending(p) {
  renderAttachments();
  try {
    const buf = await (await fetch(attURL("pending", p.audio))).arrayBuffer();
    const wav = await audioFileToWav(buf);
    const att = await api().TranscribeAttachment(p.id, wav);
    if (state.pending.includes(p)) Object.assign(p, att);
  } catch (e) {
    if (state.pending.includes(p)) {
      state.pending = state.pending.filter((x) => x !== p);
      api().RemovePendingAttachment(p.id).catch(() => {});
      toast(`${p.name}: ${errText(e)}`);
    }
  }
  renderAttachments();
}

function readAsDataURL(file) {
  return new Promise((resolve, reject) => {
    const r = new FileReader();
    r.onload = () => resolve(r.result);
    r.onerror = () => reject(new Error(`Couldn't read ${file.name}`));
    r.readAsDataURL(file);
  });
}

// Files the window has in memory: pasted screenshots, or drops when the
// file's location isn't known.
async function addBrowserFiles(files) {
  const ok = [];
  for (const f of files) {
    if (f.size > MAX_FILE_BYTES) { toast(`${f.name} is too big. Files can be up to 25 MB.`); continue; }
    const name = !f.name || /^image\.(png|jpe?g|gif|bmp|webp)$/i.test(f.name) ? "Pasted image." + ((f.type.split("/")[1] || "png").replace("jpeg", "jpg")) : f.name;
    try { ok.push({ name, data: await readAsDataURL(f) }); } catch (e) { toast(e); }
  }
  addFiles(ok);
}

async function pickFiles() {
  let paths;
  try { paths = await api().PickFiles(); } catch (e) { return toast(e); }
  if (paths && paths.length) addFiles(paths.map((path) => ({ path })));
  $("#input").focus();
}

function sentFilesEl(m) {
  const scope = m._local ? "pending" : state.chatId;
  const imgs = m.attachments.filter((a) => a.kind === "image");
  const docs = m.attachments.filter((a) => a.kind !== "image");
  const box = h("div", { class: "sent-files" });
  if (imgs.length) {
    box.append(h("div", { class: "sent-images" + (imgs.length === 1 ? " one" : "") },
      imgs.map((a) => h("button", { class: "sent-img", title: a.name, "aria-label": `View ${a.name}`, onclick: () => openImageViewer(a, scope) },
        h("img", { src: attURL(scope, a.id), alt: a.name, loading: "lazy", width: a.width || null, height: a.height || null })))));
  }
  const sounds = docs.filter((a) => a.kind === "audio");
  const texts = docs.filter((a) => a.kind !== "audio");
  for (const a of sounds) {
    box.append(h("div", { class: "sent-audio" },
      h("div", { class: "sent-audio-head" }, h("span", { class: "file-tile audio" }, ic("audio")), chipText(a),
        h("button", { class: "btn btn-quiet", onclick: () => openTextViewer(a, scope) }, "Transcript")),
      h("audio", { src: attURL(scope, a.audio), controls: true, preload: "none" })));
  }
  if (texts.length) {
    box.append(h("div", { class: "sent-docs" },
      texts.map((a) => h("button", { class: "chip", title: `Open ${a.name}`, onclick: () => openTextViewer(a, scope) }, fileTile(a.name), chipText(a)))));
  }
  return box;
}

// ---------- full-screen viewer (previews, code, pictures, files) ----------

let viewerOpener = null;
function openViewer(title, tools, body) {
  viewerOpener = document.activeElement;
  $("#viewerTitle").textContent = "";
  $("#viewerTitle").append(...[].concat(title));
  const t = $("#viewerTools");
  t.textContent = "";
  t.append(...tools.filter(Boolean));
  const b = $("#viewerBody");
  b.textContent = "";
  b.append(body);
  $("#viewer").hidden = false;
  closeMenu();
  $("#viewerClose").focus();
}
function closeViewer() {
  const v = $("#viewer");
  if (!v || v.hidden) return;
  v.hidden = true;
  $("#viewerBody").textContent = ""; // stops anything running in a preview
  if (viewerOpener && document.contains(viewerOpener)) viewerOpener.focus();
  viewerOpener = null;
}

function numberedCode(src) {
  const code = h("code", { class: "numbered" });
  for (const line of src.replace(/\n$/, "").split("\n")) code.append(h("span", { class: "ln" }, line || "​"), "\n");
  return h("pre", { class: "viewer-code" }, code);
}

function openCodeViewer({ label, src, doc, mode }) {
  const body = h("div", { class: "viewer-fill" });
  const show = (m) => {
    body.textContent = "";
    body.append(m === "preview" ? previewFrame(doc, `Preview of the ${label} code`) : numberedCode(src));
  };
  const copy = h("button", { type: "button", class: "btn" }, ic("copy"), h("span", {}, "Copy code"));
  copy.addEventListener("click", () => copyText(src, copy.lastChild));
  const reload = h("button", { type: "button", class: "btn btn-quiet", title: "Run the page again from the start", onclick: () => { if (body.querySelector("iframe")) show("preview"); } }, ic("retry"), "Reload");
  const seg = segmented([["preview", "Preview"], ["code", "Code"]], mode, (m) => { reload.hidden = m !== "preview"; show(m); }, "Show the preview or the code");
  reload.hidden = mode !== "preview";
  openViewer(label, [seg, reload, copy], body);
  show(mode);
}

function openImageViewer(a, scope) {
  const img = h("img", { src: attURL(scope, a.id), alt: a.name });
  openViewer([a.name, h("small", {}, attMeta(a))], [], h("div", { class: "viewer-image" }, img));
}

async function openTextViewer(a, scope) {
  const body = h("div", { class: "viewer-fill" }, h("div", { class: "note" }, "Opening…"));
  const copy = h("button", { type: "button", class: "btn", disabled: true }, ic("copy"), h("span", {}, "Copy text"));
  openViewer([a.name, h("small", {}, attMeta(a) + (a.kind === "audio" ? ". This is what Phoenix heard." : ". This is the text the model reads."))], [copy], body);
  try {
    const text = window.__attText ? window.__attText(a.id) : await (await fetch(attURL(scope, a.id))).text();
    body.textContent = "";
    body.append(numberedCode(text));
    copy.disabled = false;
    copy.addEventListener("click", () => copyText(text, copy.lastChild));
  } catch {
    body.textContent = "";
    body.append(h("div", { class: "note" }, "Phoenix couldn't find this file anymore."));
  }
}

// ---------- composer ----------

function autosize() {
  const t = $("#input");
  t.style.height = "auto";
  t.style.height = Math.min(t.scrollHeight, 240) + "px";
}

function updateComposer() {
  const busy = !!state.gen;
  const btn = $("#sendBtn");
  btn.classList.toggle("stop", busy);
  btn.innerHTML = "";
  btn.append(ic(busy ? "stop" : "send"));
  btn.setAttribute("aria-label", busy ? "Stop" : "Send");
  btn.title = busy ? "Stop" : "Send";
  const blockedNew = !state.chatId && chatsFull();
  const m = selectedModel();
  const kind = kindOf(m);
  const media = isMediaKind(kind);
  const blind = m && !media && !m.vision && state.pending.some((p) => p.kind === "image");
  const notPicture = media && state.pending.some((p) => p.kind !== "image");
  const unready = m && m.ready === false;
  btn.disabled = !busy && (!state.selected || blockedNew || blind || notPicture || unready);
  $("#thinkToggle").setAttribute("aria-pressed", String(state.think));
  $("#attachBtn").disabled = state.pending.length >= (media ? 1 : maxFiles());
  $("#attachBtn").title = media ? "Add a picture to start from" : "Add pictures, PDFs, documents, code or audio";
  $("#input").placeholder = kind === "image" ? "Describe the picture you want" : kind === "video" ? "Describe the video you want" : "Ask anything";
  $("#talkBtn").hidden = media;
  renderMediaOpts();

  let hint = media ? "Enter to make it" : "Enter to send, Shift+Enter for a new line";
  let warn = false;
  if (!state.selected) hint = "Download a model to start chatting";
  else if (unready) { warn = true; hint = `${m.name} isn't ready yet. Finish it on the Models page.`; }
  else if (notPicture) { warn = true; hint = `${m.name} can only start from a picture. Remove the other files.`; }
  else if (busy && state.gen.chatId !== state.chatId) hint = "Answering in another chat. Stop it to send here.";
  else if (blockedNew) hint = `Delete a chat to start a new one (${maxChats()} max)`;
  else if (blind) {
    warn = true;
    hint = m.visionAvailable
      ? `${m.name} needs its image support to see pictures. Add it on the Models page.`
      : `${m.name} can't see pictures. Pick a model that sees images, or remove the picture.`;
  }
  const el = $("#composerHint");
  el.textContent = hint;
  el.classList.toggle("warn", warn);
}

async function send() {
  if (state.gen) {
    api().StopGeneration();
    return;
  }
  const input = $("#input");
  const text = input.value.trim();
  if (state.pending.some((p) => p.loading || p.transcribing)) return toast("Wait a moment, Phoenix is still reading your files.", "info", 3000);
  const files = state.pending.map(({ key, loading, ...a }) => a);
  if (!text && !files.length) return;
  if (!state.selected) {
    toast("Download a model first.");
    setView("models");
    return;
  }
  if (!state.chatId && chatsFull()) {
    toast(`You have ${maxChats()} chats already. Delete one to start a new chat.`);
    return;
  }
  const kind = kindOf(selectedModel());
  if (isMediaKind(kind) && !text) return toast("Describe what you'd like to see first.", "info", 3000);
  const isNew = !state.chatId;
  const msg = { role: "user", content: text, attachments: files, time: Date.now(), _local: true };
  const keep = state.pending;
  state.messages.push(msg);
  state.pendingNew = isNew;
  state.pending = [];
  renderAttachments();
  input.value = "";
  autosize();
  renderThread(true);
  try {
    const ids = files.map((f) => f.id);
    const id = isMediaKind(kind)
      ? await api().MakeMedia(state.chatId || "", text, state.selected, { shape: state.mediaOpts.shape, length: state.mediaOpts.length }, ids)
      : await api().SendMessage(state.chatId || "", text, state.selected, state.think, ids);
    if (!state.chatId) state.chatId = id;
    state.pendingNew = false;
    delete msg._local; // its files now live in the chat
    if (files.length) renderThread();
    syncViewing();
    renderChatList();
  } catch (e) {
    state.messages.splice(state.messages.indexOf(msg), 1);
    state.pendingNew = false;
    state.pending = keep;
    renderAttachments();
    input.value = text;
    autosize();
    renderThread();
    toast(e);
  }
}

async function retry() {
  if (!state.chatId || state.gen) return;
  if (!state.selected) return toast("Download a model first.");
  if (isMediaKind(kindOf(selectedModel()))) {
    // Make another: the earlier one stays.
    try { await api().Regenerate(state.chatId, state.selected, false); await reloadChat(); } catch (e) { toast(e); }
    return;
  }
  const removed = [];
  while (state.messages.length && state.messages[state.messages.length - 1].role === "assistant") removed.unshift(state.messages.pop());
  renderThread();
  try {
    await api().Regenerate(state.chatId, state.selected, state.think);
  } catch (e) {
    state.messages.push(...removed);
    renderThread();
    toast(e);
  }
}

async function reloadChat() {
  if (!state.chatId) return;
  try {
    const c = await api().GetChat(state.chatId);
    state.messages = c.messages || [];
    renderThread(true);
  } catch {}
}

// ---------- model picker & status ----------

function selectedModel() { return state.installed.find((m) => m.id === state.selected); }

function renderPicker() {
  const m = selectedModel();
  $("#pickerName").textContent = m ? m.name : state.installed.length ? "Choose a model" : "No models yet";
  const menu = $("#pickerMenu");
  menu.textContent = "";
  const choices = state.installed.filter(pickable);
  const groups = [["chat", "Chat"], ["image", "Pictures"], ["video", "Videos"]];
  for (const [kind, label] of groups) {
    const list = choices.filter((im) => kindOf(im) === kind);
    if (!list.length) continue;
    if (choices.some((im) => kindOf(im) !== "chat")) menu.append(h("div", { class: "picker-group" }, label));
    for (const im of list) {
      menu.append(h("button", {
        class: "picker-opt", role: "option", "aria-selected": String(im.id === state.selected),
        onclick: () => pickModel(im.id),
      },
        h("span", { class: "n" }, kind === "image" ? ic("image") : kind === "video" ? ic("film") : null, im.name),
        h("span", { class: "sz" }, fmtBytes(im.size)),
        h("span", { class: "s" }, im.ready === false ? `Still getting ready: ${im.missing}` : [im.params, im.quant, im.vision ? "sees images" : null, im.fit.label].filter(Boolean).join(", ")),
      ));
    }
  }
  if (!choices.length) menu.append(h("div", { class: "note" }, "You haven't downloaded any models yet."));
  menu.append(h("button", { class: "picker-more", onclick: () => { closePicker(); setView("models"); } }, "Get more models"));
  renderModelState();
}

function pickModel(id) {
  closePicker();
  if (id === state.selected) return;
  const was = kindOf(selectedModel());
  state.selected = id;
  // Leftover files that the new kind of model can't use stay, with a hint.
  renderPicker();
  updateComposer();
  if (kindOf(selectedModel()) !== was && !state.messages.length) renderStarter();
  if (!state.gen) api().LoadModel(id).catch(toast);
  $("#input").focus();
}

function openPicker() {
  $("#pickerMenu").hidden = false;
  $("#pickerBtn").setAttribute("aria-expanded", "true");
  const sel = $('.picker-opt[aria-selected="true"]') || $(".picker-opt");
  sel && sel.focus();
}
function closePicker() {
  $("#pickerMenu").hidden = true;
  $("#pickerBtn").setAttribute("aria-expanded", "false");
}

function renderModelState() {
  const el = $("#modelState");
  const s = state.modelStatus || { state: "idle" };
  el.className = "model-state " + s.state;
  el.textContent = "";
  el.title = "";
  const relevant = s.modelId && s.modelId === state.selected;
  if (s.state === "idle" || !relevant) return;
  let text = "";
  if (s.state === "loading") text = "Loading into memory…";
  if (s.state === "ready") text = s.note || (s.onGpu ? "Running on your graphics card" : "Running on your processor") + (s.vision ? ", sees images" : "");
  if (s.state === "error") text = s.error;
  el.title = text;
  el.append(h("span", { class: "dot" }), h("span", { class: "note" }, text));
}

// ---------- models page ----------

function heatEl(fit) {
  const lvl = { great: 3, ok: 2, slow: 1, toobig: 0 }[fit.level] ?? 0;
  return h("span", { class: "heat l" + lvl, title: fit.label },
    h("span", { class: "bars", "aria-hidden": "true" }, h("i"), h("i"), h("i")),
    fit.label,
  );
}

function progressEl(d) {
  const pct = d.total ? Math.min(100, (d.received / d.total) * 100) : 0;
  let left = "";
  let right = "";
  if (d.status === "verifying") left = "Checking the file…";
  else if (d.status === "unpacking") left = "Unpacking…";
  else if (d.status === "paused") left = `Paused at ${fmtBytes(d.received)}${d.total ? " of " + fmtBytes(d.total) : ""}`;
  else {
    left = `${fmtBytes(d.received)}${d.total ? " of " + fmtBytes(d.total) : ""}`;
    right = [d.speed ? (d.speed / 2 ** 20).toFixed(1) + " MB/s" : "", fmtEta(d.total - d.received, d.speed)].filter(Boolean).join(", ");
  }
  return h("div", { class: "progress", "data-dl": d.id },
    h("div", { class: "bar", role: "progressbar", "aria-valuemin": "0", "aria-valuemax": "100", "aria-valuenow": String(Math.round(pct)) },
      h("span", { style: `width:${pct}%` })),
    h("div", { class: "progress-text" }, h("span", {}, left), h("span", {}, right)),
  );
}

function renderPcSummary() {
  const s = state.sys;
  if (!s) return;
  const parts = [];
  if (s.gpus && s.gpus.length) {
    const g = s.gpus.reduce((a, b) => (b.vram > a.vram ? b : a));
    parts.push(`Your PC has ${g.name} with ${fmtBytes(g.vram)} of video memory, and ${fmtBytes(s.ram)} of system memory.`);
  } else {
    parts.push(`Your PC has ${fmtBytes(s.ram)} of memory${s.gpuChecked ? " and no graphics card Phoenix can use, so models run on the processor" : ""}.`);
  }
  parts.push(`${fmtBytes(s.diskFree)} free for models.`);
  $("#pcSummary").textContent = parts.join(" ");
}

function renderModelsPage() {
  renderPcSummary();
  renderDownloadSection();
  renderInstalledSection();
  renderCatalogSection();
  renderHubResults();
}

function renderDownloadSection() {
  const box = $("#dlSection");
  box.textContent = "";
  const list = Object.values(state.downloads);
  if (!list.length) return;
  const rows = h("div", { class: "rows" });
  for (const d of list) {
    const actions = h("div", { class: "row-side" });
    if (d.status === "paused") actions.append(h("button", { class: "btn", onclick: () => resumeDownload(d) }, "Resume"));
    if (d.status === "error") actions.append(h("button", { class: "btn", onclick: () => resumeDownload(d) }, ic("retry"), "Try again"));
    if (d.status === "unpacking") actions.textContent = "";
    actions.append(h("button", { class: "btn btn-quiet", onclick: () => cancelDownload(d) }, d.status === "error" ? "Remove" : "Cancel"));
    const by = d.tool ? "Runs the models" : d.part ? "Part of a model" : d.repo;
    const main = h("div", {},
      h("div", { class: "row-title" }, h("span", { class: "n" }, d.name), h("span", { class: "by" }, by)),
      d.status === "error" ? h("div", { class: "row-desc" }, d.error) : progressEl(d),
    );
    rows.append(h("div", { class: "row" }, main, actions));
  }
  box.append(h("section", { class: "block" }, h("h2", {}, "Downloads"), rows));
}

function renderInstalledSection() {
  const box = $("#installedSection");
  box.textContent = "";
  const total = state.installed.reduce((a, m) => a + m.size, 0);
  const used = modelSlotsUsed();
  const block = h("section", { class: "block" },
    h("div", { class: "block-head" },
      h("h2", {}, "On this PC"),
      h("span", { class: "count-pill" + (modelsFull() ? " full" : ""), title: `Phoenix keeps up to ${maxModels()} models` }, `${used} of ${maxModels()} models`),
    ),
    h("p", { class: "block-sub" }, state.installed.length
      ? `Using ${fmtBytes(total)} of disk space.` + (modelsFull() ? ` ${fullModelsText()}` : "")
      : "Nothing downloaded yet. Pick a model below to get started."),
  );
  if (state.installed.length) {
    const rows = h("div", { class: "rows" });
    for (const m of state.installed) {
      const kind = kindOf(m);
      const busy = Object.values(state.downloads).some((d) => d.catalogId === m.catalogId && d.status !== "error") || Object.values(state.downloads).some((d) => d.tool && d.status !== "error");
      const use = !pickable(m) ? null
        : h("button", { class: "btn", disabled: m.ready === false, onclick: () => { pickModel(m.id); newChat(); } }, kind === "image" ? "Make pictures" : kind === "video" ? "Make videos" : "Chat");
      rows.append(h("div", { class: "row" },
        h("div", {},
          h("div", { class: "row-title" }, h("span", { class: "n" }, m.name), h("span", { class: "by" }, [m.maker, m.params].filter(Boolean).join(" ")),
            KIND_LABEL[kind] ? h("span", { class: "tag kind" }, KIND_LABEL[kind]) : null),
          h("div", { class: "row-facts" }, heatEl(m.fit), h("span", {}, fmtBytes(m.size)), m.quant ? h("span", {}, m.quant) : null,
            m.vision ? h("span", { class: "sees" }, ic("image"), "Sees images") : null),
          m.visionAvailable ? visionOffer(m) : null,
          m.ready === false ? h("div", { class: "vision-offer" },
            busy ? h("span", { class: "spinner" }) : null,
            busy ? `Downloading the ${lowerFirst(m.missing)}…` : `Still needs the ${lowerFirst(m.missing)}.`,
            busy ? null : h("button", { class: "btn btn-quiet link", onclick: () => downloadCatalog(m.catalogId) }, ic("download"), "Finish download")) : null,
        ),
        h("div", { class: "row-side" },
          use,
          h("button", { class: "btn btn-quiet danger", "aria-label": `Delete ${m.name}`, onclick: () => deleteModel(m) }, ic("trash"), "Delete"),
        ),
      ));
    }
    block.append(rows);
  }
  box.append(block);
}

// A model that can see pictures once its image support is downloaded.
function visionOffer(m) {
  const dl = Object.values(state.downloads).find((d) => isVisionFile(d.id) && d.id.split("/")[0] === m.id.split("/")[0] && d.status !== "error");
  const box = h("div", { class: "vision-offer" });
  if (dl) {
    box.append(h("span", { class: "spinner" }), dl.status === "paused" ? "Image support paused" : `Adding image support: ${dl.total ? Math.floor((dl.received / dl.total) * 100) : 0}%`);
  } else {
    box.append(
      h("span", {}, "This model can see pictures with its image support."),
      h("button", { class: "btn btn-quiet link", onclick: async (e) => {
        const b = e.currentTarget;
        if (!(await roomFor(m.visionSize, "Image support for " + m.name))) return;
        b.disabled = true;
        try { await call("AddImageSupport", m.id); } catch {}
      } }, ic("image"), `Add it (${fmtBytes(m.visionSize)})`),
    );
  }
  return box;
}

const CATALOG_GROUPS = [
  ["chat", "Chat models", "Open models picked for quality. The heat bars show how well each one runs on your PC."],
  ["media", "Pictures and video", "Describe something and these make it. They need a strong graphics card to be quick."],
  ["voice", "Voice", "Small helpers that let you talk with Phoenix, add audio files, and hear answers read aloud. They run on your processor and don't count toward your model limit."],
];

function catalogRow(m, full) {
  const kind = kindOf(m);
  const counts = ["chat", "image", "video"].includes(kind);
  const dl = Object.values(state.downloads).find((d) => d.catalogId === m.id && d.status !== "error" && !d.part);
  let side;
  if (m.installed) side = h("span", { class: "tag hot" }, "Downloaded");
  else if (dl) side = h("span", { class: "tag" }, dl.status === "paused" ? "Paused" : "Downloading");
  else side = h("button", {
    class: "btn" + (m.tags.includes("Recommended") && !full && m.fit.level !== "toobig" ? " btn-primary" : ""),
    disabled: full && counts, title: full && counts ? fullModelsText() : null,
    onclick: () => downloadCatalog(m.id),
  }, ic("download"), "Download");
  return h("div", { class: "row" },
    h("div", {},
      h("div", { class: "row-title" },
        h("span", { class: "n" }, m.name),
        h("span", { class: "by" }, `${m.maker} ${m.params}`),
        m.tags.map((t) => h("span", { class: "tag" + (t === "Recommended" || t === "Best for coding" ? " hot" : "") }, t)),
      ),
      h("div", { class: "row-desc" }, m.description),
      h("div", { class: "row-facts" }, heatEl(m.fit), h("span", {}, fmtBytes(m.totalSize || m.size)),
        m.parts && m.parts.length ? h("span", {}, `includes ${m.parts.length === 1 ? "1 extra part" : m.parts.length + " extra parts"}`) : null),
    ),
    h("div", { class: "row-side" }, side),
  );
}

function renderCatalogSection() {
  const box = $("#catalogSection");
  box.textContent = "";
  const full = modelsFull();
  for (const [group, title, sub] of CATALOG_GROUPS) {
    const list = state.catalog.filter((m) => {
      const k = kindOf(m);
      return group === "chat" ? k === "chat" : group === "media" ? isMediaKind(k) : !["chat", "image", "video"].includes(k);
    });
    if (!list.length) continue;
    const rows = h("div", { class: "rows" });
    list.forEach((m) => rows.append(catalogRow(m, full)));
    box.append(h("section", { class: "block" },
      h("h2", {}, title),
      h("p", { class: "block-sub" }, sub),
      full && group !== "voice" ? h("p", { class: "limit-note" }, fullModelsText()) : null,
      rows,
    ));
  }
}

function renderDownloadBits() {
  const active = Object.values(state.downloads).filter((d) => d.status === "downloading" || d.status === "verifying");
  const badge = $("#dlBadge");
  badge.hidden = !active.length;
  if (active.length) {
    const got = active.reduce((a, d) => a + d.received, 0);
    const tot = active.reduce((a, d) => a + (d.total || 0), 0);
    badge.textContent = tot ? Math.floor((got / tot) * 100) + "%" : "…";
  }
  if (state.view === "models") {
    renderDownloadSection();
    renderInstalledSection();
    renderCatalogSection();
    updateHubButtons();
  }
  if (!state.installed.length && $("#empty") && !$("#empty").hidden) renderStarter();
}

// Ask first when a download would use more than a quarter of the free space.
async function roomFor(bytes, name) {
  try { state.sys = await api().GetSystemInfo(); } catch {}
  const free = state.sys && state.sys.diskFree;
  if (!free || bytes <= free * 0.25) return true;
  return confirmDialog({
    title: "This is a big download",
    body: `${name} needs ${fmtBytes(bytes)}. That's more than a quarter of the ${fmtBytes(free)} free on your drive. Download it anyway?`,
    ok: "Download anyway",
    danger: false,
  });
}

async function downloadCatalog(id) {
  const m = state.catalog.find((x) => x.id === id);
  if (m && !m.installed && !(await roomFor(m.totalSize || m.size, m.name))) return;
  try { await call("DownloadModel", id); } catch {}
}
async function resumeDownload(d) {
  try {
    if (d.tool || !d.repo) await call("DownloadModel", d.catalogId || state.catalog.find((m) => !m.installed && m.kind && m.kind !== "chat")?.id);
    else if (d.catalogId && !d.part) await call("DownloadModel", d.catalogId);
    else await call("DownloadFromHub", d.repo, d.file);
  } catch {}
}
async function cancelDownload(d) {
  await api().CancelDownload(d.id).catch(toast);
  delete state.downloads[d.id];
  renderDownloadBits();
}
async function deleteModel(m) {
  const ok = await confirmDialog({ title: `Delete ${m.name}?`, body: `This frees ${fmtBytes(m.size)}. You can download it again any time.` });
  if (!ok) return;
  try { await call("DeleteModel", m.id); } catch {}
  await refreshModels();
}

// Hugging Face search

async function searchHub(q) {
  state.hub.query = q;
  state.hub.loading = true;
  state.hub.open = {};
  renderHubResults();
  try {
    state.hub.results = (await api().SearchHub(q)) || [];
  } catch (e) {
    state.hub.results = [];
    toast(e);
  }
  state.hub.loading = false;
  renderHubResults();
}

function renderHubResults() {
  const box = $("#hubResults");
  box.textContent = "";
  const hs = state.hub;
  if (hs.loading) return box.append(h("div", { class: "note" }, "Searching…"));
  if (!hs.query) return;
  if (!hs.results.length) return box.append(h("div", { class: "note" }, `No models found for “${hs.query}”. Try a shorter name.`));
  for (const r of hs.results) {
    const item = h("div", { class: "hub-item" + (hs.open[r.repo] ? " open" : "") },
      h("button", { class: "hub-head", "aria-expanded": String(!!hs.open[r.repo]), onclick: () => toggleHub(r.repo) },
        h("span", { class: "repo" }, r.repo.split("/")[1], h("small", {}, "by " + r.repo.split("/")[0])),
        h("span", { class: "count" }, fmtCount(r.downloads) + " downloads"),
        ic("chevron"),
      ),
    );
    if (hs.open[r.repo]) item.append(hubFilesEl(r.repo));
    box.append(item);
  }
}

function hubFilesEl(repo) {
  const files = state.hub.files[repo];
  const box = h("div", { class: "hub-files" });
  if (files === undefined) return box.append(h("div", { class: "note" }, "Loading files…")), box;
  if (files === null) return box.append(h("div", { class: "note" }, "Couldn't load this model's files.")), box;
  if (!files.length) return box.append(h("div", { class: "note" }, "This model has no single-file versions Phoenix can use.")), box;
  for (const f of files) {
    const id = repoDir(repo) + "/" + f.file;
    box.append(h("div", { class: "hub-file" },
      h("span", { class: "q", title: f.file }, f.quant || "file", f.recommended ? h("span", { class: "tag hot" }, "Best pick") : null, f.vision && f.recommended ? h("span", { class: "tag" }, "Sees images") : null),
      h("span", { class: "sz" }, fmtBytes(f.size)),
      heatEl(f.fit),
      h("button", { class: "btn", "data-hub-id": id, "data-installed": f.installed ? "1" : "", onclick: () => downloadHub(repo, f.file) }, "Download"),
    ));
  }
  setTimeout(updateHubButtons, 0);
  return box;
}

function updateHubButtons() {
  $$("[data-hub-id]").forEach((b) => {
    const id = b.dataset.hubId;
    const d = state.downloads[id];
    const installed = b.dataset.installed === "1" || state.installed.some((m) => m.id === id);
    const busy = d && d.status !== "error";
    const blocked = !installed && !busy && modelsFull();
    b.disabled = installed || busy || blocked;
    b.title = blocked ? fullModelsText() : "";
    b.textContent = installed ? "Downloaded" : busy ? (d.status === "paused" ? "Paused" : "Downloading") : "Download";
  });
}

function repoDir(repo) { return repo.replace("/", "__"); }

async function toggleHub(repo) {
  const hs = state.hub;
  hs.open[repo] = !hs.open[repo];
  renderHubResults();
  if (hs.open[repo] && hs.files[repo] === undefined) {
    try {
      hs.files[repo] = (await api().GetHubFiles(repo)) || [];
    } catch (e) {
      hs.files[repo] = null;
      toast(e);
    }
    renderHubResults();
  }
}

async function downloadHub(repo, file) {
  const f = (state.hub.files[repo] || []).find((x) => x.file === file);
  if (f && !(await roomFor(f.size, file))) return;
  try { await call("DownloadFromHub", repo, file); } catch {}
}

// ---------- settings ----------

const CONTEXT_OPTIONS = [
  [4096, "Short (4K)"],
  [8192, "Normal (8K)"],
  [16384, "Long (16K)"],
  [32768, "Very long (32K)"],
];

async function saveSettings(patch) {
  const next = { ...state.settings, ...patch };
  try {
    state.settings = await api().SaveSettings(next);
    applyTheme();
  } catch (e) { toast(e); }
}

function makeSwitch(checked, onToggle, label) {
  const b = h("button", { class: "switch", role: "switch", "aria-checked": String(checked), "aria-label": label });
  b.addEventListener("click", () => {
    const v = b.getAttribute("aria-checked") !== "true";
    b.setAttribute("aria-checked", String(v));
    onToggle(v);
  });
  return b;
}

function timeAgo(ms) {
  const s = (Date.now() - ms) / 1000;
  if (s < 60) return "just now";
  if (s < 3600) return Math.round(s / 60) + " min ago";
  if (s < 86400) return Math.round(s / 3600) + " hr ago";
  return new Date(ms).toLocaleDateString();
}

function updateLine(u) {
  const auto = state.settings && state.settings.autoUpdate;
  switch (u && u.state) {
    case "disabled": return "Updates are turned off in this version.";
    case "checking": return "Checking for updates…";
    case "downloading": return `Downloading version ${u.latest}` + (u.total ? `: ${Math.floor((u.received / u.total) * 100)}%` : "…");
    case "ready": return `Version ${u.latest} is ready. Restart Phoenix to finish updating` + (auto ? ", or it installs by itself when you close Phoenix." : ".");
    case "installing": return "Installing the update. Phoenix will reopen in a moment.";
    case "error": return u.error;
    case "uptodate": return "You have the latest version." + (u.checked ? ` Checked ${timeAgo(u.checked)}.` : "");
    default: return auto ? "Phoenix checks for updates by itself a few times a day." : "Automatic updates are off. Use Check for updates.";
  }
}

async function checkUpdates() {
  try {
    state.update = await api().CheckForUpdates();
  } catch {
    // The error is shown in the Updates section through update:status.
  }
  renderUpdateBlock();
  renderUpdatePill();
}

async function installUpdate() {
  if (state.gen) {
    const ok = await confirmDialog({ title: "Restart to update?", body: "An answer is still being written. Restarting stops it but keeps what's written so far.", ok: "Restart", danger: false });
    if (!ok) return;
    api().StopGeneration();
  }
  try { await api().InstallUpdate(); } catch (e) { toast(e); }
}

function renderUpdateBlock() {
  const box = $("#updateBlock");
  if (!box) return;
  const u = state.update || { state: "idle" };
  box.textContent = "";
  const actions = h("div", { class: "control" });
  if (u.state === "ready") {
    actions.append(h("button", { class: "btn btn-primary", onclick: installUpdate }, ic("update"), "Restart and update"));
  } else if (u.state !== "disabled" && u.state !== "installing") {
    const busy = u.state === "checking" || u.state === "downloading";
    actions.append(h("button", { class: "btn", disabled: busy, onclick: checkUpdates },
      u.state === "checking" ? "Checking…" : u.state === "downloading" ? "Downloading…" : "Check for updates"));
  }
  box.append(
    h("h2", {}, "Updates"),
    h("div", { class: "setting" },
      h("div", { class: "label" }, `Phoenix AI ${state.info.version}`),
      h("div", { class: "help", "aria-live": "polite" }, updateLine(u)),
      actions,
    ),
    h("div", { class: "setting" },
      h("div", { class: "label" }, "Update automatically"),
      h("div", { class: "help" }, "Download new versions in the background and install them when you close Phoenix."),
      h("div", { class: "control" }, makeSwitch(!!state.settings.autoUpdate, (v) => saveSettings({ autoUpdate: v }).then(renderUpdateBlock), "Update automatically")),
    ),
    u.state === "ready" && u.notes
      ? h("details", { class: "notes" }, h("summary", {}, ic("chevronRight"), `What's new in ${u.latest}`), markdownEl(u.notes))
      : "",
  );
}

function renderUpdatePill() {
  const u = state.update;
  const pill = $("#updatePill");
  const show = u && (u.state === "ready" || u.state === "installing");
  pill.hidden = !show;
  if (!show) return;
  pill.disabled = u.state === "installing";
  $("#updatePillTitle").textContent = u.state === "installing" ? "Updating…" : "Update ready";
  $("#updatePillSub").textContent = u.state === "installing" ? "Phoenix will reopen" : `Restart to get ${u.latest}`;
}

function renderSettings() {
  const s = state.settings;
  const page = $("#settingsPage");
  page.textContent = "";

  const save = saveSettings;

  const seg = (value, options, onPick) => h("div", { class: "segmented", role: "group" },
    options.map(([v, label]) => h("button", { "aria-pressed": String(v === value), onclick: (e) => {
      $$("button", e.currentTarget.parentElement).forEach((b) => b.setAttribute("aria-pressed", "false"));
      e.currentTarget.setAttribute("aria-pressed", "true");
      onPick(v);
    } }, label)));

  const sw = makeSwitch;

  const ctxSelect = h("select", { class: "select", "aria-label": "Memory length" },
    CONTEXT_OPTIONS.map(([v, l]) => h("option", { value: v, selected: v === s.contextSize }, l)));
  if (!CONTEXT_OPTIONS.some(([v]) => v === s.contextSize)) ctxSelect.prepend(h("option", { value: s.contextSize, selected: true }, `${s.contextSize} tokens`));
  ctxSelect.addEventListener("change", () => save({ contextSize: Number(ctxSelect.value) }));

  const sysPrompt = h("textarea", { placeholder: "For example: Keep answers short. I'm learning Java, so explain code step by step." });
  sysPrompt.value = s.systemPrompt || "";
  const sysSave = h("button", { class: "btn", disabled: true, onclick: async () => {
    await save({ systemPrompt: sysPrompt.value.trim() });
    sysSave.disabled = true;
    sysSave.textContent = "Saved";
  } }, "Save");
  sysPrompt.addEventListener("input", () => { sysSave.disabled = false; sysSave.textContent = "Save"; });

  const engineBox = h("div", { class: "control", style: "display:flex;gap:8px;align-items:center;flex-wrap:wrap;justify-content:flex-end" });
  const engineHelp = h("div", { class: "help", id: "engineHelp" }, engineLine());
  const checkBtn = h("button", { class: "btn" }, "Check for updates");
  checkBtn.addEventListener("click", async () => {
    checkBtn.disabled = true;
    checkBtn.textContent = "Checking…";
    try {
      const u = await api().CheckEngineUpdate();
      if (u.available) {
        checkBtn.remove();
        engineBox.append(h("button", { class: "btn btn-primary", onclick: (e) => { e.currentTarget.disabled = true; api().InstallEngine().catch(toast); } }, `Update to ${u.latest}`));
      } else {
        checkBtn.textContent = "Up to date";
      }
    } catch (e) {
      toast(e);
      checkBtn.disabled = false;
      checkBtn.textContent = "Check for updates";
    }
  });
  engineBox.append(checkBtn);

  // Voice
  const voiceSelect = h("select", { class: "select", "aria-label": "Voice" },
    state.voices.map((v) => h("option", { value: v.id, selected: v.id === s.voice }, `${v.name} (${v.accent})`)));
  voiceSelect.addEventListener("change", () => save({ voice: voiceSelect.value }));
  const tryBtn = h("button", { class: "btn" }, ic("volume"), "Try it");
  tryBtn.addEventListener("click", async () => {
    if (!state.speech.voice) return offerVoice("read");
    stopSpeaking();
    tryBtn.disabled = true;
    try {
      const v = state.voices.find((x) => x.id === voiceSelect.value);
      const url = await api().Speak(`Hi, I'm ${v ? v.name : "Phoenix"}. This is how I'll sound when I read answers to you.`);
      const audio = new Audio(url);
      speaking = { audio, btn: null, url };
      audio.onended = () => stopSpeaking();
      await audio.play();
    } catch (e) { toast(e); }
    tryBtn.disabled = false;
  });
  const sp = state.speech;
  const voiceStatus = (sp.listen && sp.voice ? "Speech recognition and the Kokoro voice are ready."
    : sp.listen ? "Speech recognition is ready. The Kokoro voice isn't downloaded yet."
    : sp.voice ? "The Kokoro voice is ready. Speech recognition isn't downloaded yet."
    : "Talking to Phoenix and hearing answers needs speech recognition and the Kokoro voice.")
    + (sp.speakers ? " Speaker detection is on for audio files." : "");

  page.append(
    h("header", { class: "page-head" }, h("h1", {}, "Settings"), h("p", { class: "page-sub" }, "Changes save as you make them.")),

    h("section", { class: "block" },
      h("h2", {}, "Look"),
      h("div", { class: "setting" },
        h("div", { class: "label" }, "Theme"),
        h("div", { class: "help" }, "Follow Windows, or always use dark or light."),
        h("div", { class: "control" }, seg(s.theme, [["system", "System"], ["dark", "Dark"], ["light", "Light"]], (v) => save({ theme: v }))),
      ),
    ),

    h("section", { class: "block" },
      h("h2", {}, "Notifications"),
      h("div", { class: "setting" },
        h("div", { class: "label" }, "Windows notifications"),
        h("div", { class: "help" }, "When you're in another window, Phoenix lets you know an answer or download is done. Click the notification to jump back."),
        h("div", { class: "control" }, sw(s.notifications !== false, (v) => save({ notifications: v }), "Windows notifications")),
      ),
    ),

    h("section", { class: "block" },
      h("h2", {}, "Voice"),
      h("p", { class: "block-sub" }, voiceStatus,
        (!sp.listen || !sp.voice) ? h("button", { class: "btn btn-quiet link", onclick: () => offerVoice("talk") }, ic("download"), "Get voice features") : null),
      h("div", { class: "setting" },
        h("div", { class: "label" }, "Voice"),
        h("div", { class: "help" }, "Who reads answers aloud and talks in voice chat."),
        h("div", { class: "control", style: "display:flex;gap:8px;align-items:center;justify-content:flex-end" }, voiceSelect, tryBtn),
      ),
      h("div", { class: "setting" },
        h("div", { class: "label" }, "Speaking speed"),
        h("div", { class: "help" }, "How fast the voice talks."),
        h("div", { class: "control" }, seg(s.voiceSpeed || 1, [[0.85, "Slower"], [1, "Normal"], [1.2, "Faster"]], (v) => save({ voiceSpeed: v }))),
      ),
    ),

    h("section", { class: "block", id: "updateBlock" }),

    h("section", { class: "block" },
      h("h2", {}, "Performance"),
      h("div", { class: "setting" },
        h("div", { class: "label" }, "Use graphics card"),
        h("div", { class: "help" }, "Much faster when your PC has one. Turn off only if models fail to load."),
        h("div", { class: "control" }, sw(s.useGPU, (v) => save({ useGPU: v }), "Use graphics card")),
      ),
      h("div", { class: "setting" },
        h("div", { class: "label" }, "Memory length"),
        h("div", { class: "help" }, "How much of a conversation the model can keep in mind. Longer uses more memory and may load slower."),
        h("div", { class: "control" }, ctxSelect),
      ),
    ),

    h("section", { class: "block" },
      h("h2", {}, "Custom instructions"),
      h("div", { class: "setting stack" },
        h("div", { class: "help" }, "Tell the AI how to answer in every chat."),
        h("div", { class: "control" }, sysPrompt, h("div", { style: "margin-top:8px;display:flex;justify-content:flex-end" }, sysSave)),
      ),
    ),

    h("section", { class: "block" },
      h("h2", {}, "AI engine"),
      h("div", { class: "setting" },
        h("div", { class: "label" }, "llama.cpp"),
        engineHelp,
        engineBox,
      ),
    ),

    h("section", { class: "block" },
      h("h2", {}, "Storage"),
      h("div", { class: "setting" },
        h("div", { class: "label" }, "Phoenix folder"),
        h("div", { class: "help" }, "Models, chats and the engine are kept here, away from your other files.", h("div", { class: "path" }, state.info.dataDir)),
        h("div", { class: "control" }, h("button", { class: "btn", onclick: () => api().OpenDataFolder().catch(toast) }, ic("folder"), "Open folder")),
      ),
    ),

    h("section", { class: "block" },
      h("h2", {}, "About"),
      h("p", { class: "credits" },
        `Phoenix AI ${state.info.version}. Runs models with llama.cpp (MIT license). `,
        "Uses marked and DOMPurify for formatting, and the Bricolage Grotesque typeface (SIL Open Font License). ",
        "Reads PDFs with ledongthuc/pdf (BSD) and shows notifications with go-toast (MIT). ",
        "Models come from Hugging Face and keep their own licenses.",
      ),
    ),
  );
  renderUpdateBlock();
}

function engineLine() {
  const e = state.engine || {};
  if (e.busy) return `${e.stage || "Working"}${e.total ? `: ${fmtBytes(e.received)} of ${fmtBytes(e.total)}` : ""}`;
  if (e.error) return e.error;
  return e.installed ? `The program that runs your models. Version ${e.version}.` : "Not installed yet.";
}

// ---------- setup ----------

function renderSetup() {
  const e = state.engine;
  const show = !e.installed || (e.busy && !document.body.dataset.ready);
  $("#setup").hidden = !show;
  if (!show) return;
  $("#setupProgress").hidden = !e.busy;
  $("#setupBtn").hidden = e.busy;
  $("#setupBtn").textContent = e.error ? "Try again" : "Set up Phoenix";
  $("#setupError").hidden = !e.error;
  $("#setupError").textContent = e.error ? errText(e.error) : "";
  const pct = e.total ? Math.min(100, (e.received / e.total) * 100) : e.stage === "Unpacking" ? 100 : 4;
  $("#setupBar").style.width = pct + "%";
  $("#setupStage").textContent = e.stage + (e.total && e.stage === "Downloading the engine" ? `: ${fmtBytes(e.received)} of ${fmtBytes(e.total)}` : "…");
}

// ---------- events from Go ----------

function wireEvents() {
  const on = window.runtime.EventsOn;

  on("engine:status", async (e) => {
    const wasInstalled = state.engine && state.engine.installed && !state.engine.busy;
    state.engine = e;
    renderSetup();
    if (state.view === "settings") {
      const help = $("#engineHelp");
      if (help) help.textContent = engineLine();
    }
    if (e.installed && !e.busy && !e.error) {
      document.body.dataset.ready = "1";
      $("#setup").hidden = true;
      if (!wasInstalled) toast("Phoenix is ready.", "info", 3000);
      await refreshSystem();
      refreshModels();
      if (state.view === "settings") renderSettings();
    }
  });

  on("download:progress", (d) => {
    if (d.status === "canceled") delete state.downloads[d.id];
    else if (d.status === "done") {
      delete state.downloads[d.id];
      toast(`${d.name} is ready to use.`, "info", 4000);
    } else state.downloads[d.id] = d;
    if (d.status === "error") toast(`${d.name}: ${d.error}`);
    renderDownloadBits();
  });

  on("models:changed", () => refreshModels());

  on("model:status", (s) => {
    state.modelStatus = s;
    renderModelState();
    if (state.gen && !state.gen.content && !state.gen.thinking) scheduleLive();
  });

  on("chats:changed", () => refreshChats());

  // A Windows notification was clicked.
  on("notify:open", (target) => {
    if (typeof target !== "string") return;
    if (target.startsWith("chat:")) openChat(target.slice(5));
    else if (target === "models") setView("models");
  });

  on("chat:start", (e) => {
    if (state.pendingNew && !state.chatId) {
      state.chatId = e.chatId;
      syncViewing();
    }
    state.gen = { chatId: e.chatId, content: "", thinking: "", modelName: e.modelName, kind: e.kind || "chat", shape: state.mediaOpts.shape, progress: null };
    if (e.chatId === state.chatId) renderThread(true);
    updateComposer();
    renderChatList();
  });

  on("media:progress", (p) => {
    if (!state.gen || state.gen.chatId !== p.chatId) return;
    const prev = state.gen.progress;
    // Keep the last preview until a newer one arrives.
    if (!p.preview && prev && prev.preview) p.preview = prev.preview;
    state.gen.progress = p;
    if (p.chatId === state.chatId) scheduleLive();
  });

  on("chat:delta", (d) => {
    if (!state.gen || state.gen.chatId !== d.chatId) return;
    if (d.thinking) state.gen.thinking += d.thinking;
    if (d.content) state.gen.content += d.content;
    if (d.chatId === state.chatId) scheduleLive();
    if (vc.on && d.chatId === state.chatId && d.content) voiceFeed(state.gen.content, false);
  });

  on("chat:done", (d) => {
    const onScreen = d.chatId === state.chatId && state.view === "chat";
    if (vc.on && d.chatId === state.chatId) {
      if (d.message.error) { toast(d.message.error); vc.done = true; voiceListen(); }
      else voiceFeed(d.message.content || "", true);
    }
    state.gen = null;
    if (d.chatId === state.chatId) {
      state.messages.push(d.message);
      const stick = nearBottom();
      renderThread();
      if (stick) scrollEnd(true);
    }
    if (!onScreen && !d.message.stopped) {
      const c = state.chats.find((x) => x.id === d.chatId);
      const what = d.message.media && d.message.media.length ? (d.message.media[0].kind === "video" ? "Your video is ready" : "Your picture is ready") : "New answer";
      if (c) toast(`${what} in “${c.title}”. Click to open it.`, "info", 6000, () => openChat(c.id));
    }
    updateComposer();
    renderChatList();
  });

  let announced = "";
  on("update:status", (u) => {
    state.update = u;
    renderUpdatePill();
    if (state.view === "settings") renderUpdateBlock();
    if (u.state === "ready" && announced !== u.latest) {
      announced = u.latest;
      toast(`Phoenix ${u.latest} is ready. Click to restart and update.`, "info", 8000, installUpdate);
    }
  });
}

// ---------- window: title bar, focus, dropping files ----------

function wireWindow() {
  const rt = window.runtime;
  $("#winMin").addEventListener("click", () => rt.WindowMinimise());
  $("#winMax").addEventListener("click", () => rt.WindowToggleMaximise());
  $("#winClose").addEventListener("click", () => rt.Quit());
  // Double-click the bar to maximize or restore, like any Windows app.
  $("#titlebar").addEventListener("dblclick", (e) => { if (!e.target.closest(".tb-controls")) rt.WindowToggleMaximise(); });

  let resizeTimer = 0;
  const syncMax = async () => {
    let max = false;
    try { max = await rt.WindowIsMaximised(); } catch {}
    document.body.classList.toggle("maximised", max);
    const b = $("#winMax");
    b.title = max ? "Restore down" : "Maximize";
    b.setAttribute("aria-label", b.title);
    return max;
  };
  addEventListener("resize", () => { clearTimeout(resizeTimer); resizeTimer = setTimeout(syncMax, 80); });
  // Never bigger than the screen: if Phoenix was last used on a bigger
  // monitor, or the screen changed, fill this one instead.
  syncMax().then((max) => {
    if (!max && (outerWidth > screen.availWidth + 8 || outerHeight > screen.availHeight + 8)) rt.WindowMaximise();
  });

  // Tell Phoenix whether you're looking at it, so it only shows Windows
  // notifications when you're somewhere else.
  let active = null;
  const syncActive = () => {
    const now = document.visibilityState === "visible" && document.hasFocus();
    if (now === active) return;
    active = now;
    api().SetWindowActive(now).catch(() => {});
  };
  addEventListener("focus", syncActive);
  addEventListener("blur", () => setTimeout(syncActive, 0)); // focus moving into a preview still counts as here
  document.addEventListener("visibilitychange", syncActive);
  syncActive();

  // Dropping files anywhere adds them to the message.
  const zone = $("#dropZone");
  let depth = 0;
  const hasFiles = (e) => e.dataTransfer && [...e.dataTransfer.types].includes("Files");
  const canDrop = () => $("#setup").hidden && $("#dialog").hidden;
  const hide = () => { depth = 0; zone.hidden = true; };
  document.addEventListener("dragenter", (e) => {
    if (!hasFiles(e) || !canDrop()) return;
    depth++;
    if (state.view !== "chat") setView("chat");
    zone.hidden = false;
  });
  document.addEventListener("dragleave", (e) => { if (hasFiles(e) && --depth <= 0) hide(); });
  document.addEventListener("dragover", (e) => { if (hasFiles(e)) { e.preventDefault(); e.dataTransfer.dropEffect = canDrop() ? "copy" : "none"; } });
  // In the app, Windows hands over the dropped files' locations (Wails
  // turns this on once the page has loaded, so check at drop time).
  const native = () => !!(window.wails && window.wails.flags && window.wails.flags.enableWailsDragAndDrop && rt.OnFileDrop);
  document.addEventListener("drop", (e) => {
    if (!hasFiles(e)) return;
    e.preventDefault();
    hide();
    if (!native() && canDrop()) addBrowserFiles([...e.dataTransfer.files]);
  });
  if (rt.OnFileDrop) rt.OnFileDrop((x, y, paths) => { hide(); if (canDrop() && paths && paths.length) addFiles(paths.map((path) => ({ path }))); }, false);
  addEventListener("blur", hide);
}

// ---------- startup ----------

function wireUI() {
  hydrateIcons();

  $("#newChat").addEventListener("click", newChat);
  $$(".nav-item").forEach((b) => b.addEventListener("click", () => setView(b.dataset.view)));

  const input = $("#input");
  input.addEventListener("input", autosize);
  input.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !e.shiftKey && !e.isComposing) {
      e.preventDefault();
      if (!state.gen) send();
    }
  });
  $("#composer").addEventListener("submit", (e) => { e.preventDefault(); send(); });
  $("#attachBtn").addEventListener("click", pickFiles);
  $("#micBtn").addEventListener("click", toggleDictation);
  $("#talkBtn").addEventListener("click", startVoiceChat);
  $("#voiceEnd").addEventListener("click", endVoiceChat);
  $("#voiceOrb").addEventListener("click", voiceInterrupt);
  $("#voiceMute").addEventListener("click", (e) => {
    vc.muted = !vc.muted;
    e.currentTarget.setAttribute("aria-pressed", String(vc.muted));
    $("span:last-child", e.currentTarget).textContent = vc.muted ? "Unmute" : "Mute";
    if (vc.phase === "listening" || vc.phase === "muted") voiceListen();
  });
  input.addEventListener("paste", (e) => {
    const files = [...((e.clipboardData && e.clipboardData.files) || [])];
    if (!files.length) return;
    e.preventDefault();
    addBrowserFiles(files);
  });
  $("#viewerClose").addEventListener("click", closeViewer);
  $("#thinkToggle").addEventListener("click", () => { state.think = !state.think; updateComposer(); });

  $("#pickerBtn").addEventListener("click", (e) => {
    e.stopPropagation();
    $("#pickerMenu").hidden ? openPicker() : closePicker();
  });
  document.addEventListener("click", (e) => {
    if (!e.target.closest("#picker")) closePicker();
    if (!e.target.closest("#ctxMenu")) closeMenu();
  });
  document.addEventListener("contextmenu", (e) => { if (!e.target.closest(".chat-item")) closeMenu(); });
  $("#chatList").addEventListener("scroll", () => closeMenu());
  window.addEventListener("resize", () => closeMenu());
  document.addEventListener("visibilitychange", syncViewing);
  $("#updatePill").addEventListener("click", installUpdate);
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && !$("#voice").hidden && $("#dialog").hidden) { e.preventDefault(); endVoiceChat(); return; }
    if (e.key === "Escape" && dict.mic) { e.preventDefault(); stopDictation(false); return; }
    if (e.key === "Escape" && !$("#viewer").hidden && $("#dialog").hidden) { e.preventDefault(); closeViewer(); return; }
    if (e.key === "Escape") closePicker();
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "n") { e.preventDefault(); newChat(); }
  });

  // Open links from answers in the real browser, never inside the app.
  document.addEventListener("click", (e) => {
    const a = e.target.closest("a[href]");
    if (!a) return;
    e.preventDefault();
    const href = a.getAttribute("href");
    if (/^https?:\/\//i.test(href)) window.runtime.BrowserOpenURL(href);
  });

  $("#hubForm").addEventListener("submit", (e) => {
    e.preventDefault();
    const q = $("#hubQuery").value.trim();
    if (q) searchHub(q);
  });

  $("#setupBtn").addEventListener("click", () => {
    $("#setupBtn").hidden = true;
    api().InstallEngine().catch((e) => { toast(e); $("#setupBtn").hidden = false; });
  });
}

async function start() {
  marked.use({ gfm: true, breaks: false });
  wireUI();
  wireWindow();
  wireEvents();

  const a = api();
  state.info = await a.GetAppInfo();
  state.settings = await a.GetSettings();
  state.voices = (await a.GetVoices().catch(() => [])) || [];
  state.think = !!state.settings.thinking;
  applyTheme();

  state.engine = await a.GetEngineStatus();
  if (state.engine.installed) document.body.dataset.ready = "1";
  renderSetup();

  const [sys, , upd, chats, status, gen] = await Promise.all([
    a.GetSystemInfo(), refreshDownloads(), a.GetUpdateStatus(), a.ListChats(), a.GetModelStatus(), a.IsGenerating(),
  ]);
  state.sys = sys;
  state.chats = chats || [];
  state.modelStatus = status;
  state.update = upd;
  renderUpdatePill();
  if (gen) state.gen = { chatId: gen, content: "", thinking: "", modelName: "" };

  await refreshModels();
  const last = state.settings.lastModel;
  if (last && state.installed.some((m) => m.id === last)) state.selected = last;
  renderPicker();

  renderChatList();
  syncViewing();
  $("#greeting").textContent = greetingText();
  renderThread();
  $("#viewChat").classList.add("rise");
  setTimeout(() => $("#viewChat").classList.remove("rise"), 2000);
  $("#input").focus();

  document.body.dataset.booted = "1";
  // Preload the last model so the first answer is quick.
  if (state.selected && state.engine.installed && status.state === "idle") a.LoadModel(state.selected).catch(() => {});
}

let bootTries = 0;
function boot() {
  if (window.go && window.go.backend && window.runtime) {
    start().catch((e) => toast(e));
    return;
  }
  // Opened straight from disk in a browser (not inside Phoenix): load the
  // fake backend from frontend/dev so the UI can be worked on. The app
  // itself never gets here, and frontend/dev isn't built into the exe.
  if (++bootTries === 40 && location.protocol === "file:") {
    document.head.append(h("script", { src: "../dev/mock.js" }));
  }
  setTimeout(boot, 30);
}
document.addEventListener("DOMContentLoaded", boot);
