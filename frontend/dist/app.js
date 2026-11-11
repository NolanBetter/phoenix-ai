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
};

// ---------- limits ----------

const maxChats = () => state.info?.maxChats || 10;
const maxModels = () => state.info?.maxModels || 5;
const chatsFull = () => state.chats.length >= maxChats();
function modelSlotsUsed() {
  const ids = new Set(state.installed.map((m) => m.id));
  Object.keys(state.downloads).forEach((id) => ids.add(id));
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
  const [catalog, installed] = await Promise.all([api().GetCatalog(), api().GetInstalledModels()]);
  state.catalog = catalog || [];
  state.installed = installed || [];
  const before = state.selected;
  if (!state.installed.some((m) => m.id === state.selected)) {
    state.selected = state.installed.length ? state.installed[0].id : null;
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

function renderStarter() {
  const box = $("#starter");
  box.textContent = "";
  if (!state.installed.length) {
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
  box.append(h("div", { class: "suggestions" },
    SUGGESTIONS.map(([label, text]) => h("button", { class: "suggestion", onclick: () => {
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
    state.messages.forEach((m, i) => thread.append(messageEl(m, i === lastAssistant && !live && i === state.messages.length - 1)));
    if (live) thread.append(liveEl());
  }
  updateComposer();
  if (scrollToEnd) scrollEnd(true);
}

function markdownEl(text) {
  const div = h("div", { class: "answer" });
  div.innerHTML = DOMPurify.sanitize(marked.parse(text || ""), { USE_PROFILES: { html: true } });
  enhanceCode(div);
  return div;
}

function enhanceCode(root) {
  $$("pre > code", root).forEach((code) => {
    const pre = code.parentElement;
    if ($(".code-head", pre)) return;
    const lang = ([...code.classList].find((c) => c.startsWith("language-")) || "").slice(9) || "code";
    const btn = h("button", { class: "copy-code" }, "Copy");
    btn.addEventListener("click", () => copyText(code.textContent, btn));
    pre.prepend(h("div", { class: "code-head" }, h("span", {}, lang), btn));
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
}

function thinkingEl(text, live) {
  const d = h("details", { class: "thinking" + (live ? " live" : "") },
    h("summary", {}, ic("chevronRight"), live ? "Thinking…" : "Thought process"),
    h("div", { class: "thought" }, text),
  );
  if (live) d.open = true;
  return d;
}

function messageEl(m, isLast) {
  if (m.role === "user") {
    return h("div", { class: "msg user" }, h("div", { class: "bubble" }, m.content));
  }
  const wrap = h("div", { class: "msg assistant" });
  if (m.thinking) wrap.append(thinkingEl(m.thinking, false));
  if (m.content) wrap.append(markdownEl(m.content));
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
    const host = last.tagName === "PRE" ? md : last.tagName === "UL" || last.tagName === "OL" ? last.lastElementChild || last : last;
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
  btn.disabled = !busy && (!state.selected || blockedNew);
  $("#thinkToggle").setAttribute("aria-pressed", String(state.think));

  let hint = "Enter to send, Shift+Enter for a new line";
  if (!state.selected) hint = "Download a model to start chatting";
  else if (busy && state.gen.chatId !== state.chatId) hint = "Answering in another chat. Stop it to send here.";
  else if (blockedNew) hint = `Delete a chat to start a new one (${maxChats()} max)`;
  $("#composerHint").textContent = hint;
}

async function send() {
  if (state.gen) {
    api().StopGeneration();
    return;
  }
  const input = $("#input");
  const text = input.value.trim();
  if (!text) return;
  if (!state.selected) {
    toast("Download a model first.");
    setView("models");
    return;
  }
  if (!state.chatId && chatsFull()) {
    toast(`You have ${maxChats()} chats already. Delete one to start a new chat.`);
    return;
  }
  const isNew = !state.chatId;
  state.messages.push({ role: "user", content: text, time: Date.now() });
  state.pendingNew = isNew;
  input.value = "";
  autosize();
  renderThread(true);
  try {
    const id = await api().SendMessage(state.chatId || "", text, state.selected, state.think);
    if (!state.chatId) state.chatId = id;
    state.pendingNew = false;
    syncViewing();
    renderChatList();
  } catch (e) {
    state.messages.pop();
    state.pendingNew = false;
    input.value = text;
    autosize();
    renderThread();
    toast(e);
  }
}

async function retry() {
  if (!state.chatId || state.gen) return;
  if (!state.selected) return toast("Download a model first.");
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

// ---------- model picker & status ----------

function selectedModel() { return state.installed.find((m) => m.id === state.selected); }

function renderPicker() {
  const m = selectedModel();
  $("#pickerName").textContent = m ? m.name : state.installed.length ? "Choose a model" : "No models yet";
  const menu = $("#pickerMenu");
  menu.textContent = "";
  for (const im of state.installed) {
    menu.append(h("button", {
      class: "picker-opt", role: "option", "aria-selected": String(im.id === state.selected),
      onclick: () => pickModel(im.id),
    },
      h("span", { class: "n" }, im.name),
      h("span", { class: "sz" }, fmtBytes(im.size)),
      h("span", { class: "s" }, [im.params, im.quant, im.fit.label].filter(Boolean).join(", ")),
    ));
  }
  if (!state.installed.length) menu.append(h("div", { class: "note" }, "You haven't downloaded any models yet."));
  menu.append(h("button", { class: "picker-more", onclick: () => { closePicker(); setView("models"); } }, "Get more models"));
  renderModelState();
}

function pickModel(id) {
  closePicker();
  if (id === state.selected) return;
  state.selected = id;
  renderPicker();
  updateComposer();
  if (!state.gen) api().LoadModel(id).catch(toast);
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
  if (s.state === "ready") text = s.note || (s.onGpu ? "Running on your graphics card" : "Running on your processor");
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
    actions.append(h("button", { class: "btn btn-quiet", onclick: () => cancelDownload(d) }, d.status === "error" ? "Remove" : "Cancel"));
    const main = h("div", {},
      h("div", { class: "row-title" }, h("span", { class: "n" }, d.name), h("span", { class: "by" }, d.repo)),
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
      rows.append(h("div", { class: "row" },
        h("div", {},
          h("div", { class: "row-title" }, h("span", { class: "n" }, m.name), h("span", { class: "by" }, [m.maker, m.params].filter(Boolean).join(" "))),
          h("div", { class: "row-facts" }, heatEl(m.fit), h("span", {}, fmtBytes(m.size)), m.quant ? h("span", {}, m.quant) : null),
        ),
        h("div", { class: "row-side" },
          h("button", { class: "btn", onclick: () => { pickModel(m.id); newChat(); } }, "Chat"),
          h("button", { class: "btn btn-quiet danger", "aria-label": `Delete ${m.name}`, onclick: () => deleteModel(m) }, ic("trash"), "Delete"),
        ),
      ));
    }
    block.append(rows);
  }
  box.append(block);
}

function renderCatalogSection() {
  const box = $("#catalogSection");
  box.textContent = "";
  const rows = h("div", { class: "rows" });
  const full = modelsFull();
  for (const m of state.catalog) {
    const dl = Object.values(state.downloads).find((d) => d.catalogId === m.id && d.status !== "error");
    let side;
    if (m.installed) side = h("span", { class: "tag hot" }, "Downloaded");
    else if (dl) side = h("span", { class: "tag" }, dl.status === "paused" ? "Paused" : "Downloading");
    else side = h("button", {
      class: "btn" + (m.tags.includes("Recommended") && !full && m.fit.level !== "toobig" ? " btn-primary" : ""),
      disabled: full, title: full ? fullModelsText() : null,
      onclick: () => downloadCatalog(m.id),
    }, ic("download"), "Download");
    rows.append(h("div", { class: "row" },
      h("div", {},
        h("div", { class: "row-title" },
          h("span", { class: "n" }, m.name),
          h("span", { class: "by" }, `${m.maker} ${m.params}`),
          m.tags.map((t) => h("span", { class: "tag" + (t === "Recommended" ? " hot" : "") }, t)),
        ),
        h("div", { class: "row-desc" }, m.description),
        h("div", { class: "row-facts" }, heatEl(m.fit), h("span", {}, fmtBytes(m.size))),
      ),
      h("div", { class: "row-side" }, side),
    ));
  }
  box.append(h("section", { class: "block" },
    h("h2", {}, "Recommended"),
    h("p", { class: "block-sub" }, "Open models picked for quality. The heat bars show how well each one runs on your PC."),
    full ? h("p", { class: "limit-note" }, fullModelsText()) : null,
    rows,
  ));
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

async function downloadCatalog(id) {
  try { await call("DownloadModel", id); } catch {}
}
async function resumeDownload(d) {
  try { await call("DownloadFromHub", d.repo, d.file); } catch {}
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
      h("span", { class: "q", title: f.file }, f.quant || "file", f.recommended ? h("span", { class: "tag hot" }, "Best pick") : null),
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
      : null,
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

  on("chat:start", (e) => {
    if (state.pendingNew && !state.chatId) {
      state.chatId = e.chatId;
      syncViewing();
    }
    state.gen = { chatId: e.chatId, content: "", thinking: "", modelName: e.modelName };
    if (e.chatId === state.chatId) renderThread(true);
    updateComposer();
    renderChatList();
  });

  on("chat:delta", (d) => {
    if (!state.gen || state.gen.chatId !== d.chatId) return;
    if (d.thinking) state.gen.thinking += d.thinking;
    if (d.content) state.gen.content += d.content;
    if (d.chatId === state.chatId) scheduleLive();
  });

  on("chat:done", (d) => {
    const onScreen = d.chatId === state.chatId && state.view === "chat";
    state.gen = null;
    if (d.chatId === state.chatId) {
      state.messages.push(d.message);
      const stick = nearBottom();
      renderThread();
      if (stick) scrollEnd(true);
    }
    if (!onScreen && !d.message.stopped) {
      const c = state.chats.find((x) => x.id === d.chatId);
      if (c) toast(`New answer in “${c.title}”. Click to open it.`, "info", 6000, () => openChat(c.id));
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
  wireEvents();

  const a = api();
  state.info = await a.GetAppInfo();
  state.settings = await a.GetSettings();
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
