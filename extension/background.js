const DEFAULT_CONFIG = {
  serverUrl: "http://127.0.0.1:8080",
  soundEnabled: true,
  filterLevels: [],
  soundLevels: ["R4", "R5", "S4", "S5"]
};

const STORAGE_KEYS = {
  config: "config",
  signals: "signals",
  unread: "unread"
};

const MAX_SIGNALS = 500;

function getFromStorage(key) {
  return new Promise((resolve) => {
    chrome.storage.local.get([key], (res) => resolve(res[key]));
  });
}

function setInStorage(obj) {
  return new Promise((resolve) => {
    chrome.storage.local.set(obj, () => resolve());
  });
}

async function loadConfig() {
  const cfg = (await getFromStorage(STORAGE_KEYS.config)) || {};
  return {
    serverUrl: typeof cfg.serverUrl === "string" && cfg.serverUrl ? cfg.serverUrl : DEFAULT_CONFIG.serverUrl,
    soundEnabled: typeof cfg.soundEnabled === "boolean" ? cfg.soundEnabled : DEFAULT_CONFIG.soundEnabled,
    filterLevels: Array.isArray(cfg.filterLevels) ? cfg.filterLevels : DEFAULT_CONFIG.filterLevels,
    soundLevels: Array.isArray(cfg.soundLevels) ? cfg.soundLevels : DEFAULT_CONFIG.soundLevels
  };
}

async function saveConfig(cfg) {
  await setInStorage({ [STORAGE_KEYS.config]: cfg });
}

function normalizeServerUrl(url) {
  url = String(url || "").trim();
  url = url.replace(/\/+$/, "");
  return url;
}

function setBadge(unread) {
  const text = unread > 0 ? String(unread) : "";
  chrome.action.setBadgeText({ text });
  if (unread > 0) {
    chrome.action.setBadgeBackgroundColor({ color: "#d32f2f" });
  }
}

function safeRuntimeSendMessage(msg) {
  try {
    const p = chrome.runtime.sendMessage(msg);
    if (p && typeof p.catch === "function") {
      p.catch(() => { });
    }
  } catch (_) {
  }
}

async function ensureOffscreen() {
  if (!chrome.offscreen || !chrome.offscreen.createDocument) {
    return;
  }

  try {
    await chrome.offscreen.createDocument({
      url: "offscreen.html",
      reasons: ["AUDIO_PLAYBACK", "DOM_PARSER"],
      justification: "Use EventSource (DOM API) to maintain SSE connection and play alert sound"
    });
  } catch (_) {
  }
}

async function broadcastConfig() {
  const cfg = await loadConfig();
  safeRuntimeSendMessage({ type: "config", config: cfg });
}

async function bootstrap() {
  await ensureOffscreen();

  const unread = (await getFromStorage(STORAGE_KEYS.unread)) || 0;
  setBadge(unread);

  await broadcastConfig();
}

chrome.runtime.onInstalled.addListener(() => {
  bootstrap();
});

chrome.runtime.onStartup.addListener(() => {
  bootstrap();
});

chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  (async () => {
    if (!msg || typeof msg.type !== "string") {
      sendResponse({ ok: false });
      return;
    }

    if (msg.type === "offscreen_ready") {
      await broadcastConfig();
      sendResponse({ ok: true });
      return;
    }

    if (msg.type === "status") {
      await setInStorage({ connectionStatus: msg.status || "unknown" });
      safeRuntimeSendMessage({ type: "status_update", status: msg.status || "unknown" });
      sendResponse({ ok: true });
      return;
    }

    if (msg.type === "signal") {
      const sig = msg.signal;
      let signals = (await getFromStorage(STORAGE_KEYS.signals)) || [];
      if (!Array.isArray(signals)) signals = [];

      signals.unshift(sig);
      if (signals.length > MAX_SIGNALS) {
        signals = signals.slice(0, MAX_SIGNALS);
      }

      let unread = (await getFromStorage(STORAGE_KEYS.unread)) || 0;
      unread += 1;

      await setInStorage({ [STORAGE_KEYS.signals]: signals, [STORAGE_KEYS.unread]: unread });
      setBadge(unread);

      const cfg = await loadConfig();
      const sigLevel = String(sig.level || "");
      if (cfg.soundEnabled && cfg.soundLevels && cfg.soundLevels.includes(sigLevel)) {
        safeRuntimeSendMessage({ type: "play_sound" });
      }

      safeRuntimeSendMessage({ type: "signal_update", signal: sig });
      sendResponse({ ok: true });
      return;
    }

    if (msg.type === "get_state") {
      const cfg = await loadConfig();
      const unread = (await getFromStorage(STORAGE_KEYS.unread)) || 0;
      const status = (await getFromStorage("connectionStatus")) || "unknown";
      sendResponse({ ok: true, config: cfg, unread, connectionStatus: status });
      return;
    }

    if (msg.type === "get_signals") {
      const signals = (await getFromStorage(STORAGE_KEYS.signals)) || [];
      sendResponse({ ok: true, signals: Array.isArray(signals) ? signals : [] });
      return;
    }

    if (msg.type === "mark_read") {
      await setInStorage({ [STORAGE_KEYS.unread]: 0 });
      setBadge(0);
      sendResponse({ ok: true });
      return;
    }

    if (msg.type === "set_config") {
      const current = await loadConfig();
      const next = {
        serverUrl: normalizeServerUrl(msg.config?.serverUrl || current.serverUrl),
        soundEnabled: typeof msg.config?.soundEnabled === "boolean" ? msg.config.soundEnabled : current.soundEnabled,
        filterLevels: Array.isArray(msg.config?.filterLevels) ? msg.config.filterLevels : current.filterLevels,
        soundLevels: Array.isArray(msg.config?.soundLevels) ? msg.config.soundLevels : current.soundLevels
      };
      await saveConfig(next);
      await broadcastConfig();
      sendResponse({ ok: true, config: next });
      return;
    }

    if (msg.type === "jump_tab") {
      const symbol = msg.symbol || "";

      chrome.tabs.query({}, (tabs) => {
        // 优先查找 TradingView 或 Binance 页面
        let target = tabs.find((t) =>
          typeof t.url === "string" && (t.url.includes("tradingview.com") || t.url.includes("binance.com/futures"))
        );

        if (target && typeof target.id === "number") {
          // 如果有交易对，更新 URL 跳转到对应交易对
          if (symbol) {
            let newUrl = "";
            if (target.url.includes("tradingview.com")) {
              // TradingView: https://www.tradingview.com/chart/?symbol=BINANCE:BTCUSDT.P
              const baseSymbol = symbol.replace("USDT", "");
              newUrl = `https://www.tradingview.com/chart/?symbol=BINANCE:${baseSymbol}USDT.P`;
            } else if (target.url.includes("binance.com")) {
              // Binance Futures: https://www.binance.com/futures/BTCUSDT
              newUrl = `https://www.binance.com/futures/${symbol}`;
            }

            if (newUrl) {
              chrome.tabs.update(target.id, { url: newUrl, active: true }, () => {
                if (typeof target.windowId === "number") {
                  chrome.windows.update(target.windowId, { focused: true });
                }
                sendResponse({ ok: true, url: newUrl });
              });
              return;
            }
          }

          // 没有交易对，只切换到该标签页
          if (typeof target.windowId === "number") {
            chrome.windows.update(target.windowId, { focused: true });
          }
          chrome.tabs.update(target.id, { active: true }, () => sendResponse({ ok: true }));
        } else {
          // 没有找到现有页面，打开新标签页
          if (symbol) {
            const newUrl = `https://www.binance.com/futures/${symbol}`;
            chrome.tabs.create({ url: newUrl }, () => sendResponse({ ok: true, url: newUrl }));
          } else {
            chrome.tabs.create({ url: "https://www.binance.com/futures" }, () => sendResponse({ ok: true }));
          }
        }
      });
      return;
    }

    sendResponse({ ok: false });
  })();

  return true;
});

bootstrap();
