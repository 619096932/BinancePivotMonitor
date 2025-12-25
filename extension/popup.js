function $(id) {
  return document.getElementById(id);
}

function getFromStorage(key) {
  return new Promise((resolve) => {
    chrome.storage.local.get([key], (res) => resolve(res[key]));
  });
}

function normalizeServerUrl(url) {
  url = String(url || "").trim();
  url = url.replace(/\/+$/, "");
  return url;
}

function fmtTime(v) {
  try {
    const d = new Date(v);
    if (Number.isNaN(d.getTime())) return String(v);
    const now = Date.now();
    const diff = Math.floor((now - d.getTime()) / 1000);
    if (diff < 0) return "just now";
    if (diff < 60) return diff + "s ago";
    if (diff < 3600) return Math.floor(diff / 60) + "m ago";
    if (diff < 28800) return Math.floor(diff / 3600) + "h " + Math.floor((diff % 3600) / 60) + "m ago";
    return d.toLocaleString();
  } catch (_) {
    return String(v);
  }
}

function fmtPrice(v) {
  if (typeof v === "number") {
    const abs = Math.abs(v);
    if (abs >= 1000) return v.toFixed(2);
    if (abs >= 1) return v.toFixed(4);
    return v.toPrecision(6);
  }
  return String(v);
}

function setStatus(status) {
  const el = $("status");
  el.textContent = status || "unknown";
  el.classList.remove("connected", "reconnecting", "disconnected");
  if (status === "connected") el.classList.add("connected");
  if (status === "reconnecting") el.classList.add("reconnecting");
  if (status === "disconnected") el.classList.add("disconnected");
}

let selectedFilterLevels = new Set();
let selectedSoundLevels = new Set(["R4", "R5", "S4", "S5"]);

function getFilters() {
  return {
    symbol: String($("search").value || "").trim(),
    period: String($("period")?.value || "").trim(),
    direction: String($("direction")?.value || "").trim(),
    filterLevels: Array.from(selectedFilterLevels)
  };
}

function matchesFilters(s, f) {
  if (!s || typeof s !== "object") return false;
  if (f.symbol) {
    const sym = String(s.symbol || "").toUpperCase();
    if (!sym.includes(f.symbol.toUpperCase())) return false;
  }
  if (f.period && String(s.period || "") !== f.period) return false;
  if (f.filterLevels && f.filterLevels.length > 0) {
    if (!f.filterLevels.includes(String(s.level || ""))) return false;
  }
  if (f.direction && String(s.direction || "") !== f.direction) return false;
  return true;
}

function sortByTimeDesc(list) {
  list.sort((a, b) => {
    const ta = new Date(a.triggered_at).getTime();
    const tb = new Date(b.triggered_at).getTime();
    return (tb || 0) - (ta || 0);
  });
}

function mergeSignals(base, incoming) {
  const m = new Map();
  for (const s of base || []) {
    if (s && s.id) {
      m.set(String(s.id), s);
    }
  }
  for (const s of incoming || []) {
    if (s && s.id) {
      m.set(String(s.id), s);
    }
  }
  const out = Array.from(m.values());
  sortByTimeDesc(out);
  return out;
}

function debounce(fn, ms) {
  let t;
  return (...args) => {
    clearTimeout(t);
    t = setTimeout(() => fn(...args), ms);
  };
}

function render(signals) {
  const list = $("list");
  list.innerHTML = "";

  const f = getFilters();
  const filtered = Array.isArray(signals) ? signals.filter((s) => matchesFilters(s, f)) : [];

  if (filtered.length === 0) {
    const div = document.createElement("div");
    div.className = "info";
    div.textContent = "No signals";
    list.appendChild(div);
    return;
  }

  for (const s of filtered) {
    const item = document.createElement("div");
    item.className = "item";

    const row = document.createElement("div");
    row.className = "row";

    const sym = document.createElement("div");
    sym.className = "symbol";
    sym.textContent = String(s.symbol || "");

    const tags = document.createElement("div");
    tags.className = "tags";

    const tagP = document.createElement("span");
    tagP.className = "tag";
    tagP.textContent = String(s.period || "");

    const tagL = document.createElement("span");
    tagL.className = "tag";
    tagL.textContent = String(s.level || "");

    const tagD = document.createElement("span");
    tagD.className = `tag dir ${String(s.direction || "")}`;
    tagD.textContent = String(s.direction || "");

    tags.appendChild(tagP);
    tags.appendChild(tagL);
    tags.appendChild(tagD);

    row.appendChild(sym);
    row.appendChild(tags);

    const sub = document.createElement("div");
    sub.className = "sub";

    const left = document.createElement("div");
    left.textContent = fmtPrice(s.price);

    const right = document.createElement("div");
    right.className = "muted";
    right.textContent = fmtTime(s.triggered_at);

    sub.appendChild(left);
    sub.appendChild(right);

    item.appendChild(row);
    item.appendChild(sub);

    item.addEventListener("click", () => {
      chrome.runtime.sendMessage({ type: "jump_tab", symbol: s.symbol }, () => { });
    });

    list.appendChild(item);
  }
}

let allSignals = [];

async function loadLocalState() {
  const cfg = (await getFromStorage("config")) || {};
  const status = (await getFromStorage("connectionStatus")) || "unknown";
  let signals = (await getFromStorage("signals")) || [];
  if (!Array.isArray(signals)) signals = [];
  return {
    config: {
      serverUrl: typeof cfg.serverUrl === "string" ? cfg.serverUrl : "",
      soundEnabled: typeof cfg.soundEnabled === "boolean" ? cfg.soundEnabled : true,
      filterLevels: Array.isArray(cfg.filterLevels) ? cfg.filterLevels : [],
      soundLevels: Array.isArray(cfg.soundLevels) ? cfg.soundLevels : ["R4", "R5", "S4", "S5"]
    },
    connectionStatus: status,
    signals
  };
}

async function fetchRemoteHistory(serverUrl, filters, limit) {
  serverUrl = normalizeServerUrl(serverUrl);
  if (!serverUrl) {
    return [];
  }
  const q = new URLSearchParams();
  if (filters.symbol) q.set("symbol", filters.symbol);
  if (filters.period) q.set("period", filters.period);
  if (filters.filterLevels && filters.filterLevels.length > 0) {
    filters.filterLevels.forEach(l => q.append("level", l));
  }
  if (filters.direction) q.set("direction", filters.direction);
  q.set("limit", String(limit || 200));

  const url = `${serverUrl}/api/history?${q.toString()}`;
  const res = await fetch(url, { cache: "no-store" });
  if (!res.ok) {
    throw new Error(`history http ${res.status}`);
  }
  const data = await res.json();
  return Array.isArray(data) ? data : [];
}

let currentServerUrl = "";

function setupLevelButtons() {
  document.querySelectorAll("#filterLevels button").forEach(btn => {
    btn.addEventListener("click", () => {
      const level = btn.dataset.level;
      if (selectedFilterLevels.has(level)) {
        selectedFilterLevels.delete(level);
        btn.classList.remove("active");
      } else {
        selectedFilterLevels.add(level);
        btn.classList.add("active");
      }
      render(allSignals);
    });
  });

  document.querySelectorAll("#soundLevels button").forEach(btn => {
    btn.addEventListener("click", () => {
      const level = btn.dataset.level;
      if (selectedSoundLevels.has(level)) {
        selectedSoundLevels.delete(level);
        btn.classList.remove("active");
      } else {
        selectedSoundLevels.add(level);
        btn.classList.add("active");
      }
      saveSoundConfig();
    });
  });

  $("soundEnabled").addEventListener("change", saveSoundConfig);
}

function updateLevelButtons() {
  document.querySelectorAll("#filterLevels button").forEach(btn => {
    if (selectedFilterLevels.has(btn.dataset.level)) {
      btn.classList.add("active");
    } else {
      btn.classList.remove("active");
    }
  });

  document.querySelectorAll("#soundLevels button").forEach(btn => {
    if (selectedSoundLevels.has(btn.dataset.level)) {
      btn.classList.add("active");
    } else {
      btn.classList.remove("active");
    }
  });
}

function saveSoundConfig() {
  const config = {
    soundEnabled: $("soundEnabled").checked,
    soundLevels: Array.from(selectedSoundLevels)
  };
  chrome.runtime.sendMessage({ type: "set_config", config }, () => { });
}

async function refresh() {
  const info = $("info");
  info.textContent = "Loading...";

  const f = getFilters();
  let local;
  try {
    local = await loadLocalState();
  } catch (_) {
    local = { config: { serverUrl: "", filterLevels: [], soundLevels: ["R4", "R5", "S4", "S5"] }, connectionStatus: "unknown", signals: [] };
  }

  currentServerUrl = local.config?.serverUrl || "";

  // 加载保存的配置
  if (Array.isArray(local.config?.filterLevels) && local.config.filterLevels.length > 0) {
    selectedFilterLevels = new Set(local.config.filterLevels);
  }
  if (Array.isArray(local.config?.soundLevels)) {
    selectedSoundLevels = new Set(local.config.soundLevels);
  }
  $("soundEnabled").checked = local.config?.soundEnabled !== false;
  updateLevelButtons();

  f.filterLevels = Array.from(selectedFilterLevels);

  setStatus(local.connectionStatus);
  info.textContent = `Server: ${currentServerUrl || ""} | local: ${local.signals?.length || 0}`;

  const localFiltered = Array.isArray(local.signals) ? local.signals.filter((s) => matchesFilters(s, f)) : [];
  allSignals = local.signals || [];
  render(allSignals);

  if (!currentServerUrl) {
    return;
  }

  try {
    const remote = await fetchRemoteHistory(currentServerUrl, f, 400);
    allSignals = mergeSignals(allSignals, remote);
    render(allSignals);
    info.textContent = `Server: ${currentServerUrl || ""} | total: ${allSignals.length}`;
  } catch (_) {
    info.textContent = `Server: ${currentServerUrl || ""} | remote: failed`;
  }
}

$("optionsBtn").addEventListener("click", () => {
  if (chrome.runtime.openOptionsPage) {
    chrome.runtime.openOptionsPage();
  }
});

$("popoutBtn").addEventListener("click", () => {
  // 在独立窗口中打开 popup
  const popupUrl = chrome.runtime.getURL("popup.html");
  chrome.windows.create({
    url: popupUrl,
    type: "popup",
    width: 420,
    height: 600,
    focused: true
  });
  // 关闭当前 popup
  window.close();
});

$("sidePanelBtn").addEventListener("click", async () => {
  // 打开 Side Panel
  try {
    // 获取当前窗口
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
    if (tab && tab.windowId) {
      await chrome.sidePanel.open({ windowId: tab.windowId });
    }
  } catch (e) {
    console.error("Failed to open side panel:", e);
  }
  window.close();
});

$("jumpBtn").addEventListener("click", () => {
  const searchSymbol = $("search").value.trim().toUpperCase();
  // 如果搜索框有内容，尝试跳转到该交易对
  const symbol = searchSymbol ? (searchSymbol.endsWith("USDT") ? searchSymbol : searchSymbol + "USDT") : "";
  chrome.runtime.sendMessage({ type: "jump_tab", symbol }, () => { });
});

$("refreshBtn").addEventListener("click", () => {
  refresh();
});

const debouncedRefresh = debounce(() => render(allSignals), 300);
$("search").addEventListener("input", () => {
  debouncedRefresh();
});

$("period").addEventListener("change", () => render(allSignals));
$("direction").addEventListener("change", () => render(allSignals));

chrome.runtime.onMessage.addListener((msg) => {
  if (!msg || typeof msg.type !== "string") {
    return;
  }
  if (msg.type === "status_update") {
    setStatus(msg.status);
  }
  if (msg.type === "signal_update") {
    const sig = msg.signal;
    allSignals = mergeSignals([sig], allSignals);
    if (allSignals.length > 500) {
      allSignals = allSignals.slice(0, 500);
    }
    render(allSignals);
  }
});

setupLevelButtons();
chrome.runtime.sendMessage({ type: "mark_read" }, () => { });
refresh();
