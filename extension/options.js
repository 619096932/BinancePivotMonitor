function $(id) {
  return document.getElementById(id);
}

function normalizeServerUrl(url) {
  url = String(url || "").trim();
  url = url.replace(/\/+$/, "");
  return url;
}

function setStatus(text) {
  $("status").textContent = text || "";
}

let filterLevels = new Set();
let soundLevels = new Set(["R4", "R5", "S4", "S5"]);

function setupLevelButtons() {
  document.querySelectorAll("#filterLevels button").forEach(btn => {
    btn.addEventListener("click", () => {
      const level = btn.dataset.level;
      if (filterLevels.has(level)) {
        filterLevels.delete(level);
        btn.classList.remove("active");
      } else {
        filterLevels.add(level);
        btn.classList.add("active");
      }
    });
  });

  document.querySelectorAll("#soundLevels button").forEach(btn => {
    btn.addEventListener("click", () => {
      const level = btn.dataset.level;
      if (soundLevels.has(level)) {
        soundLevels.delete(level);
        btn.classList.remove("active");
      } else {
        soundLevels.add(level);
        btn.classList.add("active");
      }
    });
  });
}

function updateLevelButtons() {
  document.querySelectorAll("#filterLevels button").forEach(btn => {
    if (filterLevels.has(btn.dataset.level)) {
      btn.classList.add("active");
    } else {
      btn.classList.remove("active");
    }
  });

  document.querySelectorAll("#soundLevels button").forEach(btn => {
    if (soundLevels.has(btn.dataset.level)) {
      btn.classList.add("active");
    } else {
      btn.classList.remove("active");
    }
  });
}

function load() {
  chrome.runtime.sendMessage({ type: "get_state" }, (res) => {
    if (!res || !res.ok) {
      setStatus("Failed to load");
      return;
    }
    $("serverUrl").value = res.config?.serverUrl || "";
    $("sound").checked = !!res.config?.soundEnabled;

    if (Array.isArray(res.config?.filterLevels)) {
      filterLevels = new Set(res.config.filterLevels);
    }
    if (Array.isArray(res.config?.soundLevels)) {
      soundLevels = new Set(res.config.soundLevels);
    }

    updateLevelButtons();
    setStatus(`Connection: ${res.connectionStatus || "unknown"}`);
  });
}

$("save").addEventListener("click", () => {
  const next = {
    serverUrl: normalizeServerUrl($("serverUrl").value),
    soundEnabled: $("sound").checked,
    filterLevels: Array.from(filterLevels),
    soundLevels: Array.from(soundLevels)
  };

  chrome.runtime.sendMessage({ type: "set_config", config: next }, (res) => {
    if (!res || !res.ok) {
      setStatus("Save failed");
      return;
    }
    setStatus("Saved");
    $("serverUrl").value = res.config?.serverUrl || next.serverUrl;
    $("sound").checked = !!res.config?.soundEnabled;
  });
});

chrome.runtime.onMessage.addListener((msg) => {
  if (!msg || typeof msg.type !== "string") return;
  if (msg.type === "status_update") {
    setStatus(`Connection: ${msg.status || "unknown"}`);
  }
});

setupLevelButtons();
load();
