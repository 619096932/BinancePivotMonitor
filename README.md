# Binance Pivot Monitor

[English](#english) | [中文](#中文)

---

## Screenshots / 截图预览

| Web Dashboard | Side Panel + TradingView |
|:---:|:---:|
| ![Browser](docs/screenshots/browser.png) | ![TradingView](docs/screenshots/tradingview.png) |

| Side Panel + Binance | iOS PWA |
|:---:|:---:|
| ![Binance](docs/screenshots/binance.png) | ![iOS](docs/screenshots/ios.jpg) |

---

## English

### Overview

Binance Pivot Monitor is a real-time cryptocurrency pivot point monitoring system for Binance USDT perpetual futures. It calculates Camarilla pivot levels and sends alerts when prices cross key support/resistance levels.

### Features

- **Real-time Monitoring**: WebSocket connection to Binance for live mark price updates
- **Camarilla Pivot Points**: Automatic calculation of R3-R5 and S3-S5 levels
- **Daily & Weekly Pivots**: Support for both timeframes with automatic refresh at 08:00 UTC+8
- **Multi-platform Alerts**:
  - Web Dashboard with SSE (Server-Sent Events)
  - Chrome Extension with sound notifications
  - Side Panel mode for persistent display alongside trading pages
- **Smart Navigation**: Click signals to jump to corresponding trading pair on TradingView or Binance
- **Binance Dark Theme**: UI styled to match Binance's dark mode
- **Signal History**: Persistent storage with configurable retention
- **Cooldown System**: Prevents duplicate alerts within 30 minutes

### Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Binance WS     │────▶│   Go Backend    │────▶│  Web Dashboard  │
│  (Mark Price)   │     │                 │     │  (SSE)          │
└─────────────────┘     │  - Pivot Calc   │     └─────────────────┘
                        │  - Signal Gen   │
                        │  - History      │     ┌─────────────────┐
                        │                 │────▶│ Chrome Extension│
                        └─────────────────┘     │  (SSE + Sound)  │
                                                └─────────────────┘
```

### Project Structure

```
.
├── cmd/server/          # Main entry point
├── internal/
│   ├── binance/         # Binance REST & WebSocket clients
│   ├── httpapi/         # HTTP API server & dashboard
│   ├── monitor/         # Price monitoring & signal generation
│   ├── pivot/           # Pivot calculation & scheduling
│   ├── signal/          # Signal types, history & cooldown
│   └── sse/             # Server-Sent Events broker
├── extension/           # Chrome extension
│   ├── icons/           # Extension icons
│   ├── background.js    # Service worker
│   ├── popup.*          # Popup UI
│   ├── options.*        # Settings page
│   ├── sidepanel.*      # Side Panel UI
│   └── offscreen.*      # SSE & audio handling
├── static/              # Web assets (favicon, icons)
├── data/                # Runtime data
│   ├── pivots/          # Cached pivot levels
│   └── signals/         # Signal history
└── packaging/           # Deployment scripts
```

### Installation

#### Prerequisites

- Go 1.21+
- Chrome/Edge browser (for extension)

#### Build from Source

```bash
# Clone repository
git clone https://github.com/your-repo/binance-pivot-monitor.git
cd binance-pivot-monitor

# Build
go build -o binance-pivot-monitor ./cmd/server

# Run
./binance-pivot-monitor
```

#### Command Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-addr` | `:8080` | HTTP server address |
| `-data-dir` | `data` | Data directory path |
| `-cors-origins` | `*` | Allowed CORS origins |
| `-binance-rest` | `https://fapi.binance.com` | Binance REST API base URL |
| `-refresh-workers` | `16` | Concurrent workers for pivot refresh |
| `-monitor-heartbeat` | `0` | Heartbeat log interval (0=disabled) |
| `-history-max` | `20000` | Maximum signals in history |
| `-history-file` | `signals/history.jsonl` | History file path |

#### Chrome Extension Installation

1. Open Chrome and navigate to `chrome://extensions/`
2. Enable "Developer mode"
3. Click "Load unpacked"
4. Select the `extension/` directory

### API Reference

#### GET /api/history

Query signal history.

**Parameters:**
- `symbol` - Filter by symbol (partial match)
- `period` - Filter by period (`1d` or `1w`)
- `level` - Filter by level(s) (`R3`, `R4`, `R5`, `S3`, `S4`, `S5`)
- `direction` - Filter by direction (`up` or `down`)
- `limit` - Maximum results (default: 200)

**Example:**
```bash
curl "http://localhost:8080/api/history?level=R4&level=S4&limit=100"
```

#### GET /api/sse

Server-Sent Events stream for real-time signals.

**Events:**
- `signal` - New signal triggered

#### GET /api/pivot-status

Get pivot data status.

**Response:**
```json
{
  "daily": {
    "updated_at": "2025-12-25T00:00:00Z",
    "next_refresh_at": "2025-12-26T00:00:00Z",
    "seconds_until": 86400,
    "is_stale": false,
    "symbol_count": 658
  },
  "weekly": { ... }
}
```

#### GET /healthz

Health check endpoint.

### Pivot Levels

The system uses Camarilla pivot points:

| Level | Formula | Description |
|-------|---------|-------------|
| R5 | (H/L) × C | Breakout resistance |
| R4 | C + Range × 1.1/2 | Strong resistance |
| R3 | C + Range × 1.1/4 | Resistance |
| S3 | C - Range × 1.1/4 | Support |
| S4 | C - Range × 1.1/2 | Strong support |
| S5 | C - (R5 - C) | Breakout support |

Where: H = High, L = Low, C = Close, Range = H - L

### Deployment

#### Systemd Service

```bash
# Build .deb package
cd packaging
./build-deb.sh

# Install
sudo dpkg -i binance-pivot-monitor_*.deb

# Configure
sudo vim /etc/binance-pivot-monitor/binance-pivot-monitor.env

# Start service
sudo systemctl enable binance-pivot-monitor
sudo systemctl start binance-pivot-monitor
```

### License

MIT License

---

## 中文

### 概述

Binance Pivot Monitor 是一个实时加密货币枢轴点监控系统，专为币安 USDT 永续合约设计。系统自动计算 Camarilla 枢轴点位，并在价格突破关键支撑/阻力位时发送警报。

### 功能特性

- **实时监控**：通过 WebSocket 连接币安获取实时标记价格
- **Camarilla 枢轴点**：自动计算 R3-R5 和 S3-S5 点位
- **日线和周线枢轴点**：支持两种时间周期，每天 UTC+8 08:00 自动刷新
- **多平台警报**：
  - Web 仪表板（SSE 实时推送）
  - Chrome 扩展（支持声音提醒）
  - Side Panel 模式（侧边栏持久显示，配合交易页面使用）
- **智能跳转**：点击信号自动跳转到 TradingView 或币安对应交易对
- **币安暗色主题**：UI 风格与币安暗色模式统一
- **信号历史**：持久化存储，可配置保留数量
- **冷却系统**：30 分钟内防止重复警报

### 系统架构

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  币安 WebSocket │────▶│   Go 后端服务   │────▶│   Web 仪表板    │
│  (标记价格)     │     │                 │     │  (SSE 推送)     │
└─────────────────┘     │  - 枢轴点计算   │     └─────────────────┘
                        │  - 信号生成     │
                        │  - 历史记录     │     ┌─────────────────┐
                        │                 │────▶│  Chrome 扩展    │
                        └─────────────────┘     │  (SSE + 声音)   │
                                                └─────────────────┘
```

### 项目结构

```
.
├── cmd/server/          # 程序入口
├── internal/
│   ├── binance/         # 币安 REST 和 WebSocket 客户端
│   ├── httpapi/         # HTTP API 服务器和仪表板
│   ├── monitor/         # 价格监控和信号生成
│   ├── pivot/           # 枢轴点计算和调度
│   ├── signal/          # 信号类型、历史和冷却
│   └── sse/             # Server-Sent Events 代理
├── extension/           # Chrome 扩展
│   ├── icons/           # 扩展图标
│   ├── background.js    # Service Worker
│   ├── popup.*          # 弹出窗口界面
│   ├── options.*        # 设置页面
│   ├── sidepanel.*      # 侧边栏界面
│   └── offscreen.*      # SSE 和音频处理
├── static/              # Web 资源（图标等）
├── data/                # 运行时数据
│   ├── pivots/          # 缓存的枢轴点数据
│   └── signals/         # 信号历史记录
└── packaging/           # 部署脚本
```

### 安装

#### 环境要求

- Go 1.21+
- Chrome/Edge 浏览器（用于扩展）

#### 从源码构建

```bash
# 克隆仓库
git clone https://github.com/619096932/BinancePivotMonitor.git
cd binance-pivot-monitor

# 构建
go build -o binance-pivot-monitor ./cmd/server

# 运行
./binance-pivot-monitor
```

#### 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-addr` | `:8080` | HTTP 服务器地址 |
| `-data-dir` | `data` | 数据目录路径 |
| `-cors-origins` | `*` | 允许的 CORS 来源 |
| `-binance-rest` | `https://fapi.binance.com` | 币安 REST API 地址 |
| `-refresh-workers` | `16` | 枢轴点刷新并发数 |
| `-monitor-heartbeat` | `0` | 心跳日志间隔（0=禁用） |
| `-history-max` | `20000` | 历史记录最大数量 |
| `-history-file` | `signals/history.jsonl` | 历史文件路径 |

#### Chrome 扩展安装

1. 打开 Chrome，访问 `chrome://extensions/`
2. 开启「开发者模式」
3. 点击「加载已解压的扩展程序」
4. 选择 `extension/` 目录

### API 接口

#### GET /api/history

查询信号历史。

**参数：**
- `symbol` - 按交易对过滤（模糊匹配）
- `period` - 按周期过滤（`1d` 或 `1w`）
- `level` - 按级别过滤（`R3`、`R4`、`R5`、`S3`、`S4`、`S5`）
- `direction` - 按方向过滤（`up` 或 `down`）
- `limit` - 最大返回数量（默认：200）

**示例：**
```bash
curl "http://localhost:8080/api/history?level=R4&level=S4&limit=100"
```

#### GET /api/sse

Server-Sent Events 实时信号流。

**事件：**
- `signal` - 新信号触发

#### GET /api/pivot-status

获取枢轴点数据状态。

**响应：**
```json
{
  "daily": {
    "updated_at": "2025-12-25T00:00:00Z",
    "next_refresh_at": "2025-12-26T00:00:00Z",
    "seconds_until": 86400,
    "is_stale": false,
    "symbol_count": 658
  },
  "weekly": { ... }
}
```

#### GET /healthz

健康检查接口。

### 枢轴点级别

系统使用 Camarilla 枢轴点：

| 级别 | 公式 | 说明 |
|------|------|------|
| R5 | (H/L) × C | 突破阻力位 |
| R4 | C + 振幅 × 1.1/2 | 强阻力位 |
| R3 | C + 振幅 × 1.1/4 | 阻力位 |
| S3 | C - 振幅 × 1.1/4 | 支撑位 |
| S4 | C - 振幅 × 1.1/2 | 强支撑位 |
| S5 | C - (R5 - C) | 突破支撑位 |

其中：H = 最高价，L = 最低价，C = 收盘价，振幅 = H - L

### 部署

#### Systemd 服务

```bash
# 构建 .deb 包
cd packaging
./build-deb.sh

# 安装
sudo dpkg -i binance-pivot-monitor_*.deb

# 配置
sudo vim /etc/binance-pivot-monitor/binance-pivot-monitor.env

# 启动服务
sudo systemctl enable binance-pivot-monitor
sudo systemctl start binance-pivot-monitor
```

### 使用说明

#### Web 仪表板

访问 `http://localhost:8080` 打开仪表板：

- **状态栏**：显示连接状态和枢轴点数据状态
- **过滤器**：
  - Symbol：按交易对搜索
  - Period：选择日线或周线
  - Direction：选择上穿或下穿
  - Levels：多选要显示的级别
- **声音提醒**：选择触发声音的级别，可开关

#### Chrome 扩展

1. 点击扩展图标打开弹出窗口
2. 在 Settings 中配置服务器地址
3. 设置 Filter Levels 过滤显示的信号
4. 设置 Sound Alert Levels 选择触发声音的级别
5. 开启/关闭声音提醒

**Side Panel 模式（推荐）**：
1. 点击弹出窗口中的 ◫ 按钮打开侧边栏
2. 侧边栏会加载 Web 仪表板，可持久显示
3. 点击信号会自动跳转到当前激活的交易页面（TradingView 或币安）
4. 适合配合交易页面一起使用

**独立窗口模式**：
1. 点击弹出窗口中的 ⧉ 按钮
2. 弹出窗口会分离成独立浮动窗口
3. 不会因点击其他地方而关闭

### 常见问题

**Q: 枢轴点数据显示 STALE？**

A: 表示数据已过期，系统会在下次 08:00 UTC+8 自动刷新。如果系统休眠后唤醒，会立即检测并刷新过期数据。

**Q: 没有收到声音提醒？**

A: 检查以下几点：
1. 确认 Sound 开关已开启
2. 确认 Sound Alert Levels 中选择了对应级别
3. 浏览器可能需要用户交互后才能播放音频

**Q: 如何关闭心跳日志？**

A: 不设置 `-monitor-heartbeat` 参数，或设置为 `0`。

### 许可证

MIT License
