# Design Document: PC 宽屏多面板布局

## Overview

本设计为枢轴监控系统实现 PC 宽屏场景下的多面板并排显示功能。通过 CSS 媒体查询检测屏幕宽度，在宽屏设备上自动切换到三面板布局，同时保持移动端原有的 tab 切换体验。

核心设计原则：
- 响应式布局，自动适配不同屏幕尺寸
- 各面板数据和交互完全独立
- 最小化对现有代码的改动
- 复用现有的渲染函数和 Clusterize 虚拟滚动

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Header Area                              │
│  (搜索、语言、状态、过滤器 - 全局共享)                            │
├─────────────────────────────────────────────────────────────────┤
│                    Multi-Panel Container                         │
│  ┌─────────────────┬─────────────────┬─────────────────┐        │
│  │   Signal Panel  │  Volume Panel   │  Trades Panel   │        │
│  │                 │                 │                 │        │
│  │  ┌───────────┐  │  ┌───────────┐  │  ┌───────────┐  │        │
│  │  │  Header   │  │  │  Header   │  │  │  Header   │  │        │
│  │  ├───────────┤  │  ├───────────┤  │  ├───────────┤  │        │
│  │  │           │  │  │           │  │  │           │  │        │
│  │  │  Virtual  │  │  │  Virtual  │  │  │  Virtual  │  │        │
│  │  │  Scroll   │  │  │  Scroll   │  │  │  Scroll   │  │        │
│  │  │  List     │  │  │  List     │  │  │  List     │  │        │
│  │  │           │  │  │           │  │  │           │  │        │
│  │  └───────────┘  │  └───────────┘  │  └───────────┘  │        │
│  └─────────────────┴─────────────────┴─────────────────┘        │
├─────────────────────────────────────────────────────────────────┤
│                         Footer Stats                             │
└─────────────────────────────────────────────────────────────────┘
```

### 布局模式切换

```
Viewport Width > 1200px
        │
        ▼
┌───────────────────┐
│ isWidescreen=true │
│ 显示三面板并排    │
│ 隐藏 tab 按钮     │
└───────────────────┘

Viewport Width ≤ 1200px
        │
        ▼
┌───────────────────┐
│ isWidescreen=false│
│ 显示单面板+tab    │
│ 保持原有行为      │
└───────────────────┘
```

## Components and Interfaces

### 1. 布局检测模块

```javascript
// 宽屏断点常量
const WIDESCREEN_BREAKPOINT = 1200;

// 布局状态
let isWidescreen = false;

// 检测并更新布局模式
function checkWidescreenMode() {
    const newIsWidescreen = window.innerWidth > WIDESCREEN_BREAKPOINT;
    if (newIsWidescreen !== isWidescreen) {
        isWidescreen = newIsWidescreen;
        updateLayoutMode();
    }
}

// 初始化时检测，并监听 resize 事件
window.addEventListener('resize', debounce(checkWidescreenMode, 150));
```

### 2. 多面板容器组件

```html
<!-- 宽屏多面板容器 -->
<div id="multiPanelContainer" class="multi-panel-container">
    <!-- 信号面板 -->
    <div class="panel signal-panel">
        <div class="panel-header">
            <span class="panel-title">信号</span>
            <span class="panel-count" id="signalPanelCount">0</span>
        </div>
        <div id="wideSignalScroll" class="panel-scroll clusterize-scroll">
            <div id="wideSignalList" class="clusterize-content"></div>
        </div>
    </div>
    
    <!-- 成交额面板 -->
    <div class="panel volume-panel">
        <div class="panel-header">
            <span class="panel-title">成交额</span>
            <span class="panel-count" id="volumePanelCount">0</span>
        </div>
        <div id="wideVolumeScroll" class="panel-scroll clusterize-scroll">
            <div id="wideVolumeList" class="clusterize-content"></div>
        </div>
    </div>
    
    <!-- 成交笔数面板 -->
    <div class="panel trades-panel">
        <div class="panel-header">
            <span class="panel-title">成交笔数</span>
            <span class="panel-count" id="tradesPanelCount">0</span>
        </div>
        <div id="wideTradesScroll" class="panel-scroll clusterize-scroll">
            <div id="wideTradesList" class="clusterize-content"></div>
        </div>
    </div>
</div>
```

### 3. CSS 样式

```css
/* 多面板容器 - 默认隐藏 */
.multi-panel-container {
    display: none;
}

/* 宽屏模式激活 */
@media (min-width: 1201px) {
    /* 隐藏原有的 tab 按钮和单面板滚动区域 */
    .tabs {
        display: none !important;
    }
    
    .scroll-area {
        display: none !important;
    }
    
    /* 显示多面板容器 */
    .multi-panel-container {
        display: flex;
        flex: 1;
        gap: 12px;
        padding: 0 12px 12px;
        overflow: hidden;
        max-width: 1800px;
        margin: 0 auto;
    }
    
    /* 单个面板 */
    .panel {
        flex: 1;
        min-width: 300px;
        display: flex;
        flex-direction: column;
        background: var(--card);
        border: 1px solid var(--line);
        border-radius: 8px;
        overflow: hidden;
    }
    
    /* 面板头部 */
    .panel-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 10px 12px;
        background: var(--vessel);
        border-bottom: 1px solid var(--line);
        flex-shrink: 0;
    }
    
    .panel-title {
        font-weight: 600;
        font-size: 13px;
        color: var(--text);
    }
    
    .panel-count {
        font-size: 11px;
        color: var(--text-secondary);
        background: var(--tag-bg);
        padding: 2px 8px;
        border-radius: 4px;
    }
    
    /* 面板滚动区域 */
    .panel-scroll {
        flex: 1;
        overflow-y: auto;
        overflow-x: hidden;
        -webkit-overflow-scrolling: touch;
    }
    
    /* 调整 header-area 最大宽度 */
    .header-area {
        max-width: 1800px;
    }
}
```

### 4. Clusterize 实例管理

```javascript
// 宽屏模式下的 Clusterize 实例
let wideSignalCluster = null;
let wideVolumeCluster = null;
let wideTradesCluster = null;

// 初始化宽屏 Clusterize 实例
function initWidescreenClusters() {
    if (wideSignalCluster) return; // 已初始化
    
    wideSignalCluster = new Clusterize({
        rows: [],
        scrollId: 'wideSignalScroll',
        contentId: 'wideSignalList',
        rows_in_block: 15,
        blocks_in_cluster: 4,
        no_data_text: t("no_signals"),
        callbacks: {
            clusterChanged: bindWideSignalEvents
        }
    });
    
    wideVolumeCluster = new Clusterize({
        rows: [],
        scrollId: 'wideVolumeScroll',
        contentId: 'wideVolumeList',
        rows_in_block: 15,
        blocks_in_cluster: 4,
        no_data_text: t("no_ranking"),
        callbacks: {
            clusterChanged: bindWideVolumeEvents
        }
    });
    
    wideTradesCluster = new Clusterize({
        rows: [],
        scrollId: 'wideTradesScroll',
        contentId: 'wideTradesList',
        rows_in_block: 15,
        blocks_in_cluster: 4,
        no_data_text: t("no_ranking"),
        callbacks: {
            clusterChanged: bindWideTradesEvents
        }
    });
}

// 销毁宽屏 Clusterize 实例
function destroyWidescreenClusters() {
    if (wideSignalCluster) {
        wideSignalCluster.destroy();
        wideSignalCluster = null;
    }
    if (wideVolumeCluster) {
        wideVolumeCluster.destroy();
        wideVolumeCluster = null;
    }
    if (wideTradesCluster) {
        wideTradesCluster.destroy();
        wideTradesCluster = null;
    }
}
```

### 5. 数据更新接口

```javascript
// 更新所有宽屏面板
function updateWidescreenPanels() {
    if (!isWidescreen) return;
    
    // 更新信号面板
    updateWideSignalPanel();
    
    // 更新成交额面板
    updateWideVolumePanel();
    
    // 更新成交笔数面板
    updateWideTradesPanel();
}

// 更新信号面板
function updateWideSignalPanel() {
    if (!wideSignalCluster) return;
    
    computeRanking();
    const rows = filteredSignals.map((s, i) => renderSignalItem(s, i));
    wideSignalCluster.update(rows);
    
    // 更新计数
    const countEl = $("signalPanelCount");
    if (countEl) {
        countEl.textContent = `${filteredSignals.length}/${masterSignals.length}`;
    }
}

// 更新成交额面板
async function updateWideVolumePanel() {
    if (!wideVolumeCluster) return;
    
    await loadRanking('volume');
    const items = rankingData.volume || [];
    const filtered = filterRankingItems(items);
    const sorted = sortRankingItems(filtered, 'volume');
    const rows = sorted.map((item, i) => renderRankingItemFromAPI(item, i, 'volume'));
    wideVolumeCluster.update(rows);
    
    // 更新计数
    const countEl = $("volumePanelCount");
    if (countEl) {
        countEl.textContent = String(filtered.length);
    }
}

// 更新成交笔数面板
async function updateWideTradesPanel() {
    if (!wideTradesCluster) return;
    
    await loadRanking('trades');
    const items = rankingData.trades || [];
    const filtered = filterRankingItems(items);
    const sorted = sortRankingItems(filtered, 'trades');
    const rows = sorted.map((item, i) => renderRankingItemFromAPI(item, i, 'trades'));
    wideTradesCluster.update(rows);
    
    // 更新计数
    const countEl = $("tradesPanelCount");
    if (countEl) {
        countEl.textContent = String(filtered.length);
    }
}
```

### 6. 布局模式切换

```javascript
// 切换布局模式
function updateLayoutMode() {
    const multiPanel = $("multiPanelContainer");
    const scrollArea = document.querySelector(".scroll-area");
    const tabs = document.querySelector(".tabs");
    
    if (isWidescreen) {
        // 切换到宽屏模式
        if (multiPanel) multiPanel.style.display = 'flex';
        if (scrollArea) scrollArea.style.display = 'none';
        if (tabs) tabs.style.display = 'none';
        
        // 初始化宽屏 Clusterize
        initWidescreenClusters();
        
        // 加载所有面板数据
        updateWidescreenPanels();
    } else {
        // 切换到窄屏模式
        if (multiPanel) multiPanel.style.display = 'none';
        if (scrollArea) scrollArea.style.display = '';
        if (tabs) tabs.style.display = '';
        
        // 恢复原有视图
        updateView();
    }
}
```

## Data Models

### 面板状态模型

```javascript
// 各面板独立的滚动位置（可选，用于恢复滚动位置）
const panelScrollPositions = {
    signal: 0,
    volume: 0,
    trades: 0
};

// 保存滚动位置
function savePanelScrollPosition(panelType) {
    const scrollEl = $(`wide${capitalize(panelType)}Scroll`);
    if (scrollEl) {
        panelScrollPositions[panelType] = scrollEl.scrollTop;
    }
}

// 恢复滚动位置
function restorePanelScrollPosition(panelType) {
    const scrollEl = $(`wide${capitalize(panelType)}Scroll`);
    if (scrollEl && panelScrollPositions[panelType]) {
        scrollEl.scrollTop = panelScrollPositions[panelType];
    }
}
```



## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system—essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: 布局模式响应断点

*For any* viewport width value, the layout mode should correctly reflect the breakpoint rule: widescreen mode (three panels visible, tabs hidden) when width > 1200px, and narrow mode (tabs visible, single panel) when width ≤ 1200px.

**Validates: Requirements 1.1, 1.2, 7.1**

### Property 2: 数据在布局切换时保持完整

*For any* set of signal and ranking data, and any sequence of layout mode transitions (resize across breakpoint), the data arrays (masterSignals, filteredSignals, rankingData) should remain unchanged after the transition.

**Validates: Requirements 1.3, 1.4**

### Property 3: 面板滚动独立性

*For any* scroll action on one panel, the scroll positions of the other two panels should remain unchanged.

**Validates: Requirements 2.2, 3.2, 4.2, 6.1**

### Property 4: 全局过滤器影响所有面板

*For any* symbol filter input, all three panels (signal, volume, trades) should filter their displayed items to match the filter criteria.

**Validates: Requirements 2.3, 2.4, 3.3, 4.3**

### Property 5: 各面板独立显示数据

*For any* valid data state, when in widescreen mode, all three panels should display their respective data (signals in signal panel, volume ranking in volume panel, trades ranking in trades panel) simultaneously.

**Validates: Requirements 2.1, 3.1, 4.1, 6.4**

### Property 6: 面板等宽分布

*For any* viewport width > 1200px, all three panels should have equal width (within 1px tolerance due to rounding).

**Validates: Requirements 5.1, 5.4**

## Error Handling

### 布局检测失败

如果 `window.innerWidth` 返回异常值（如 0 或 undefined），系统应默认使用窄屏模式，确保基本功能可用。

```javascript
function checkWidescreenMode() {
    const width = window.innerWidth;
    // 防御性检查
    if (!width || width <= 0) {
        isWidescreen = false;
        return;
    }
    // ... 正常逻辑
}
```

### Clusterize 初始化失败

如果 Clusterize 库加载失败或初始化异常，系统应捕获错误并回退到简单的 innerHTML 渲染。

```javascript
function initWidescreenClusters() {
    try {
        wideSignalCluster = new Clusterize({ ... });
        // ...
    } catch (e) {
        console.error('Clusterize init failed:', e);
        // 回退到简单渲染
        useFallbackRendering = true;
    }
}
```

### 数据加载失败

如果排行榜 API 请求失败，面板应显示友好的错误提示，而不是空白。

```javascript
async function updateWideVolumePanel() {
    try {
        await loadRanking('volume');
        // ... 正常渲染
    } catch (e) {
        // 显示错误状态
        wideVolumeCluster.update([`<div class="error-state">${t("hint_load_failed", { error: e.message })}</div>`]);
    }
}
```

## Testing Strategy

### 单元测试

单元测试用于验证具体的边界情况和错误处理：

1. **断点边界测试**
   - 测试 width = 1200 时应为窄屏模式
   - 测试 width = 1201 时应为宽屏模式
   - 测试 width = 0 或 undefined 时的防御性处理

2. **过滤器应用测试**
   - 测试 symbol 精确匹配（$开头）
   - 测试 symbol 模糊匹配
   - 测试空过滤器返回全部数据

3. **数据渲染测试**
   - 测试空数据时显示 "no data" 提示
   - 测试大量数据时 Clusterize 正确初始化

### 属性测试

属性测试用于验证系统在各种输入下的通用行为：

1. **Property 1: 布局模式响应断点**
   - 生成随机 viewport width 值
   - 验证 isWidescreen 状态与断点规则一致

2. **Property 2: 数据保持完整**
   - 生成随机信号和排行数据
   - 模拟多次布局切换
   - 验证数据数组内容不变

3. **Property 3: 滚动独立性**
   - 生成随机滚动位置
   - 对一个面板执行滚动
   - 验证其他面板滚动位置不变

4. **Property 4: 过滤器影响所有面板**
   - 生成随机过滤条件
   - 验证所有面板的过滤结果一致

5. **Property 5: 面板数据独立显示**
   - 生成随机数据集
   - 验证三个面板同时显示各自数据

6. **Property 6: 面板等宽**
   - 生成随机 viewport width > 1200
   - 验证三个面板宽度相等

### 测试框架

由于这是前端 JavaScript 代码，推荐使用：
- **Jest** 作为测试运行器
- **fast-check** 作为属性测试库
- **jsdom** 模拟 DOM 环境

每个属性测试应运行至少 100 次迭代以确保覆盖各种边界情况。
