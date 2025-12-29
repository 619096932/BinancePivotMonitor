# Requirements Document

## Introduction

本功能增强旨在扩展 Binance Pivot Monitor 的枢轴点监控能力，包括：全点位触发推送、信号存储扩容、枢轴点 API 查询、前端实时价格与枢轴点位渲染、以及枢轴点预览功能。核心关注点是性能优化、数据一致性和数据联动性。

## Glossary

- **Pivot_Monitor**: 枢轴点监控系统，负责监控价格与枢轴点位的交叉触发
- **Signal_History**: 信号历史存储模块，负责持久化和查询历史信号
- **Pivot_Store**: 枢轴点数据存储，包含日级和周级枢轴点数据
- **SSE_Broker**: Server-Sent Events 消息推送代理
- **Ticker_Data**: 实时价格数据，通过 WebSocket 推送
- **Pivot_Level**: 枢轴点位，包括 R1-R5（阻力位）和 S1-S5（支撑位）
- **Frontend_Renderer**: 前端渲染模块，负责信号列表和枢轴点预览的渲染
- **Intersection_Observer**: 浏览器 API，用于检测元素是否在可视区域内

## Requirements

### Requirement 1: 全点位触发推送

**User Story:** As a trader, I want to receive notifications for all pivot levels (PP, R1-R5, S1-S5), so that I can monitor more price action opportunities.

#### Acceptance Criteria

1. WHEN price crosses any pivot level (PP, R1, R2, R3, R4, R5, S1, S2, S3, S4, S5) THEN the Pivot_Monitor SHALL emit a signal
2. WHEN a signal is emitted for any level THEN the Signal_History SHALL persist the signal
3. WHEN a signal is emitted THEN the SSE_Broker SHALL broadcast the signal to all connected clients
4. THE Pivot_Monitor SHALL maintain the existing cooldown mechanism for all levels
5. THE Pivot_Monitor SHALL include PP (Pivot Point) as a monitored level
6. THE cooldown mechanism SHALL be applied per symbol-period-level combination (e.g., BTCUSDT|1d|R4)
7. WHEN price crosses multiple different levels within the cooldown period THEN the Pivot_Monitor SHALL emit signals for each distinct level
8. WHEN a symbol receives its first price update THEN the Pivot_Monitor SHALL record the price without emitting signals (baseline establishment)
9. WHEN price jumps across multiple levels in a single update THEN the Pivot_Monitor SHALL emit signals for all crossed levels (not just the final level)
10. IF pivot data is not available for a symbol THEN the Pivot_Monitor SHALL skip that symbol silently

### Requirement 2: 信号存储扩容

**User Story:** As a system administrator, I want to increase signal storage capacity to 4000 records, so that I can retain more historical data for analysis.

#### Acceptance Criteria

1. THE Signal_History SHALL support storing up to 4000 signals (increased from 2000)
2. WHEN the signal count exceeds 4000 THEN the Signal_History SHALL remove the oldest signals
3. THE Signal_History SHALL maintain file compaction behavior when file lines exceed 2x the max limit
4. WHEN querying signals THEN the API SHALL support limit parameter up to 4000

### Requirement 3: 枢轴点 API 查询

**User Story:** As a developer, I want to query pivot levels for a specific trading pair, so that I can integrate pivot data into other applications.

#### Acceptance Criteria

1. WHEN a GET request is made to `/api/pivots/{symbol}` THEN the Server SHALL return both daily and weekly pivot levels for that symbol
2. WHEN a GET request is made to `/api/pivots/{symbol}?period=1d` THEN the Server SHALL return only daily pivot levels
3. WHEN a GET request is made to `/api/pivots/{symbol}?period=1w` THEN the Server SHALL return only weekly pivot levels
4. IF the symbol does not exist THEN the Server SHALL return a 404 status with an error message
5. THE API response SHALL include all pivot levels (PP, R1-R5, S1-S5, High, Low, Close) and metadata (updated_at)

### Requirement 4: 前端枢轴点位渲染

**User Story:** As a trader, I want to see nearby pivot levels relative to the current price in each signal item, so that I can quickly assess potential support and resistance.

#### Acceptance Criteria

1. WHEN rendering a signal item THEN the Frontend_Renderer SHALL display the nearest pivot levels above and below the current price
2. THE Frontend_Renderer SHALL display daily pivot levels with format: "日:R4(-0.5%)红色 R5(1%)绿色"
3. THE Frontend_Renderer SHALL display weekly pivot levels with format: "周S5(-0.5%)红色 S4(1%)绿色"
4. WHEN the current price is below a pivot level THEN the percentage SHALL be displayed in green (positive)
5. WHEN the current price is above a pivot level THEN the percentage SHALL be displayed in red (negative)
6. WHEN Ticker_Data updates via SSE THEN the Frontend_Renderer SHALL update the pivot level percentages in real-time
7. THE Frontend_Renderer SHALL only render pivot levels for items within the visible viewport (using Intersection_Observer)
8. WHEN an item scrolls out of the viewport THEN the Frontend_Renderer SHALL stop updating that item's pivot levels

### Requirement 5: 枢轴点预览功能

**User Story:** As a trader, I want to preview all pivot levels for a symbol when clicking on any tab area, so that I can see the complete price structure.

#### Acceptance Criteria

1. WHEN a user clicks on a signal item or ranking item THEN the Frontend_Renderer SHALL display a pivot preview modal
2. THE pivot preview modal SHALL display all levels (R5, R4, R3, R2, R1, PP, S1, S2, S3, S4, S5) sorted by price value in descending order
3. THE pivot preview modal SHALL display the current price as the central reference point, with levels above and below it
4. THE pivot preview modal SHALL show percentage distance from current price for each level (e.g., "R5 (10%)", "S4 (-20%)")
5. THE pivot preview modal SHALL support switching between daily (日级) and weekly (周级) pivot data
6. WHEN daily view is selected THEN the Frontend_Renderer SHALL display daily pivot levels
7. WHEN weekly view is selected THEN the Frontend_Renderer SHALL display weekly pivot levels
8. THE pivot preview modal SHALL update in real-time when Ticker_Data changes (current price position may shift)
9. IF pivot data is not available for the symbol THEN the Frontend_Renderer SHALL display a "No pivot data" message
10. THE current price indicator SHALL be visually distinct and positioned between the nearest upper and lower pivot levels

### Requirement 6: 性能优化

**User Story:** As a user, I want the application to remain responsive even with increased data volume, so that I can use it efficiently.

#### Acceptance Criteria

1. THE Frontend_Renderer SHALL use virtual scrolling for signal lists (existing Clusterize.js)
2. THE Frontend_Renderer SHALL throttle real-time updates to prevent excessive DOM operations
3. THE Frontend_Renderer SHALL use requestAnimationFrame for batched DOM updates
4. THE Signal_History SHALL use efficient in-memory indexing for queries
5. WHEN loading pivot data for preview THEN the Frontend_Renderer SHALL cache the data to avoid redundant API calls

### Requirement 7: 数据一致性

**User Story:** As a user, I want to see consistent data across all views, so that I can trust the information displayed.

#### Acceptance Criteria

1. WHEN Ticker_Data updates THEN all visible components displaying that symbol SHALL update simultaneously
2. THE Frontend_Renderer SHALL maintain a single source of truth for ticker data (tickerData Map)
3. THE Frontend_Renderer SHALL maintain a single source of truth for pivot data (pivotCache Map)
4. WHEN pivot data is refreshed on the server THEN the Frontend_Renderer SHALL fetch updated data on next access

### Requirement 8: 成交额过滤

**User Story:** As a trader, I want to filter out low-volume trading pairs, so that I can focus on more liquid markets.

#### Acceptance Criteria

1. THE Frontend_Renderer SHALL provide a volume filter input with unit selector (K/M/B)
2. WHEN a minimum volume is set THEN the Frontend_Renderer SHALL hide signals from trading pairs with quote_volume below the threshold
3. THE volume filter SHALL be applied in real-time as Ticker_Data updates
4. THE volume filter setting SHALL be persisted in localStorage
5. THE volume filter UI SHALL be compact and fit within the existing filter row
