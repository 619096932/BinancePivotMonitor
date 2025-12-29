// Pivot Monitor Frontend v2.0 - Virtual List + Local Filtering
(function () {
    'use strict';

    const $ = id => document.getElementById(id);

    // ==================== 格式化函数 ====================
    const fmtRelTime = v => {
        try {
            const d = new Date(v);
            if (isNaN(d)) return String(v);
            const now = Date.now(), diff = Math.floor((now - d.getTime()) / 1000);
            if (diff < 0) return "just now";
            if (diff < 60) return diff + "s ago";
            if (diff < 3600) return Math.floor(diff / 60) + "m ago";
            if (diff < 86400) return Math.floor(diff / 3600) + "h " + Math.floor((diff % 3600) / 60) + "m ago";
            const days = Math.floor(diff / 86400);
            const hours = Math.floor((diff % 86400) / 3600);
            return days + "d " + hours + "h ago";
        } catch (_) { return String(v); }
    };

    const fmtPrice = v => {
        if (typeof v === "number") {
            const a = Math.abs(v);
            return a >= 1000 ? v.toFixed(2) : a >= 1 ? v.toFixed(4) : v.toPrecision(6);
        }
        return String(v);
    };

    const fmtPct = v => {
        if (typeof v !== "number") return "";
        const sign = v >= 0 ? "+" : "";
        return sign + v.toFixed(2) + "%";
    };

    const fmtTradeCount = n => {
        if (typeof n !== "number") return "";
        if (n >= 1e6) return (n / 1e6).toFixed(1) + "M";
        if (n >= 1e3) return (n / 1e3).toFixed(1) + "K";
        return String(n);
    };

    const fmtVolume = v => {
        if (typeof v !== "number") return "";
        if (v >= 1e9) return "$" + (v / 1e9).toFixed(2) + "B";
        if (v >= 1e6) return "$" + (v / 1e6).toFixed(1) + "M";
        if (v >= 1e3) return "$" + (v / 1e3).toFixed(0) + "K";
        return "$" + v.toFixed(0);
    };

    const fmtDur = s => {
        if (s < 0) return "now";
        const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60);
        return h > 0 ? h + "h " + m + "m" : m + "m";
    };

    // 效率等级颜色
    const getEfficiencyColor = rank => {
        if (!rank) return '#6b7280';
        const first = rank[0];
        return {
            'A': '#22c55e',  // 绿色 - 高效
            'B': '#84cc16',  // 黄绿 - 较高效
            'C': '#eab308',  // 黄色 - 中等
            'D': '#f97316',  // 橙色 - 较低
            'E': '#ef4444',  // 红色 - 低效
            'F': '#ef4444',
            'G': '#6b7280',
            'J': '#6b7280'
        }[first] || '#6b7280';
    };

    // 方向颜色
    const getDirectionColor = dir => {
        return dir === 'bullish' ? '#22c55e' : dir === 'bearish' ? '#ef4444' : '#6b7280';
    };

    // 格式化时间
    const fmtTime = v => {
        try {
            const d = new Date(v);
            if (isNaN(d)) return String(v);
            return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
        } catch (_) { return String(v); }
    };

    // ==================== 状态管理 ====================
    let masterSignals = [];      // 主数据（从后端加载）
    let filteredSignals = [];    // 过滤后数据（前端计算）
    let masterPatterns = [];     // 形态信号数据
    let filteredPatterns = [];   // 过滤后形态数据
    let tickerData = new Map();  // Ticker 数据
    let symbolRanking = { volume: new Map(), trades: new Map() };

    // 枢轴点缓存 (5分钟过期)
    const pivotCache = new Map(); // symbol -> { data, timestamp }
    const PIVOT_CACHE_TTL = 5 * 60 * 1000; // 5 minutes

    let selectedLevels = new Set();
    let soundLevels = new Set(["R4", "R5", "S4", "S5"]);
    let currentView = 'signals';
    let menuSymbol = null;
    let menuFromRanking = false;
    let currentPatternSignal = null; // 当前显示详情的形态信号
    let currentPivotPreviewSymbol = null; // 当前预览的交易对
    let currentPivotPreviewPeriod = '1d'; // 当前预览的周期

    // Clusterize 实例
    let signalCluster = null;
    let patternCluster = null;
    let rankingCluster = null;

    // localStorage keys
    const STORAGE_KEYS = {
        soundLevels: "pivot_sound_levels",
        soundEnabled: "pivot_sound_enabled",
        limit: "pivot_limit",
        minDiff: "pivot_min_diff",
        minVolume: "pivot_min_volume",
        volumeUnit: "pivot_volume_unit",
        filterLevels: "pivot_filter_levels",
        filterPeriod: "pivot_filter_period",
        filterDirection: "pivot_filter_direction"
    };

    // ==================== 工具函数 ====================
    const setStatus = s => {
        const e = $("status");
        e.textContent = s || "unknown";
        e.classList.remove("connected", "reconnecting", "disconnected");
        if (s) e.classList.add(s);
    };

    const showToast = (msg, duration = 2000) => {
        const t = $("toast");
        t.textContent = msg;
        t.classList.add("show");
        setTimeout(() => t.classList.remove("show"), duration);
    };

    const debounce = (fn, ms) => { let t; return (...args) => { clearTimeout(t); t = setTimeout(() => fn(...args), ms); }; };

    // ==================== 设置持久化 ====================
    function loadSettings() {
        try {
            const saved = localStorage.getItem(STORAGE_KEYS.soundLevels);
            if (saved) soundLevels = new Set(JSON.parse(saved));

            const enabled = localStorage.getItem(STORAGE_KEYS.soundEnabled);
            if (enabled !== null) $("soundEnabled").checked = enabled === "true";

            const limit = localStorage.getItem(STORAGE_KEYS.limit);
            if (limit) $("limit").value = limit;

            const minDiff = localStorage.getItem(STORAGE_KEYS.minDiff);
            if (minDiff) $("minDiff").value = minDiff;

            const minVolume = localStorage.getItem(STORAGE_KEYS.minVolume);
            if (minVolume) $("minVolume").value = minVolume;

            const volumeUnit = localStorage.getItem(STORAGE_KEYS.volumeUnit);
            if (volumeUnit) $("volumeUnit").value = volumeUnit;

            // 恢复过滤器 Levels
            const filterLevels = localStorage.getItem(STORAGE_KEYS.filterLevels);
            if (filterLevels) {
                selectedLevels = new Set(JSON.parse(filterLevels));
            }

            // 恢复 Period 和 Direction
            const filterPeriod = localStorage.getItem(STORAGE_KEYS.filterPeriod);
            if (filterPeriod) $("period").value = filterPeriod;

            const filterDirection = localStorage.getItem(STORAGE_KEYS.filterDirection);
            if (filterDirection) $("direction").value = filterDirection;
        } catch (_) { }
    }

    function saveSettings() {
        try {
            localStorage.setItem(STORAGE_KEYS.soundLevels, JSON.stringify(Array.from(soundLevels)));
            localStorage.setItem(STORAGE_KEYS.soundEnabled, $("soundEnabled").checked);
            localStorage.setItem(STORAGE_KEYS.limit, $("limit").value);
            const minDiff = $("minDiff").value;
            if (minDiff) localStorage.setItem(STORAGE_KEYS.minDiff, minDiff);
            else localStorage.removeItem(STORAGE_KEYS.minDiff);

            const minVolume = $("minVolume").value;
            if (minVolume) localStorage.setItem(STORAGE_KEYS.minVolume, minVolume);
            else localStorage.removeItem(STORAGE_KEYS.minVolume);

            localStorage.setItem(STORAGE_KEYS.volumeUnit, $("volumeUnit").value);

            // 保存过滤器 Levels
            localStorage.setItem(STORAGE_KEYS.filterLevels, JSON.stringify(Array.from(selectedLevels)));

            // 保存 Period 和 Direction
            localStorage.setItem(STORAGE_KEYS.filterPeriod, $("period").value);
            localStorage.setItem(STORAGE_KEYS.filterDirection, $("direction").value);
        } catch (_) { }
    }

    // ==================== 成交额过滤 ====================
    const VOLUME_UNITS = { K: 1e3, M: 1e6, B: 1e9 };

    function parseVolumeInput() {
        const value = parseFloat($("minVolume").value) || 0;
        const unit = $("volumeUnit").value || 'M';
        return value * (VOLUME_UNITS[unit] || 1e6);
    }

    // ==================== 过滤逻辑 ====================
    function getFilters() {
        return {
            symbol: $("symbol").value.trim(),
            period: $("period").value,
            levels: Array.from(selectedLevels),
            direction: $("direction").value,
            minDiff: parseFloat($("minDiff").value) || 0,
            minVolume: parseVolumeInput()
        };
    }

    function matchSignal(signal, filters) {
        if (!signal) return false;

        // Symbol 过滤：$开头精确匹配，否则模糊匹配
        if (filters.symbol) {
            const query = filters.symbol.toUpperCase();
            const sym = String(signal.symbol || "").toUpperCase();
            if (query.startsWith("$")) {
                // 精确匹配
                if (sym !== query.slice(1)) return false;
            } else {
                // 模糊匹配
                if (!sym.includes(query)) return false;
            }
        }

        // Period 过滤
        if (filters.period && signal.period !== filters.period) return false;

        // Level 过滤
        if (filters.levels.length && !filters.levels.includes(signal.level)) return false;

        // Direction 过滤
        if (filters.direction && signal.direction !== filters.direction) return false;

        // Diff% 过滤
        if (filters.minDiff > 0) {
            const ticker = tickerData.get(signal.symbol);
            if (ticker && signal.price > 0) {
                const diffPct = Math.abs((ticker.last_price - signal.price) / signal.price * 100);
                if (diffPct < filters.minDiff) return false;
            }
        }

        // 成交额过滤
        if (filters.minVolume > 0) {
            const ticker = tickerData.get(signal.symbol);
            if (!ticker || (ticker.quote_volume || 0) < filters.minVolume) {
                return false;
            }
        }

        return true;
    }

    function applyFilters() {
        const filters = getFilters();
        filteredSignals = masterSignals.filter(s => matchSignal(s, filters));
        updateHint();
    }

    function updateHint() {
        if (currentView === 'patterns') {
            $("hint").textContent = `Patterns: ${filteredPatterns.length}/${masterPatterns.length}`;
        } else {
            $("hint").textContent = `Signals: ${filteredSignals.length}/${masterSignals.length}`;
        }
    }

    // ==================== Pattern 过滤逻辑 ====================
    function getPatternFilters() {
        return {
            symbol: $("symbol").value.trim(),
            patternType: $("patternType").value,
            efficiencyRank: $("efficiencyRank").value,
            direction: $("patternDirection").value
        };
    }

    function matchPattern(pattern, filters) {
        if (!pattern) return false;

        // Symbol 过滤
        if (filters.symbol) {
            const query = filters.symbol.toUpperCase();
            const sym = String(pattern.symbol || "").toUpperCase();
            if (query.startsWith("$")) {
                if (sym !== query.slice(1)) return false;
            } else {
                if (!sym.includes(query)) return false;
            }
        }

        // Pattern type 过滤
        if (filters.patternType && pattern.pattern !== filters.patternType) return false;

        // Efficiency rank 过滤
        if (filters.efficiencyRank) {
            const rank = pattern.efficiency_rank || '';
            if (!rank.startsWith(filters.efficiencyRank)) return false;
        }

        // Direction 过滤
        if (filters.direction && pattern.direction !== filters.direction) return false;

        return true;
    }

    function applyPatternFilters() {
        const filters = getPatternFilters();
        filteredPatterns = masterPatterns.filter(p => matchPattern(p, filters));
        updateHint();
    }

    // ==================== 排行计算 ====================
    function computeRanking() {
        // 基于 masterSignals 中的交易对计算排行
        const signalSymbols = new Set(masterSignals.map(s => s.symbol));
        const items = [];

        for (const symbol of signalSymbols) {
            const ticker = tickerData.get(symbol);
            if (ticker) {
                items.push({
                    symbol,
                    volume: ticker.quote_volume || 0,
                    trades: ticker.trade_count || 0
                });
            }
        }

        // 成交额排名
        const byVolume = [...items].sort((a, b) => b.volume - a.volume);
        symbolRanking.volume.clear();
        byVolume.forEach((it, i) => symbolRanking.volume.set(it.symbol, i + 1));

        // 交易数排名
        const byTrades = [...items].sort((a, b) => b.trades - a.trades);
        symbolRanking.trades.clear();
        byTrades.forEach((it, i) => symbolRanking.trades.set(it.symbol, i + 1));

        return { byVolume, byTrades };
    }

    // ==================== 渲染函数 ====================
    function renderSignalItem(signal, index) {
        const ticker = tickerData.get(signal.symbol);
        const volRank = symbolRanking.volume.get(signal.symbol);
        const tradeRank = symbolRanking.trades.get(signal.symbol);

        // 排行徽章
        let rankHtml = '';
        if (volRank || tradeRank) {
            const vr = volRank ? `<span class="rank-badge vol" title="Volume Rank">#${volRank}V</span>` : '';
            const tr = tradeRank ? `<span class="rank-badge trd" title="Trades Rank">#${tradeRank}T</span>` : '';
            rankHtml = `<div class="ranks">${vr}${tr}</div>`;
        }

        // 关联形态徽章
        let patternBadgeHtml = '';
        if (signal.related_pattern) {
            patternBadgeHtml = renderPatternBadge(signal.related_pattern);
        }

        // 价格差异
        let diffHtml = '';
        let tickerHtml = '';
        if (ticker) {
            const diff = ticker.last_price - signal.price;
            const diffPct = signal.price > 0 ? ((diff / signal.price) * 100) : 0;
            const diffSign = diff >= 0 ? '+' : '';
            const diffClass = diff >= 0 ? 'up' : 'down';
            diffHtml = `<span class="price-diff ${diffClass}">${diffSign}${diffPct.toFixed(2)}%</span>`;

            const pctClass = ticker.price_percent >= 0 ? 'up' : 'down';
            tickerHtml = `
                <div class="price-info">
                    <span class="price-now">${fmtPrice(ticker.last_price)}</span>
                    <span class="price-pct ${pctClass}">${fmtPct(ticker.price_percent)}</span>
                    <span class="volume">${fmtVolume(ticker.quote_volume)}</span>
                    <span class="trades">${fmtTradeCount(ticker.trade_count)} trades</span>
                </div>
            `;
        }

        return `
            <div class="item" data-index="${index}" data-symbol="${signal.symbol}" data-price="${signal.price}" data-time="${signal.triggered_at}">
                <div class="top">
                    <div class="sym">${signal.symbol} ${rankHtml}</div>
                    <div class="tags">
                        <span class="tag">${signal.period}</span>
                        <span class="tag">${signal.level}</span>
                        <span class="tag ${signal.direction}">${signal.direction}</span>
                        ${patternBadgeHtml}
                    </div>
                </div>
                <div class="sub">
                    <div>Signal: ${fmtPrice(signal.price)} ${diffHtml}</div>
                    <div class="muted time-rel">${fmtRelTime(signal.triggered_at)}</div>
                </div>
                ${tickerHtml}
                <div class="pivot-levels-row" data-symbol="${signal.symbol}"></div>
            </div>
        `;
    }

    function renderRankingItem(item, index, type) {
        const rankClass = index < 3 ? 'ranking-rank top3' : 'ranking-rank';
        const value = type === 'volume'
            ? fmtVolume(item.volume)
            : fmtTradeCount(item.trades) + ' trades';

        return `
            <div class="ranking-item" data-symbol="${item.symbol}">
                <span class="${rankClass}">#${index + 1}</span>
                <span class="ranking-symbol">${item.symbol}</span>
                <span class="ranking-value">${value}</span>
            </div>
        `;
    }

    // ==================== Pattern 渲染函数 ====================
    function renderPatternItem(pattern, index) {
        const effColor = getEfficiencyColor(pattern.efficiency_rank);
        const dirColor = getDirectionColor(pattern.direction);
        const dirArrow = pattern.direction === 'bullish' ? '↑' : pattern.direction === 'bearish' ? '↓' : '•';

        // 显示 confidence（需求 5.2）
        const confidence = pattern.confidence || 0;

        return `
            <div class="item pattern-item" data-index="${index}" data-symbol="${pattern.symbol}" data-pattern-id="${pattern.id}">
                <div class="top">
                    <div class="sym">${pattern.symbol}</div>
                    <div class="tags">
                        <span class="tag">${pattern.pattern_cn || pattern.pattern}</span>
                        <span class="tag" style="background:${dirColor};color:#fff">${dirArrow}</span>
                        <span class="tag rate" style="background:${effColor};color:#fff">${confidence}%</span>
                    </div>
                </div>
                <div class="sub">
                    <div>
                        <span class="efficiency">Rank: ${pattern.efficiency_rank || '-'}</span>
                        <span class="source-tag">${pattern.source || 'unknown'}</span>
                    </div>
                    <div class="muted time-rel" data-time="${pattern.detected_at}">${fmtRelTime(pattern.detected_at)}</div>
                </div>
            </div>
        `;
    }

    // 在枢轴点信号上显示关联形态徽章
    function renderPatternBadge(relatedPattern) {
        if (!relatedPattern) return '';

        const isBullish = relatedPattern.direction === 'bullish';
        const isBearish = relatedPattern.direction === 'bearish';

        // 上涨绿色，下跌红色
        const colorClass = isBullish ? 'bullish' : isBearish ? 'bearish' : 'neutral';
        const arrow = isBullish ? '↑' : isBearish ? '↓' : '•';

        // 显示对应方向的概率
        const rate = isBullish ? relatedPattern.up_percent :
            isBearish ? relatedPattern.down_percent :
                Math.max(relatedPattern.up_percent || 0, relatedPattern.down_percent || 0);

        // 形态名称（中文优先）
        const name = relatedPattern.pattern_cn || relatedPattern.pattern || '';

        // 时间差和数量信息
        const timeDiff = relatedPattern.time_diff || '';
        const count = relatedPattern.count || 1;
        const countInfo = count > 1 ? `(${count})` : '';

        // 构建 title 提示
        const dirText = isBullish ? '看涨' : isBearish ? '看跌' : '中性';
        const title = `${name} - ${dirText} ${rate}%${timeDiff ? ' - ' + timeDiff : ''}${count > 1 ? ' - 共' + count + '个形态' : ''}`;

        return `
            <span class="pattern-badge ${colorClass}" title="${title}">
                ${arrow}${rate}% ${name}${countInfo} ${timeDiff}
            </span>
        `;
    }

    // ==================== Clusterize 管理 ====================
    function initClusterize() {
        // 信号列表
        signalCluster = new Clusterize({
            rows: [],
            scrollId: 'signalScroll',
            contentId: 'signalList',
            rows_in_block: 20,
            blocks_in_cluster: 4,
            tag: null,
            no_data_text: 'No signals',
            no_data_class: 'clusterize-no-data',
            callbacks: {
                clusterChanged: function () {
                    bindSignalItemEvents();
                }
            }
        });

        // 形态列表
        patternCluster = new Clusterize({
            rows: [],
            scrollId: 'patternScroll',
            contentId: 'patternList',
            rows_in_block: 20,
            blocks_in_cluster: 4,
            tag: null,
            no_data_text: 'No patterns',
            no_data_class: 'clusterize-no-data',
            callbacks: {
                clusterChanged: function () {
                    bindPatternItemEvents();
                }
            }
        });

        // 排行榜
        rankingCluster = new Clusterize({
            rows: [],
            scrollId: 'rankingScroll',
            contentId: 'rankingList',
            rows_in_block: 15,
            blocks_in_cluster: 4,
            tag: null,
            no_data_text: 'No ranking data',
            no_data_class: 'clusterize-no-data',
            callbacks: {
                clusterChanged: function () {
                    bindRankingItemEvents();
                }
            }
        });
    }

    function updateSignalList() {
        computeRanking();
        const rows = filteredSignals.map((s, i) => renderSignalItem(s, i));
        signalCluster.update(rows);
    }

    function updatePatternList() {
        const rows = filteredPatterns.map((p, i) => renderPatternItem(p, i));
        patternCluster.update(rows);
    }

    function updateRankingList() {
        const { byVolume, byTrades } = computeRanking();
        const type = currentView;
        const items = type === 'volume' ? byVolume : byTrades;
        const rows = items.map((item, i) => renderRankingItem(item, i, type));
        rankingCluster.update(rows);
    }

    function updateView() {
        const signalScroll = $("signalScroll");
        const patternScroll = $("patternScroll");
        const rankingScroll = $("rankingScroll");
        const patternFilters = $("patternFilters");
        const showSignalsBtn = document.querySelector('[data-action="signals"]');

        // 隐藏所有滚动区域
        signalScroll.style.display = 'none';
        patternScroll.style.display = 'none';
        rankingScroll.style.display = 'none';
        patternFilters.style.display = 'none';

        if (currentView === 'signals') {
            signalScroll.style.display = '';
            showSignalsBtn.style.display = 'none';
            applyFilters();
            updateSignalList();
        } else if (currentView === 'patterns') {
            patternScroll.style.display = '';
            patternFilters.style.display = 'flex';
            showSignalsBtn.style.display = 'none';
            applyPatternFilters();
            updatePatternList();
        } else {
            rankingScroll.style.display = '';
            showSignalsBtn.style.display = '';
            updateRankingList();
        }
    }

    // ==================== Intersection Observer for Pivot Levels ====================
    const visibleSymbols = new Set();
    let pivotObserver = null;
    let pendingPivotSymbols = new Set(); // 待获取枢轴点的交易对
    let pivotFetchTimer = null;
    const PIVOT_FETCH_DEBOUNCE = 300; // 防抖延迟

    function initPivotObserver() {
        if (pivotObserver) return;

        pivotObserver = new IntersectionObserver((entries) => {
            let needsFetch = false;
            entries.forEach(entry => {
                const item = entry.target;
                const symbol = item.dataset.symbol;
                if (!symbol) return;

                if (entry.isIntersecting) {
                    visibleSymbols.add(symbol);
                    // 检查是否需要获取枢轴点数据
                    if (!pivotCache.has(symbol)) {
                        pendingPivotSymbols.add(symbol);
                        needsFetch = true;
                    } else {
                        // 已有缓存，直接更新
                        updateItemPivotLevelsSync(item, symbol);
                    }
                } else {
                    visibleSymbols.delete(symbol);
                    pendingPivotSymbols.delete(symbol);
                }
            });

            // 防抖批量获取
            if (needsFetch) {
                schedulePivotFetch();
            }
        }, { threshold: 0.1, rootMargin: '50px' });
    }

    function schedulePivotFetch() {
        if (pivotFetchTimer) clearTimeout(pivotFetchTimer);
        pivotFetchTimer = setTimeout(async () => {
            pivotFetchTimer = null;
            if (pendingPivotSymbols.size === 0) return;

            // 批量获取（目前 API 是单个获取，但我们可以并行请求）
            const symbols = Array.from(pendingPivotSymbols);
            pendingPivotSymbols.clear();

            // 并行获取所有待处理的交易对
            await Promise.all(symbols.map(async (symbol) => {
                if (!visibleSymbols.has(symbol)) return; // 已滚出可视区域
                await getPivotData(symbol);
            }));

            // 更新所有可视项的枢轴点位
            updateAllVisiblePivotLevels();
        }, PIVOT_FETCH_DEBOUNCE);
    }

    function updateAllVisiblePivotLevels() {
        document.querySelectorAll("#signalList .item[data-symbol]").forEach(item => {
            const symbol = item.dataset.symbol;
            if (visibleSymbols.has(symbol)) {
                updateItemPivotLevelsSync(item, symbol);
            }
        });
    }

    // 同步版本，使用已缓存的数据
    function updateItemPivotLevelsSync(item, symbol) {
        const pivotRow = item.querySelector('.pivot-levels-row');
        if (!pivotRow) return;

        const ticker = tickerData.get(symbol);
        if (!ticker || !ticker.last_price) {
            pivotRow.innerHTML = '';
            return;
        }

        const cached = pivotCache.get(symbol);
        if (!cached || !cached.data) {
            pivotRow.innerHTML = '';
            return;
        }

        const pivotData = cached.data;
        const currentPrice = ticker.last_price;

        // 日级点位
        const dailyLevels = findNearestLevels(pivotData, currentPrice, '1d');
        const dailyAbove = formatPivotLevel(dailyLevels.above, currentPrice, '1d');
        const dailyBelow = formatPivotLevel(dailyLevels.below, currentPrice, '1d');

        // 周级点位
        const weeklyLevels = findNearestLevels(pivotData, currentPrice, '1w');
        const weeklyAbove = formatPivotLevel(weeklyLevels.above, currentPrice, '1w');
        const weeklyBelow = formatPivotLevel(weeklyLevels.below, currentPrice, '1w');

        let html = '';
        if (dailyAbove || dailyBelow) {
            html += `<span class="pivot-pair">${dailyAbove} ${dailyBelow}</span>`;
        }
        if (weeklyAbove || weeklyBelow) {
            html += `<span class="pivot-pair">${weeklyAbove} ${weeklyBelow}</span>`;
        }

        pivotRow.innerHTML = html;
    }

    async function updateItemPivotLevels(item, symbol) {
        const pivotRow = item.querySelector('.pivot-levels-row');
        if (!pivotRow) return;

        const ticker = tickerData.get(symbol);
        if (!ticker || !ticker.last_price) {
            pivotRow.innerHTML = '';
            return;
        }

        const pivotData = await getPivotData(symbol);
        if (!pivotData) {
            pivotRow.innerHTML = '';
            return;
        }

        const currentPrice = ticker.last_price;

        // 日级点位
        const dailyLevels = findNearestLevels(pivotData, currentPrice, '1d');
        const dailyAbove = formatPivotLevel(dailyLevels.above, currentPrice, '1d');
        const dailyBelow = formatPivotLevel(dailyLevels.below, currentPrice, '1d');

        // 周级点位
        const weeklyLevels = findNearestLevels(pivotData, currentPrice, '1w');
        const weeklyAbove = formatPivotLevel(weeklyLevels.above, currentPrice, '1w');
        const weeklyBelow = formatPivotLevel(weeklyLevels.below, currentPrice, '1w');

        let html = '';
        if (dailyAbove || dailyBelow) {
            html += `<span class="pivot-pair">${dailyAbove} ${dailyBelow}</span>`;
        }
        if (weeklyAbove || weeklyBelow) {
            html += `<span class="pivot-pair">${weeklyAbove} ${weeklyBelow}</span>`;
        }

        pivotRow.innerHTML = html;
    }

    // ==================== 事件绑定 ====================
    function bindSignalItemEvents() {
        initPivotObserver();

        document.querySelectorAll("#signalList .item").forEach(item => {
            item.onclick = (e) => {
                e.preventDefault();
                e.stopPropagation();
                menuFromRanking = false;
                showActionMenu(e, item.dataset.symbol);
            };

            // 观察可视状态以更新枢轴点位
            if (pivotObserver) {
                pivotObserver.observe(item);
            }
        });
    }

    function bindRankingItemEvents() {
        document.querySelectorAll("#rankingList .ranking-item").forEach(item => {
            item.onclick = (e) => {
                e.preventDefault();
                e.stopPropagation();
                menuFromRanking = true;
                showActionMenu(e, item.dataset.symbol);
            };
        });
    }

    function bindPatternItemEvents() {
        document.querySelectorAll("#patternList .pattern-item").forEach(item => {
            item.onclick = (e) => {
                e.preventDefault();
                e.stopPropagation();
                const patternId = item.dataset.patternId;
                const pattern = masterPatterns.find(p => p.id === patternId);
                if (pattern) {
                    showPatternDetail(pattern);
                }
            };
        });
    }

    // ==================== Pattern 详情弹窗 ====================
    function showPatternDetail(pattern) {
        currentPatternSignal = pattern;

        $("modalSymbol").textContent = pattern.symbol;
        $("modalPatternName").textContent = pattern.pattern_cn || pattern.pattern;

        const dirText = pattern.direction === 'bullish' ? 'Bullish ↑' :
            pattern.direction === 'bearish' ? 'Bearish ↓' : 'Neutral';
        $("modalDirection").textContent = dirText;
        $("modalDirection").className = 'value ' + pattern.direction;

        $("modalUpPct").textContent = (pattern.up_percent || 0) + '%';
        $("modalUpBar").style.width = (pattern.up_percent || 0) + '%';

        $("modalDownPct").textContent = (pattern.down_percent || 0) + '%';
        $("modalDownBar").style.width = (pattern.down_percent || 0) + '%';

        const effRank = pattern.efficiency_rank || '-';
        $("modalEfficiency").textContent = effRank;
        $("modalEfficiency").className = 'value efficiency-' + (effRank[0] || '');

        $("modalConfidence").textContent = (pattern.confidence || 0) + '%';

        const sourceText = pattern.source === 'talib' ? 'TA-Lib' :
            pattern.source === 'custom' ? 'Custom' : pattern.source || '-';
        $("modalSource").textContent = sourceText + (pattern.stats_source ? ' (' + pattern.stats_source + ')' : '');

        $("modalDetectedAt").textContent = fmtTime(pattern.detected_at);

        $("patternModal").style.display = 'flex';
    }

    function hidePatternModal() {
        $("patternModal").style.display = 'none';
        currentPatternSignal = null;
    }

    // 暴露到全局以便 HTML onclick 调用
    window.hidePatternModal = hidePatternModal;

    // ==================== 枢轴点预览 Modal ====================
    async function showPivotPreview(symbol) {
        currentPivotPreviewSymbol = symbol;
        const pivotData = await getPivotData(symbol);
        if (!pivotData) {
            showToast('No pivot data for ' + symbol);
            return;
        }

        const modal = $("pivotModal");
        if (!modal) return;

        $("pivotModalSymbol").textContent = symbol;
        renderPivotPreview(pivotData, currentPivotPreviewPeriod);
        modal.style.display = 'flex';
    }

    function renderPivotPreview(pivotData, period) {
        const container = $("pivotLevelsList");
        if (!container) return;

        const levels = period === '1w' ? pivotData.weekly : pivotData.daily;
        if (!levels) {
            container.innerHTML = '<div class="no-data">No data</div>';
            return;
        }

        const ticker = tickerData.get(currentPivotPreviewSymbol);
        const currentPrice = ticker ? ticker.last_price : 0;

        // 构建所有点位数组
        const allLevels = [
            { name: 'R5', price: levels.r5, type: 'resistance' },
            { name: 'R4', price: levels.r4, type: 'resistance' },
            { name: 'R3', price: levels.r3, type: 'resistance' },
            { name: 'R2', price: levels.r2, type: 'resistance' },
            { name: 'R1', price: levels.r1, type: 'resistance' },
            { name: 'PP', price: levels.pp, type: 'pivot' },
            { name: 'S1', price: levels.s1, type: 'support' },
            { name: 'S2', price: levels.s2, type: 'support' },
            { name: 'S3', price: levels.s3, type: 'support' },
            { name: 'S4', price: levels.s4, type: 'support' },
            { name: 'S5', price: levels.s5, type: 'support' },
        ].filter(l => l.price > 0);

        // 按价格降序排列
        allLevels.sort((a, b) => b.price - a.price);

        // 找到当前价格应该插入的位置
        let currentPriceInserted = false;
        let html = '';

        for (let i = 0; i < allLevels.length; i++) {
            const level = allLevels[i];
            const pct = currentPrice > 0 ? ((level.price - currentPrice) / currentPrice * 100) : 0;
            const pctStr = (pct >= 0 ? '+' : '') + pct.toFixed(2) + '%';

            // 在适当位置插入当前价格
            if (!currentPriceInserted && currentPrice > 0 && level.price < currentPrice) {
                html += `<div class="pivot-level current-price">
                    <span class="level-name">当前价格</span>
                    <span class="level-price">${fmtPrice(currentPrice)}</span>
                    <span class="level-pct">--</span>
                </div>`;
                currentPriceInserted = true;
            }

            const typeClass = level.type === 'resistance' ? 'resistance' :
                level.type === 'support' ? 'support' : 'pivot-point';
            const pctClass = pct >= 0 ? 'pct-up' : 'pct-down';

            html += `<div class="pivot-level ${typeClass}">
                <span class="level-name">${level.name}</span>
                <span class="level-price">${fmtPrice(level.price)}</span>
                <span class="level-pct ${pctClass}">${pctStr}</span>
            </div>`;
        }

        // 如果当前价格低于所有点位
        if (!currentPriceInserted && currentPrice > 0) {
            html += `<div class="pivot-level current-price">
                <span class="level-name">当前价格</span>
                <span class="level-price">${fmtPrice(currentPrice)}</span>
                <span class="level-pct">--</span>
            </div>`;
        }

        container.innerHTML = html;

        // 更新周期按钮状态
        document.querySelectorAll('.pivot-period-btn').forEach(btn => {
            btn.classList.toggle('active', btn.dataset.period === period);
        });
    }

    function hidePivotModal() {
        const modal = $("pivotModal");
        if (modal) modal.style.display = 'none';
        currentPivotPreviewSymbol = null;
    }

    function switchPivotPeriod(period) {
        currentPivotPreviewPeriod = period;
        if (currentPivotPreviewSymbol) {
            getPivotData(currentPivotPreviewSymbol).then(data => {
                if (data) renderPivotPreview(data, period);
            });
        }
    }

    // 暴露到全局
    window.hidePivotModal = hidePivotModal;
    window.switchPivotPeriod = switchPivotPeriod;

    // ==================== 操作菜单 ====================
    function showActionMenu(e, symbol) {
        menuSymbol = symbol;
        const menu = $("actionMenu");
        const showSignalsBtn = document.querySelector('[data-action="signals"]');
        showSignalsBtn.style.display = menuFromRanking ? '' : 'none';

        menu.classList.add("show");
        const x = Math.min(e.clientX, window.innerWidth - 200);
        const y = Math.min(e.clientY, window.innerHeight - 180);
        menu.style.left = x + "px";
        menu.style.top = y + "px";
    }

    function hideActionMenu() {
        $("actionMenu").classList.remove("show");
        menuSymbol = null;
    }

    function copyToClipboard(text) {
        if (navigator.clipboard && window.isSecureContext) {
            return navigator.clipboard.writeText(text);
        }
        return new Promise((resolve, reject) => {
            const textarea = document.createElement('textarea');
            textarea.value = text;
            textarea.style.position = 'fixed';
            textarea.style.left = '-9999px';
            document.body.appendChild(textarea);
            textarea.focus();
            textarea.select();
            try {
                const success = document.execCommand('copy');
                document.body.removeChild(textarea);
                if (success) resolve();
                else reject(new Error('execCommand failed'));
            } catch (err) {
                document.body.removeChild(textarea);
                reject(err);
            }
        });
    }

    function setupActionMenu() {
        document.addEventListener("click", (e) => {
            if (!e.target.closest("#actionMenu")) hideActionMenu();
        });

        document.querySelectorAll(".action-menu-item").forEach(item => {
            item.addEventListener("click", (e) => {
                e.stopPropagation();
                const action = item.dataset.action;
                if (!menuSymbol) return;

                switch (action) {
                    case "trade":
                        jumpToTrade(menuSymbol);
                        break;

                    case "copy":
                        copyToClipboard(menuSymbol)
                            .then(() => showToast("Copied: " + menuSymbol))
                            .catch(() => showToast("Copy failed"));
                        break;

                    case "filter":
                        const currentSymbol = $("symbol").value.trim().toUpperCase();
                        const exactSymbol = "$" + menuSymbol;
                        if (currentSymbol === menuSymbol || currentSymbol === exactSymbol) {
                            $("symbol").value = "";
                            applyFilters();
                            updateView();
                            showToast("Filter cleared");
                        } else {
                            $("symbol").value = exactSymbol;
                            applyFilters();
                            updateView();
                            showToast("Filtered: " + menuSymbol);
                        }
                        break;

                    case "signals":
                        // 切换到 Signals 面板并过滤
                        $("symbol").value = "$" + menuSymbol;
                        currentView = 'signals';
                        document.querySelectorAll(".tab").forEach(t => {
                            t.classList.toggle("active", t.dataset.view === 'signals');
                        });
                        applyFilters();
                        updateView();
                        showToast("Showing signals for " + menuSymbol);
                        break;

                    case "pivots":
                        // 显示枢轴点预览
                        showPivotPreview(menuSymbol);
                        break;
                }
                hideActionMenu();
            });
        });
    }

    // ==================== 控件设置 ====================
    function setupLevelBtns() {
        // 过滤 Level 按钮
        document.querySelectorAll("#filterLevels button").forEach(b => {
            b.addEventListener("click", () => {
                const l = b.dataset.level;
                if (selectedLevels.has(l)) {
                    selectedLevels.delete(l);
                    b.classList.remove("active");
                } else {
                    selectedLevels.add(l);
                    b.classList.add("active");
                }
                saveSettings();
                applyFilters();
                updateView();
            });
        });

        // 声音 Level 按钮
        document.querySelectorAll("#soundLevels button").forEach(b => {
            b.addEventListener("click", () => {
                const l = b.dataset.level;
                if (soundLevels.has(l)) {
                    soundLevels.delete(l);
                    b.classList.remove("active");
                } else {
                    soundLevels.add(l);
                    b.classList.add("active");
                }
                saveSettings();
            });
        });

        $("soundEnabled").addEventListener("change", saveSettings);
    }

    function updateSoundLevelBtns() {
        document.querySelectorAll("#soundLevels button").forEach(b => {
            b.classList.toggle("active", soundLevels.has(b.dataset.level));
        });
    }

    function updateFilterLevelBtns() {
        document.querySelectorAll("#filterLevels button").forEach(b => {
            b.classList.toggle("active", selectedLevels.has(b.dataset.level));
        });
    }

    function setupTabs() {
        document.querySelectorAll(".tab").forEach(t => {
            t.addEventListener("click", () => {
                document.querySelectorAll(".tab").forEach(x => x.classList.remove("active"));
                t.classList.add("active");
                currentView = t.dataset.view;
                updateView();
            });
        });
    }

    function setupFilters() {
        // Symbol 搜索（防抖，仅前端过滤）
        const debouncedFilter = debounce(() => {
            applyFilters();
            applyPatternFilters();
            updateView();
        }, 200);

        $("symbol").oninput = debouncedFilter;
        $("period").onchange = () => { saveSettings(); applyFilters(); updateView(); };
        $("direction").onchange = () => { saveSettings(); applyFilters(); updateView(); };
        $("minDiff").oninput = debounce(() => {
            saveSettings();
            applyFilters();
            updateView();
        }, 300);

        // 成交额过滤
        const debouncedVolumeFilter = debounce(() => {
            saveSettings();
            applyFilters();
            updateView();
        }, 300);
        $("minVolume").oninput = debouncedVolumeFilter;
        $("volumeUnit").onchange = debouncedVolumeFilter;

        // Limit 变更需要重新请求后端
        $("limit").onchange = () => {
            saveSettings();
            loadHistory();
        };

        // Pattern 筛选器
        $("patternType").onchange = () => { applyPatternFilters(); updateView(); };
        $("efficiencyRank").onchange = () => { applyPatternFilters(); updateView(); };
        $("patternDirection").onchange = () => { applyPatternFilters(); updateView(); };
    }

    function setupPatternModal() {
        // 点击背景关闭弹窗
        $("patternModal").onclick = (e) => {
            if (e.target === $("patternModal")) {
                hidePatternModal();
            }
        };

        // Trade 按钮
        $("modalTradeBtn").onclick = () => {
            if (!currentPatternSignal) return;
            jumpToTrade(currentPatternSignal.symbol);
            hidePatternModal();
        };

        // Filter 按钮
        $("modalFilterBtn").onclick = () => {
            if (!currentPatternSignal) return;
            $("symbol").value = "$" + currentPatternSignal.symbol;
            applyPatternFilters();
            updateView();
            hidePatternModal();
            showToast("Filtered: " + currentPatternSignal.symbol);
        };
    }

    // ==================== 跳转交易 ====================
    function jumpToTrade(symbol) {
        if (!symbol) return;
        if (window.parent !== window) {
            window.parent.postMessage({ type: "jump_symbol", symbol }, "*");
        } else {
            window.open("https://www.binance.com/futures/" + symbol, "_blank");
        }
    }

    // ==================== 前端 Pattern 关联 ====================
    // 在前端查找与 signal 相关的 pattern（60分钟窗口内，前后都查）
    function findRelatedPattern(symbol, signalTime) {
        if (!masterPatterns || masterPatterns.length === 0) return null;

        const windowMs = 60 * 60 * 1000; // 60 minutes
        const signalTs = signalTime.getTime();

        let closest = null;
        let closestDiff = Infinity;

        for (const pat of masterPatterns) {
            if (pat.symbol !== symbol) continue;

            const patTs = new Date(pat.detected_at).getTime();
            const diff = Math.abs(signalTs - patTs);

            if (diff <= windowMs && diff < closestDiff) {
                closestDiff = diff;
                closest = pat;
            }
        }

        if (!closest) return null;

        // 构建 related_pattern 对象
        const pivotUp = true; // 需要从 signal 获取，这里简化处理
        const patternBullish = closest.direction === 'bullish';
        let correlation = 'moderate';
        if (closest.direction !== 'neutral') {
            correlation = (pivotUp && patternBullish) || (!pivotUp && !patternBullish) ? 'strong' : 'weak';
        }

        // 计算时间差
        const diffMs = signalTs - new Date(closest.detected_at).getTime();
        const diffMin = Math.round(Math.abs(diffMs) / 60000);
        const timeDiff = diffMs >= 0 ? `${diffMin}m ago` : `in ${diffMin}m`;

        return {
            id: closest.id,
            pattern: closest.pattern,
            pattern_cn: closest.pattern_cn,
            direction: closest.direction,
            confidence: closest.confidence,
            up_percent: closest.up_percent,
            down_percent: closest.down_percent,
            efficiency_rank: closest.efficiency_rank,
            correlation: correlation,
            detected_at: closest.detected_at,
            count: 1,
            time_diff: timeDiff
        };
    }

    // ==================== 数据加载 ====================

    // 获取枢轴点数据（带缓存）
    async function getPivotData(symbol) {
        const now = Date.now();
        const cached = pivotCache.get(symbol);
        if (cached && (now - cached.timestamp) < PIVOT_CACHE_TTL) {
            return cached.data;
        }

        try {
            const r = await fetch(`/api/pivots/${symbol}`);
            if (!r.ok) return null;
            const data = await r.json();
            pivotCache.set(symbol, { data, timestamp: now });
            return data;
        } catch (_) {
            return null;
        }
    }

    // 查找当前价格上下方最近的点位
    function findNearestLevels(pivotData, currentPrice, period) {
        if (!pivotData || !currentPrice) return { above: null, below: null };

        const levels = period === '1w' ? pivotData.weekly : pivotData.daily;
        if (!levels) return { above: null, below: null };

        const allLevels = [
            { name: 'R5', price: levels.r5 },
            { name: 'R4', price: levels.r4 },
            { name: 'R3', price: levels.r3 },
            { name: 'R2', price: levels.r2 },
            { name: 'R1', price: levels.r1 },
            { name: 'PP', price: levels.pp },
            { name: 'S1', price: levels.s1 },
            { name: 'S2', price: levels.s2 },
            { name: 'S3', price: levels.s3 },
            { name: 'S4', price: levels.s4 },
            { name: 'S5', price: levels.s5 },
        ].filter(l => l.price > 0);

        let above = null, below = null;
        let aboveDiff = Infinity, belowDiff = Infinity;

        for (const level of allLevels) {
            const diff = level.price - currentPrice;
            if (diff > 0 && diff < aboveDiff) {
                aboveDiff = diff;
                above = level;
            } else if (diff < 0 && Math.abs(diff) < belowDiff) {
                belowDiff = Math.abs(diff);
                below = level;
            }
        }

        return { above, below };
    }

    // 格式化枢轴点位显示
    function formatPivotLevel(level, currentPrice, period) {
        if (!level || !currentPrice) return '';
        const pct = ((level.price - currentPrice) / currentPrice * 100);
        const pctStr = (pct >= 0 ? '+' : '') + pct.toFixed(1) + '%';
        const colorClass = pct >= 0 ? 'pivot-up' : 'pivot-down';
        const periodLabel = period === '1w' ? '周' : '日';
        return `<span class="${colorClass}">${periodLabel}:${level.name}(${pctStr})</span>`;
    }

    async function loadHistory() {
        const limit = $("limit").value || 1000;
        $("hint").textContent = "Loading...";

        try {
            const r = await fetch(`/api/history?limit=${limit}`);
            if (!r.ok) throw new Error("http " + r.status);

            const data = await r.json();
            masterSignals = (data || []).sort((a, b) =>
                (new Date(b.triggered_at) || 0) - (new Date(a.triggered_at) || 0)
            );

            // 前端补充关联 pattern（如果后端没有关联）
            for (const sig of masterSignals) {
                if (!sig.related_pattern && masterPatterns.length > 0) {
                    const related = findRelatedPattern(sig.symbol, new Date(sig.triggered_at));
                    if (related) {
                        sig.related_pattern = related;
                    }
                }
            }

            applyFilters();
            updateView();
        } catch (e) {
            $("hint").textContent = "Load failed: " + e;
        }
    }

    async function loadPivotStatus() {
        try {
            const r = await fetch("/api/pivot-status");
            if (!r.ok) return;
            const d = await r.json();

            const fmt = p => p ? (
                (p.is_stale ? '<span class="pill stale">STALE</span>' : '<span class="pill fresh">OK</span>') +
                " " + fmtDur(p.seconds_until) + " (" + p.symbol_count + " symbols)"
            ) : "-";

            $("dailyStatus").innerHTML = fmt(d.daily);
            $("weeklyStatus").innerHTML = fmt(d.weekly);
        } catch (_) { }
    }

    async function loadRuntimeStats() {
        try {
            const r = await fetch("/api/runtime");
            if (!r.ok) return;
            const d = await r.json();

            // 头部简要信息
            $("rtGoroutines").textContent = d.goroutines || '-';
            $("rtKlines").textContent = d.kline_symbols || '-';
            // 显示 "交易对/信号数" 格式
            const symbols = d.symbols || 0;
            const signals = d.signals || 0;
            $("rtSignals").textContent = symbols + '/' + signals;
            $("rtHeap").textContent = d.heap_mb ? d.heap_mb.toFixed(0) : '-';

            // 底部详细信息
            const ftSSE = $("ftSSE");
            const ftGoroutines = $("ftGoroutines");
            const ftHeap = $("ftHeap");
            const ftSymbols = $("ftSymbols");
            const ftSignals = $("ftSignals");
            const ftUptime = $("ftUptime");
            const ftVersion = $("ftVersion");

            if (ftSSE) ftSSE.textContent = d.sse_subscribers || 0;
            if (ftGoroutines) ftGoroutines.textContent = d.goroutines || '-';
            if (ftHeap) ftHeap.textContent = d.heap_mb ? d.heap_mb.toFixed(1) + 'MB' : '-';
            if (ftSymbols) ftSymbols.textContent = symbols;
            if (ftSignals) ftSignals.textContent = signals;
            if (ftUptime) ftUptime.textContent = d.uptime || '-';
            if (ftVersion) ftVersion.textContent = d.version ? 'v' + d.version : '';
        } catch (_) { }
    }

    async function loadTickers() {
        try {
            const r = await fetch("/api/tickers");
            if (!r.ok) return;
            const data = await r.json();
            if (data && typeof data === "object") {
                for (const [symbol, ticker] of Object.entries(data)) {
                    tickerData.set(symbol, ticker);
                }
            }
        } catch (_) { }
    }

    async function loadPatterns() {
        try {
            const r = await fetch("/api/patterns?limit=500");
            if (!r.ok) return;
            const data = await r.json();
            masterPatterns = (data || []).sort((a, b) =>
                (new Date(b.detected_at) || 0) - (new Date(a.detected_at) || 0)
            );
            if (currentView === 'patterns') {
                applyPatternFilters();
                updatePatternList();
            }
        } catch (_) { }
    }

    // ==================== SSE 连接 ====================
    let tickerUpdatePending = false;
    let sseReconnectTimer = null;
    let sseReconnectDelay = 1000;
    const SSE_MAX_DELAY = 30000;

    function connectSSE() {
        // 清除之前的重连定时器
        if (sseReconnectTimer) {
            clearTimeout(sseReconnectTimer);
            sseReconnectTimer = null;
        }

        let es;
        try {
            es = new EventSource("/api/sse");
        } catch (_) {
            setStatus("disconnected");
            scheduleReconnect();
            return;
        }

        setStatus("reconnecting");

        es.onopen = () => {
            setStatus("connected");
            sseReconnectDelay = 1000; // 重置重连延迟
        };

        es.onerror = () => {
            const state = es.readyState;
            if (state === EventSource.CLOSED) {
                setStatus("disconnected");
                es.close();
                scheduleReconnect();
            } else {
                setStatus("reconnecting");
            }
        };

        // 新信号
        es.addEventListener("signal", e => {
            try {
                const signal = JSON.parse(e.data);

                // 声音提醒
                if (soundLevels.has(signal.level)) playBeep();

                // 合并到主数据
                if (signal && signal.id) {
                    const exists = masterSignals.findIndex(s => s.id === signal.id);
                    if (exists === -1) {
                        // 尝试关联最近的 pattern（前端关联）
                        if (!signal.related_pattern) {
                            const relatedPattern = findRelatedPattern(signal.symbol, new Date(signal.triggered_at));
                            if (relatedPattern) {
                                signal.related_pattern = relatedPattern;
                            }
                        }
                        masterSignals.unshift(signal);
                        // 保持数据量限制
                        const limit = parseInt($("limit").value) || 1000;
                        if (masterSignals.length > limit * 1.2) {
                            masterSignals = masterSignals.slice(0, limit);
                        }
                    }
                }

                // 重新过滤并更新视图
                applyFilters();
                if (currentView === 'signals') {
                    updateSignalList();
                }
            } catch (_) { }
        });

        // 形态信号
        es.addEventListener("pattern", e => {
            try {
                const pattern = JSON.parse(e.data);

                // 合并到主数据
                if (pattern && pattern.id) {
                    const exists = masterPatterns.findIndex(p => p.id === pattern.id);
                    if (exists === -1) {
                        masterPatterns.unshift(pattern);
                        // 保持数据量限制
                        if (masterPatterns.length > 1000) {
                            masterPatterns = masterPatterns.slice(0, 1000);
                        }
                    }
                }

                // 重新过滤并更新视图
                if (currentView === 'patterns') {
                    applyPatternFilters();
                    updatePatternList();
                }
            } catch (_) { }
        });

        // Ticker 更新（节流）
        es.addEventListener("ticker", e => {
            try {
                const batch = JSON.parse(e.data);
                if (batch && batch.tickers) {
                    // 更新数据层
                    for (const [symbol, ticker] of Object.entries(batch.tickers)) {
                        tickerData.set(symbol, ticker);
                    }

                    // 节流更新 DOM
                    if (!tickerUpdatePending) {
                        tickerUpdatePending = true;
                        requestAnimationFrame(() => {
                            updateVisibleItems();
                            tickerUpdatePending = false;
                        });
                    }
                }
            } catch (_) { }
        });
    }

    function scheduleReconnect() {
        if (sseReconnectTimer) return;

        sseReconnectTimer = setTimeout(() => {
            sseReconnectTimer = null;
            connectSSE();
        }, sseReconnectDelay);

        // 指数退避，最大 30 秒
        sseReconnectDelay = Math.min(sseReconnectDelay * 1.5, SSE_MAX_DELAY);
    }

    // 只更新可视区域的 DOM
    function updateVisibleItems() {
        if (currentView === 'signals') {
            // 更新可视的信号项
            document.querySelectorAll("#signalList .item[data-symbol]").forEach(item => {
                const symbol = item.dataset.symbol;
                const ticker = tickerData.get(symbol);
                if (!ticker) return;

                const signalPrice = parseFloat(item.dataset.price) || 0;

                // 更新价格差异
                const subDiv = item.querySelector(".sub > div:first-child");
                if (subDiv && signalPrice > 0) {
                    const diff = ticker.last_price - signalPrice;
                    const diffPct = (diff / signalPrice) * 100;
                    const diffSign = diff >= 0 ? '+' : '';
                    const diffClass = diff >= 0 ? 'up' : 'down';
                    subDiv.innerHTML = `Signal: ${fmtPrice(signalPrice)} <span class="price-diff ${diffClass}">${diffSign}${diffPct.toFixed(2)}%</span>`;
                }

                // 更新 ticker 信息
                let priceInfoEl = item.querySelector(".price-info");
                if (!priceInfoEl) {
                    priceInfoEl = document.createElement("div");
                    priceInfoEl.className = "price-info";
                    item.appendChild(priceInfoEl);
                }

                const pctClass = ticker.price_percent >= 0 ? 'up' : 'down';
                priceInfoEl.innerHTML = `
                    <span class="price-now">${fmtPrice(ticker.last_price)}</span>
                    <span class="price-pct ${pctClass}">${fmtPct(ticker.price_percent)}</span>
                    <span class="volume">${fmtVolume(ticker.quote_volume)}</span>
                    <span class="trades">${fmtTradeCount(ticker.trade_count)} trades</span>
                `;

                // 更新枢轴点位（仅可视项，使用同步版本）
                if (visibleSymbols.has(symbol)) {
                    updateItemPivotLevelsSync(item, symbol);
                }
            });

            // 更新枢轴点预览 Modal（如果打开）
            updatePivotPreviewIfOpen();
        } else {
            // 排行榜视图：重新计算并更新
            updateRankingList();
        }
    }

    // 更新枢轴点预览 Modal（实时价格更新）
    function updatePivotPreviewIfOpen() {
        const modal = $("pivotModal");
        if (!modal || modal.style.display === 'none' || !currentPivotPreviewSymbol) return;

        const cached = pivotCache.get(currentPivotPreviewSymbol);
        if (cached && cached.data) {
            renderPivotPreview(cached.data, currentPivotPreviewPeriod);
        }
    }

    // 更新相对时间
    function updateRelTimes() {
        document.querySelectorAll(".time-rel").forEach(el => {
            const item = el.closest(".item");
            if (item && item.dataset.time) {
                el.textContent = fmtRelTime(item.dataset.time);
            } else if (el.dataset.time) {
                el.textContent = fmtRelTime(el.dataset.time);
            }
        });
    }

    // ==================== 声音 ====================
    const playBeep = () => {
        if (!$("soundEnabled").checked) return;
        try {
            const c = new (window.AudioContext || window.webkitAudioContext)();
            const o = c.createOscillator();
            const g = c.createGain();
            o.type = "sine";
            o.frequency.value = 880;
            g.gain.value = 0.08;
            o.connect(g);
            g.connect(c.destination);
            o.start();
            o.stop(c.currentTime + 0.15);
            setTimeout(() => c.close(), 500);
        } catch (_) { }
    };

    // ==================== 初始化 ====================
    // iOS PWA 真实视口高度设置
    let initialVh = 0; // 记录初始视口高度

    function setVh() {
        const vh = (window.visualViewport?.height || window.innerHeight) * 0.01;
        // 首次设置时记录初始高度
        if (initialVh === 0) {
            initialVh = vh;
        }
        document.documentElement.style.setProperty('--vh', `${vh}px`);
    }

    function calcScrollHeight() {
        const headerArea = document.querySelector('.header-area');
        const footerStats = $("footerStats");
        if (!headerArea) return;

        const headerHeight = headerArea.offsetHeight;
        const footerHeight = footerStats ? footerStats.offsetHeight : 40;

        // 使用 visualViewport 获取真实可视高度（iOS PWA 兼容）
        const viewportHeight = window.visualViewport
            ? window.visualViewport.height
            : window.innerHeight;

        // 考虑底部栏高度
        const availableHeight = Math.max(200, viewportHeight - headerHeight - footerHeight - 8);

        $("signalScroll").style.height = availableHeight + 'px';
        $("patternScroll").style.height = availableHeight + 'px';
        $("rankingScroll").style.height = availableHeight + 'px';

        // 通知 Clusterize 重新计算
        if (signalCluster) signalCluster.refresh();
        if (patternCluster) patternCluster.refresh();
        if (rankingCluster) rankingCluster.refresh();
    }

    // iOS 键盘收起后恢复布局
    function handleViewportResize() {
        const currentVh = (window.visualViewport?.height || window.innerHeight) * 0.01;

        // 如果视口高度恢复到接近初始值，说明键盘收起了
        if (initialVh > 0 && currentVh >= initialVh * 0.95) {
            // 使用初始高度，避免键盘影响
            document.documentElement.style.setProperty('--vh', `${initialVh}px`);
        } else {
            setVh();
        }

        calcScrollHeight();
    }

    function init() {
        // 立即设置 --vh 变量（iOS PWA 关键）
        setVh();

        loadSettings();
        updateSoundLevelBtns();
        updateFilterLevelBtns();
        setupLevelBtns();
        setupTabs();
        setupFilters();
        setupActionMenu();
        setupPatternModal();
        initClusterize();

        // 延迟计算高度，确保 DOM 已渲染
        requestAnimationFrame(() => {
            calcScrollHeight();
        });
        window.addEventListener('resize', debounce(calcScrollHeight, 100));

        // iOS PWA 模式下监听 visualViewport 变化
        if (window.visualViewport) {
            window.visualViewport.addEventListener('resize', debounce(handleViewportResize, 150));
        }

        // 监听输入框 blur 事件，键盘收起后强制恢复布局
        document.addEventListener('focusout', (e) => {
            if (e.target.tagName === 'INPUT' || e.target.tagName === 'SELECT') {
                // 延迟执行，等待键盘完全收起
                setTimeout(() => {
                    if (initialVh > 0) {
                        document.documentElement.style.setProperty('--vh', `${initialVh}px`);
                    }
                    calcScrollHeight();
                }, 300);
            }
        });

        // Runtime stats 点击刷新
        const runtimeStats = $("runtimeStats");
        if (runtimeStats) {
            runtimeStats.onclick = loadRuntimeStats;
        }

        // Refresh 按钮
        $("refresh").onclick = () => {
            loadHistory();
            loadPatterns();
            loadPivotStatus();
            loadTickers();
            loadRuntimeStats();
        };

        // 初始加载 - 先加载 patterns，再加载 history（确保后端能关联 pattern）
        loadTickers().then(() => {
            loadPatterns().then(() => {
                loadHistory();
            });
            loadPivotStatus();
            loadRuntimeStats();
            connectSSE();
        });

        // 定时任务
        setInterval(loadPivotStatus, 60000);
        setInterval(loadRuntimeStats, 5000); // 每5秒刷新运行时状态
        setInterval(updateRelTimes, 10000);
    }

    // 启动
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
