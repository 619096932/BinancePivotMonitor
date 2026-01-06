# Requirements Document

## Introduction

本功能为枢轴监控系统的 PC 宽屏场景提供多面板并排显示能力。在宽屏设备上，用户可以同时查看信号、成交额排行、成交笔数排行三个独立窗口，无需通过 tab 切换，提升信息获取效率。

## Glossary

- **Multi_Panel_Layout**: 多面板布局系统，负责在宽屏场景下并排显示多个独立数据面板
- **Signal_Panel**: 信号面板，显示枢轴点触发信号列表
- **Volume_Panel**: 成交额面板，显示成交额排行榜
- **Trades_Panel**: 成交笔数面板，显示成交笔数排行榜
- **Breakpoint**: 响应式断点，用于判断是否启用宽屏多面板模式
- **Panel_Container**: 面板容器，包含单个数据列表及其独立的滚动区域

## Requirements

### Requirement 1: 宽屏检测与布局切换

**User Story:** As a PC user, I want the system to automatically detect my screen width and switch to multi-panel layout, so that I can view multiple data streams simultaneously without manual configuration.

#### Acceptance Criteria

1. WHEN the viewport width exceeds 1200px, THE Multi_Panel_Layout SHALL activate and display three panels side by side
2. WHEN the viewport width is 1200px or less, THE Multi_Panel_Layout SHALL deactivate and revert to the original tab-based single panel view
3. WHEN the window is resized across the breakpoint, THE Multi_Panel_Layout SHALL smoothly transition between layouts without data loss
4. THE Multi_Panel_Layout SHALL preserve user's filter settings during layout transitions

### Requirement 2: 信号面板独立显示

**User Story:** As a trader, I want to see the signal list in its own dedicated panel, so that I can monitor pivot signals while viewing other data.

#### Acceptance Criteria

1. WHILE Multi_Panel_Layout is active, THE Signal_Panel SHALL display the filtered signal list independently
2. THE Signal_Panel SHALL maintain its own scroll position independent of other panels
3. THE Signal_Panel SHALL apply the global symbol filter from the header search input
4. THE Signal_Panel SHALL support all existing signal filtering options (period, direction, levels, diff%, volume)

### Requirement 3: 成交额排行面板独立显示

**User Story:** As a trader, I want to see the volume ranking in its own dedicated panel, so that I can track high-volume symbols while monitoring signals.

#### Acceptance Criteria

1. WHILE Multi_Panel_Layout is active, THE Volume_Panel SHALL display the volume ranking list independently
2. THE Volume_Panel SHALL maintain its own scroll position independent of other panels
3. THE Volume_Panel SHALL apply the global symbol filter from the header search input
4. THE Volume_Panel SHALL support ranking comparison and sorting options

### Requirement 4: 成交笔数排行面板独立显示

**User Story:** As a trader, I want to see the trades ranking in its own dedicated panel, so that I can identify actively traded symbols while monitoring other data.

#### Acceptance Criteria

1. WHILE Multi_Panel_Layout is active, THE Trades_Panel SHALL display the trades ranking list independently
2. THE Trades_Panel SHALL maintain its own scroll position independent of other panels
3. THE Trades_Panel SHALL apply the global symbol filter from the header search input
4. THE Trades_Panel SHALL support ranking comparison and sorting options

### Requirement 5: 面板布局与样式

**User Story:** As a user, I want the three panels to be visually distinct and properly sized, so that I can easily distinguish between different data types.

#### Acceptance Criteria

1. THE Multi_Panel_Layout SHALL distribute available width equally among three panels
2. EACH Panel_Container SHALL have a visible header indicating its content type (信号/成交额/成交笔数)
3. EACH Panel_Container SHALL have a distinct border or separator from adjacent panels
4. THE Multi_Panel_Layout SHALL ensure minimum panel width of 300px per panel
5. IF viewport width is insufficient for three 300px panels, THEN THE Multi_Panel_Layout SHALL fall back to tab-based view

### Requirement 6: 数据独立性

**User Story:** As a user, I want each panel to operate independently, so that interactions in one panel don't affect the others.

#### Acceptance Criteria

1. WHEN a user scrolls in one Panel_Container, THE other Panel_Containers SHALL maintain their scroll positions
2. WHEN a user clicks an item in one panel, THE action menu SHALL appear without affecting other panels
3. EACH Panel_Container SHALL have its own Clusterize instance for virtual scrolling
4. THE Multi_Panel_Layout SHALL load data for all three panels simultaneously on initialization

### Requirement 7: 移动端兼容性

**User Story:** As a mobile user, I want the original tab-based interface to remain unchanged, so that I can continue using the app as before.

#### Acceptance Criteria

1. WHEN viewport width is 1200px or less, THE system SHALL display the original tab-based interface
2. THE tab buttons SHALL remain visible and functional in narrow viewport mode
3. THE original single-panel scrolling behavior SHALL be preserved in narrow viewport mode
