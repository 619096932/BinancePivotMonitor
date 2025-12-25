chrome.storage.local.get(['config'], (res) => {
    const cfg = res.config || {};
    const url = (cfg.serverUrl || '').trim().replace(/\/+$/, '');

    if (!url) {
        document.getElementById('content').innerHTML =
            '<div class="error">Please configure Server URL in extension settings first</div>';
        return;
    }

    const iframe = document.createElement('iframe');
    iframe.src = url;
    iframe.allow = 'autoplay';
    document.getElementById('content').appendChild(iframe);
});

// 监听来自 iframe 的消息，处理跳转请求
window.addEventListener('message', async (event) => {
    if (!event.data || event.data.type !== 'jump_symbol') return;

    const symbol = event.data.symbol || '';
    if (!symbol) return;

    // 查找当前激活的标签页，判断是 TV 还是币安
    const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
    const activeTab = tabs[0];

    if (activeTab && activeTab.url) {
        const url = activeTab.url;
        if (url.includes('tradingview.com')) {
            // TradingView: 更新当前标签页 URL
            const baseSymbol = symbol.replace('USDT', '');
            const newUrl = `https://www.tradingview.com/chart/?symbol=BINANCE:${baseSymbol}USDT.P`;
            chrome.tabs.update(activeTab.id, { url: newUrl });
            return;
        } else if (url.includes('binance.com')) {
            // Binance: 更新当前标签页 URL
            const newUrl = `https://www.binance.com/futures/${symbol}`;
            chrome.tabs.update(activeTab.id, { url: newUrl });
            return;
        }
    }

    // 如果当前不是 TV 或币安，调用 background.js 的 jump_tab 逻辑
    chrome.runtime.sendMessage({ type: 'jump_tab', symbol }, () => { });
});
