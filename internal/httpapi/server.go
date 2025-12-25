package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"example.com/binance-pivot-monitor/internal/pivot"
	signalpkg "example.com/binance-pivot-monitor/internal/signal"
	"example.com/binance-pivot-monitor/internal/sse"
)

type Server struct {
	Broker         *sse.Broker[signalpkg.Signal]
	History        *signalpkg.History
	AllowedOrigins []string
	PivotStatus    PivotStatusProvider
}

func New(broker *sse.Broker[signalpkg.Signal], history *signalpkg.History, allowedOrigins []string) *Server {
	return &Server{Broker: broker, History: history, AllowedOrigins: allowedOrigins}
}

type PivotStatusProvider interface {
	PivotStatus() pivot.PivotStatusResponse
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleDashboard)
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/sse", s.handleSSE)
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/pivot-status", s.handlePivotStatus)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	return s.cors(mux)
}

func (s *Server) handlePivotStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if s.PivotStatus == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	resp := s.PivotStatus.PivotStatus()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if s.History == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	q := r.URL.Query()
	getFirstCI := func(key string) string {
		if v := q.Get(key); v != "" {
			return v
		}
		for k, vs := range q {
			if strings.EqualFold(k, key) && len(vs) > 0 {
				return vs[0]
			}
		}
		return ""
	}
	getAllCI := func(key string) string {
		var all []string
		for k, vs := range q {
			if strings.EqualFold(k, key) {
				all = append(all, vs...)
			}
		}
		return strings.Join(all, ",")
	}

	symbol := getFirstCI("symbol")
	period := getFirstCI("period")
	level := getAllCI("level")
	if level == "" {
		level = getAllCI("levels")
	}
	direction := getFirstCI("direction")
	source := getFirstCI("source")
	limitStr := getFirstCI("limit")
	limit := 200
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil {
			limit = v
		}
	}

	res := s.History.Query(symbol, period, level, direction, source, limit)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(dashboardHTML))
}

const dashboardHTML = `<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover, user-scalable=no" />
    <meta name="apple-mobile-web-app-capable" content="yes" />
    <meta name="apple-mobile-web-app-status-bar-style" content="default" />
    <meta name="apple-mobile-web-app-title" content="Pivot Monitor" />
    <meta name="theme-color" content="#f6f7fb" />
    <meta name="format-detection" content="telephone=no" />
    <link rel="apple-touch-icon" href="/static/apple-touch-icon.png" />
    <link rel="icon" type="image/png" sizes="32x32" href="/static/favicon-32.png" />
    <link rel="icon" type="image/png" sizes="16x16" href="/static/favicon-16.png" />
    <link rel="icon" type="image/x-icon" href="/static/favicon.ico" />
    <title>Pivot Monitor</title>
    <style>
      :root{--bg:#f6f7fb;--card:#fff;--text:#0f172a;--muted:#64748b;--border:rgba(15,23,42,.08);--shadow:0 1px 2px rgba(15,23,42,.08),0 8px 24px rgba(15,23,42,.08);--up:#16a34a;--down:#dc2626;--pill:#e2e8f0;--pillText:#334155;--connectedBg:#e8f5e9;--connectedText:#1b5e20;--reconnectingBg:#fff8e1;--reconnectingText:#8d6e63;--disconnectedBg:#ffebee;--disconnectedText:#b71c1c;--staleBg:#ffebee;--staleText:#b71c1c;--freshBg:#e8f5e9;--freshText:#1b5e20}
      *{-webkit-tap-highlight-color:transparent;-webkit-touch-callout:none}
      body{margin:0;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Arial,sans-serif;background:var(--bg);color:var(--text);-webkit-font-smoothing:antialiased;padding-bottom:env(safe-area-inset-bottom)}
      .wrap{max-width:860px;margin:0 auto;padding:14px;padding-top:max(14px,env(safe-area-inset-top))}
      header{display:flex;justify-content:space-between;align-items:center;gap:12px;padding:14px;border:1px solid var(--border);border-radius:14px;background:var(--card);box-shadow:var(--shadow)}
      .title{font-weight:800;letter-spacing:.2px}
      .pill{font-size:12px;padding:4px 10px;border-radius:999px;background:var(--pill);color:var(--pillText);border:1px solid var(--border)}
      .pill.connected{background:var(--connectedBg);color:var(--connectedText)}
      .pill.reconnecting{background:var(--reconnectingBg);color:var(--reconnectingText)}
      .pill.disconnected{background:var(--disconnectedBg);color:var(--disconnectedText)}
      .pill.stale{background:var(--staleBg);color:var(--staleText)}
      .pill.fresh{background:var(--freshBg);color:var(--freshText)}
      .controls{margin-top:12px;display:flex;flex-direction:column;gap:10px}
      .row{display:flex;gap:10px;flex-wrap:wrap;align-items:center}
      input,select,button{font-size:14px}
      input,select{padding:10px 12px;border:1px solid var(--border);border-radius:12px;background:var(--card);outline:none}
      button{padding:10px 12px;border:1px solid var(--border);border-radius:12px;background:var(--card);cursor:pointer}
      button:hover{filter:brightness(.98)}
      button.active{background:#3b82f6;color:#fff;border-color:#3b82f6}
      .level-btns{display:flex;gap:6px;flex-wrap:wrap}
      .level-btns button{padding:6px 12px;font-size:13px}
      .hint{margin-top:10px;font-size:12px;color:var(--muted)}
      .pivot-status{margin-top:12px;padding:12px;border:1px solid var(--border);border-radius:12px;background:var(--card);font-size:13px}
      .pivot-status .row{justify-content:space-between}
      .list{margin-top:12px;display:flex;flex-direction:column;gap:10px}
      .item{background:var(--card);border:1px solid var(--border);border-radius:14px;padding:12px;box-shadow:0 1px 2px rgba(15,23,42,.06)}
      .top{display:flex;justify-content:space-between;gap:10px;align-items:baseline}
      .sym{font-weight:800;letter-spacing:.2px}
      .tags{display:flex;gap:6px;flex-wrap:wrap;justify-content:flex-end}
      .tag{font-size:12px;padding:2px 8px;border-radius:999px;border:1px solid var(--border);background:#f8fafc;color:#334155}
      .tag.up{background:rgba(22,163,74,.12);color:var(--up)}
      .tag.down{background:rgba(220,38,38,.12);color:var(--down)}
      .sub{margin-top:8px;display:flex;justify-content:space-between;gap:10px;font-size:13px;color:#334155}
      .muted{color:var(--muted)}
      .sound-toggle{display:flex;align-items:center;gap:6px}
      .sound-toggle input{width:18px;height:18px}
      @media(max-width:520px){.controls{grid-template-columns:1fr}}
    </style>
  </head>
  <body>
    <div class="wrap">
      <header>
        <div class="title">Pivot Monitor</div>
        <div class="row">
          <span id="status" class="pill">disconnected</span>
          <button id="refresh" type="button">Refresh</button>
        </div>
      </header>
      <div class="pivot-status">
        <div class="row">
          <div><strong>Daily:</strong> <span id="dailyStatus">-</span></div>
          <div><strong>Weekly:</strong> <span id="weeklyStatus">-</span></div>
        </div>
      </div>
      <div class="controls">
        <div class="row">
          <input id="symbol" type="text" placeholder="Symbol (e.g. BTC)" style="flex:1;min-width:120px" />
          <select id="period"><option value="">All periods</option><option value="1d">1d</option><option value="1w">1w</option></select>
          <select id="direction"><option value="">All directions</option><option value="up">up</option><option value="down">down</option></select>
        </div>
        <div class="row">
          <span style="font-size:13px">Levels:</span>
          <div class="level-btns" id="filterLevels">
            <button type="button" data-level="R3">R3</button>
            <button type="button" data-level="R4">R4</button>
            <button type="button" data-level="R5">R5</button>
            <button type="button" data-level="S3">S3</button>
            <button type="button" data-level="S4">S4</button>
            <button type="button" data-level="S5">S5</button>
          </div>
        </div>
        <div class="row">
          <span style="font-size:13px">Sound alerts:</span>
          <div class="level-btns" id="soundLevels">
            <button type="button" data-level="R3">R3</button>
            <button type="button" data-level="R4">R4</button>
            <button type="button" data-level="R5">R5</button>
            <button type="button" data-level="S3">S3</button>
            <button type="button" data-level="S4">S4</button>
            <button type="button" data-level="S5">S5</button>
          </div>
          <label class="sound-toggle"><input type="checkbox" id="soundEnabled" checked /> Sound</label>
        </div>
      </div>
      <div id="hint" class="hint"></div>
      <div id="list" class="list"></div>
    </div>
    <script>
      const $=id=>document.getElementById(id);
      const fmtRelTime=v=>{try{const d=new Date(v);if(isNaN(d))return String(v);const now=Date.now(),diff=Math.floor((now-d.getTime())/1000);if(diff<0)return"just now";if(diff<60)return diff+"s ago";if(diff<3600)return Math.floor(diff/60)+"m ago";if(diff<28800)return Math.floor(diff/3600)+"h "+Math.floor((diff%3600)/60)+"m ago";return d.toLocaleString("zh-CN",{month:"2-digit",day:"2-digit",hour:"2-digit",minute:"2-digit",hour12:false})}catch(_){return String(v)}};
      const fmtPrice=v=>{if(typeof v==="number"){const a=Math.abs(v);return a>=1000?v.toFixed(2):a>=1?v.toFixed(4):v.toPrecision(6)}return String(v)};
      const fmtDur=s=>{if(s<0)return"now";const h=Math.floor(s/3600),m=Math.floor((s%3600)/60);return h>0?h+"h "+m+"m":m+"m"};
      const setStatus=s=>{const e=$("status");e.textContent=s||"unknown";e.classList.remove("connected","reconnecting","disconnected");if(s)e.classList.add(s)};
      let selectedLevels=new Set(),soundLevels=new Set(["R4","R5","S4","S5"]);
      const STORAGE_KEY_SOUND_LEVELS="pivot_sound_levels";
      const STORAGE_KEY_SOUND_ENABLED="pivot_sound_enabled";
      function loadSoundSettings(){
        try{
          const saved=localStorage.getItem(STORAGE_KEY_SOUND_LEVELS);
          if(saved){soundLevels=new Set(JSON.parse(saved))}
          const enabled=localStorage.getItem(STORAGE_KEY_SOUND_ENABLED);
          if(enabled!==null){$("soundEnabled").checked=enabled==="true"}
        }catch(_){}
      }
      function saveSoundSettings(){
        try{
          localStorage.setItem(STORAGE_KEY_SOUND_LEVELS,JSON.stringify(Array.from(soundLevels)));
          localStorage.setItem(STORAGE_KEY_SOUND_ENABLED,$("soundEnabled").checked);
        }catch(_){}
      }
      function updateSoundLevelBtns(){
        document.querySelectorAll("#soundLevels button").forEach(b=>{
          if(soundLevels.has(b.dataset.level)){b.classList.add("active")}else{b.classList.remove("active")}
        });
      }
      function setupLevelBtns(){
        document.querySelectorAll("#filterLevels button").forEach(b=>{
          b.addEventListener("click",()=>{const l=b.dataset.level;if(selectedLevels.has(l)){selectedLevels.delete(l);b.classList.remove("active")}else{selectedLevels.add(l);b.classList.add("active")}loadHistory()});
        });
        document.querySelectorAll("#soundLevels button").forEach(b=>{
          b.addEventListener("click",()=>{const l=b.dataset.level;if(soundLevels.has(l)){soundLevels.delete(l);b.classList.remove("active")}else{soundLevels.add(l);b.classList.add("active")}saveSoundSettings()});
        });
        $("soundEnabled").addEventListener("change",saveSoundSettings);
      }
      const filters=()=>({symbol:$("symbol").value.trim(),period:$("period").value,levels:Array.from(selectedLevels),direction:$("direction").value,limit:400});
      const matchF=(s,f)=>{if(!s)return false;if(f.symbol&&!String(s.symbol||"").toUpperCase().includes(f.symbol.toUpperCase()))return false;if(f.period&&s.period!==f.period)return false;if(f.levels.length&&!f.levels.includes(s.level))return false;if(f.direction&&s.direction!==f.direction)return false;return true};
      const buildQ=f=>{const q=new URLSearchParams();if(f.symbol)q.set("symbol",f.symbol);if(f.period)q.set("period",f.period);f.levels.forEach(l=>q.append("level",l));if(f.direction)q.set("direction",f.direction);q.set("limit",f.limit);return q.toString()};
      const sortD=l=>l.sort((a,b)=>(new Date(b.triggered_at)||0)-(new Date(a.triggered_at)||0));
      const merge=(b,i)=>{const m=new Map();(b||[]).forEach(s=>s&&s.id&&m.set(s.id,s));(i||[]).forEach(s=>s&&s.id&&m.set(s.id,s));const o=Array.from(m.values());sortD(o);return o};
      const render=l=>{const e=$("list");e.innerHTML="";if(!l||!l.length){e.innerHTML='<div class="item">No signals</div>';return}l.forEach(s=>{const i=document.createElement("div");i.className="item";i.dataset.time=s.triggered_at;i.innerHTML='<div class="top"><div class="sym">'+s.symbol+'</div><div class="tags"><span class="tag">'+s.period+'</span><span class="tag">'+s.level+'</span><span class="tag '+s.direction+'">'+s.direction+'</span></div></div><div class="sub"><div>'+fmtPrice(s.price)+'</div><div class="muted time-rel">'+fmtRelTime(s.triggered_at)+'</div></div>';e.appendChild(i)})};
      function updateRelTimes(){document.querySelectorAll(".time-rel").forEach(el=>{const item=el.closest(".item");if(item&&item.dataset.time)el.textContent=fmtRelTime(item.dataset.time)})}
      let all=[];
      async function loadHistory(){const f=filters();$("hint").textContent="Loading...";try{const r=await fetch("/api/history?"+buildQ(f));if(!r.ok)throw new Error("http "+r.status);all=merge([],await r.json());render(all);$("hint").textContent="Signals: "+all.length}catch(e){$("hint").textContent="Load failed: "+e}}
      async function loadPivotStatus(){try{const r=await fetch("/api/pivot-status");if(!r.ok)return;const d=await r.json();const fmt=p=>p?((p.is_stale?'<span class="pill stale">STALE</span>':'<span class="pill fresh">OK</span>')+" "+fmtDur(p.seconds_until)+" ("+p.symbol_count+" symbols)"):"-";$("dailyStatus").innerHTML=fmt(d.daily);$("weeklyStatus").innerHTML=fmt(d.weekly)}catch(_){}}
      const playBeep=()=>{if(!$("soundEnabled").checked)return;try{const c=new(window.AudioContext||window.webkitAudioContext)(),o=c.createOscillator(),g=c.createGain();o.type="sine";o.frequency.value=880;g.gain.value=.08;o.connect(g);g.connect(c.destination);o.start();o.stop(c.currentTime+.15);setTimeout(()=>c.close(),500)}catch(_){}};
      function connectSSE(){let es;try{es=new EventSource("/api/sse")}catch(_){setStatus("disconnected");return}setStatus("reconnecting");es.onopen=()=>setStatus("connected");es.onerror=()=>setStatus(es.readyState===2?"disconnected":"reconnecting");es.addEventListener("signal",e=>{try{const s=JSON.parse(e.data);if(soundLevels.has(s.level))playBeep();if(matchF(s,filters())){all=merge(all,[s]);render(all);$("hint").textContent="Signals: "+all.length}}catch(_){}})}
      const debounce=(fn,ms)=>{let t;return()=>{clearTimeout(t);t=setTimeout(fn,ms)}};
      const dr=debounce(loadHistory,300);
      $("refresh").onclick=()=>{loadHistory();loadPivotStatus()};
      $("symbol").oninput=dr;
      $("period").onchange=loadHistory;
      $("direction").onchange=loadHistory;
      setupLevelBtns();loadSoundSettings();updateSoundLevelBtns();connectSSE();loadHistory();loadPivotStatus();setInterval(loadPivotStatus,60000);setInterval(updateRelTimes,10000);
    </script>
  </body>
</html>
`

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if s.Broker == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := s.Broker.Subscribe(256)
	defer s.Broker.Unsubscribe(ch)

	_, _ = fmt.Fprintf(w, ": connected %s\n\n", time.Now().UTC().Format(time.RFC3339))
	flusher.Flush()

	keepAlive := time.NewTicker(15 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepAlive.C:
			_, _ = fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case sig, ok := <-ch:
			if !ok {
				return
			}
			b, err := json.Marshal(sig)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "event: signal\n")
			_, _ = fmt.Fprintf(w, "data: %s\n\n", strings.ReplaceAll(string(b), "\n", ""))
			flusher.Flush()
		}
	}
}

func ParseAllowedOrigins(v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return []string{"*"}
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}

func (s *Server) cors(next http.Handler) http.Handler {
	allowed := s.AllowedOrigins
	if len(allowed) == 0 {
		allowed = []string{"*"}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		allowOrigin := ""
		for _, o := range allowed {
			if o == "*" {
				allowOrigin = "*"
				break
			}
			if o == origin {
				allowOrigin = origin
				break
			}
		}

		if allowOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
