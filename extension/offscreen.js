let currentConfig = {
  serverUrl: "http://127.0.0.1:8080",
  soundEnabled: true
};

let currentServerUrl = "";
let eventSource = null;
let reconnectTimer = null;

function safeRuntimeSendMessage(msg) {
  try {
    const p = chrome.runtime.sendMessage(msg);
    if (p && typeof p.catch === "function") {
      p.catch(() => {});
    }
  } catch (_) {
  }
}

function normalizeServerUrl(url) {
  url = String(url || "").trim();
  url = url.replace(/\/+$/, "");
  return url;
}

function sendStatus(status) {
  safeRuntimeSendMessage({ type: "status", status });
}

function closeSSE() {
  if (reconnectTimer) {
    clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }
  if (eventSource) {
    try {
      eventSource.close();
    } catch (_) {
    }
    eventSource = null;
  }
}

function scheduleReconnect() {
  if (reconnectTimer) {
    return;
  }
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null;
    connectSSE(currentServerUrl);
  }, 3000);
}

function connectSSE(serverUrl) {
  serverUrl = normalizeServerUrl(serverUrl);
  if (!serverUrl) {
    return;
  }

  if (serverUrl === currentServerUrl && eventSource) {
    return;
  }

  currentServerUrl = serverUrl;
  closeSSE();

  const url = `${serverUrl}/api/sse`;
  sendStatus("reconnecting");

  try {
    eventSource = new EventSource(url);
  } catch (_) {
    sendStatus("disconnected");
    scheduleReconnect();
    return;
  }

  eventSource.addEventListener("open", () => {
    sendStatus("connected");
  });

  eventSource.addEventListener("error", () => {
    if (!eventSource) {
      return;
    }
    if (eventSource.readyState === EventSource.CLOSED) {
      sendStatus("disconnected");
      scheduleReconnect();
      return;
    }
    sendStatus("reconnecting");
  });

  eventSource.addEventListener("signal", (ev) => {
    try {
      const sig = JSON.parse(ev.data);
      safeRuntimeSendMessage({ type: "signal", signal: sig });
    } catch (_) {
    }
  });
}

function playBeep() {
  try {
    const AudioCtx = window.AudioContext || window.webkitAudioContext;
    if (!AudioCtx) {
      return;
    }

    const ctx = new AudioCtx();
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();

    osc.type = "sine";
    osc.frequency.value = 880;
    gain.gain.value = 0.06;

    osc.connect(gain);
    gain.connect(ctx.destination);

    const now = ctx.currentTime;
    osc.start(now);
    osc.stop(now + 0.18);

    setTimeout(() => {
      try {
        ctx.close();
      } catch (_) {
      }
    }, 500);
  } catch (_) {
  }
}

chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (!msg || typeof msg.type !== "string") {
    sendResponse({ ok: false });
    return;
  }

  if (msg.type === "config") {
    if (msg.config && typeof msg.config === "object") {
      currentConfig = {
        serverUrl: normalizeServerUrl(msg.config.serverUrl || currentConfig.serverUrl),
        soundEnabled:
          typeof msg.config.soundEnabled === "boolean" ? msg.config.soundEnabled : currentConfig.soundEnabled
      };
      connectSSE(currentConfig.serverUrl);
    }
    sendResponse({ ok: true });
    return;
  }

  if (msg.type === "play_sound") {
    playBeep();
    sendResponse({ ok: true });
    return;
  }

  sendResponse({ ok: false });
});

safeRuntimeSendMessage({ type: "offscreen_ready" });
connectSSE(currentConfig.serverUrl);
