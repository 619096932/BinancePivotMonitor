# Implementation Plan: PC 宽屏多面板布局

## Overview

本实现计划将 PC 宽屏多面板功能分解为增量式的编码任务。主要修改集中在 `internal/httpapi/static/index.html` 和 `internal/httpapi/static/app.js` 两个文件。

## Tasks

- [x] 1. 添加多面板 HTML 结构
  - 在 `index.html` 中添加 `multiPanelContainer` 容器
  - 添加三个面板的 HTML 结构（signal-panel, volume-panel, trades-panel）
  - 每个面板包含 header 和 scroll area
  - _Requirements: 5.2_

- [x] 2. 添加宽屏布局 CSS 样式
  - 添加 `.multi-panel-container` 基础样式（默认隐藏）
  - 添加 `@media (min-width: 1201px)` 媒体查询
  - 在媒体查询中隐藏 tabs 和原有 scroll-area
  - 添加面板 flexbox 布局样式
  - 添加面板 header 和 scroll 样式
  - _Requirements: 5.1, 5.3, 5.4_

- [x] 3. 实现布局检测模块
  - [x] 3.1 添加 `WIDESCREEN_BREAKPOINT` 常量和 `isWidescreen` 状态变量
    - _Requirements: 1.1, 1.2_
  - [x] 3.2 实现 `checkWidescreenMode()` 函数
    - 检测 viewport 宽度并更新 isWidescreen 状态
    - 调用 `updateLayoutMode()` 切换布局
    - _Requirements: 1.1, 1.2_
  - [x] 3.3 在 `init()` 中添加 resize 事件监听
    - 使用 debounce 防抖处理
    - _Requirements: 1.3_

- [x] 4. 实现宽屏 Clusterize 实例管理
  - [x] 4.1 添加三个宽屏 Clusterize 实例变量
    - `wideSignalCluster`, `wideVolumeCluster`, `wideTradesCluster`
    - _Requirements: 6.3_
  - [x] 4.2 实现 `initWidescreenClusters()` 函数
    - 初始化三个独立的 Clusterize 实例
    - 配置各自的 scrollId 和 contentId
    - _Requirements: 6.3_
  - [x] 4.3 实现 `destroyWidescreenClusters()` 函数
    - 销毁 Clusterize 实例释放资源
    - _Requirements: 6.3_

- [x] 5. 实现面板数据更新函数
  - [x] 5.1 实现 `updateWideSignalPanel()` 函数
    - 复用 `renderSignalItem()` 渲染信号列表
    - 更新面板计数显示
    - _Requirements: 2.1, 2.3, 2.4_
  - [x] 5.2 实现 `updateWideVolumePanel()` 函数
    - 调用 `loadRanking('volume')` 加载数据
    - 复用 `renderRankingItemFromAPI()` 渲染排行
    - 更新面板计数显示
    - _Requirements: 3.1, 3.3, 3.4_
  - [x] 5.3 实现 `updateWideTradesPanel()` 函数
    - 调用 `loadRanking('trades')` 加载数据
    - 复用 `renderRankingItemFromAPI()` 渲染排行
    - 更新面板计数显示
    - _Requirements: 4.1, 4.3, 4.4_
  - [x] 5.4 实现 `updateWidescreenPanels()` 统一更新函数
    - 调用三个面板的更新函数
    - _Requirements: 6.4_

- [x] 6. 实现布局模式切换
  - [x] 6.1 实现 `updateLayoutMode()` 函数
    - 根据 isWidescreen 切换显示/隐藏元素
    - 宽屏时初始化 Clusterize 并加载数据
    - 窄屏时恢复原有视图
    - _Requirements: 1.1, 1.2, 1.3, 1.4_

- [x] 7. 集成事件绑定
  - [x] 7.1 实现 `bindWideSignalEvents()` 函数
    - 为宽屏信号列表项绑定点击事件
    - 复用现有的 `showActionMenu()` 逻辑
    - _Requirements: 6.2_
  - [x] 7.2 实现 `bindWideVolumeEvents()` 和 `bindWideTradesEvents()` 函数
    - 为排行列表项绑定点击事件
    - 复用现有的 `showRankingDetail()` 逻辑
    - _Requirements: 6.2_

- [x] 8. 修改现有函数以支持宽屏模式
  - [x] 8.1 修改 `applyFilters()` 触发宽屏面板更新
    - 在过滤后检查 isWidescreen 并更新面板
    - _Requirements: 2.3, 2.4, 3.3, 4.3_
  - [x] 8.2 修改 SSE 数据更新回调
    - 在收到新数据时更新宽屏面板
    - _Requirements: 2.1, 3.1, 4.1_

- [x] 9. 添加国际化支持
  - 在 I18N 对象中添加面板标题翻译
  - `panel_signals`, `panel_volume`, `panel_trades`
  - _Requirements: 5.2_

- [x] 10. Checkpoint - 功能验证
  - 确保所有功能正常工作
  - 在不同屏幕宽度下测试布局切换
  - 验证三个面板数据独立显示和滚动
  - 如有问题请询问用户

- [x] 11. 编写属性测试
  - [x] 11.1 编写布局模式响应断点属性测试
    - **Property 1: 布局模式响应断点**
    - **Validates: Requirements 1.1, 1.2, 7.1**
  - [x] 11.2 编写面板等宽分布属性测试
    - **Property 6: 面板等宽分布**
    - **Validates: Requirements 5.1, 5.4**

## Notes

- 所有任务都必须完成
- 每个任务引用具体的需求条款以确保可追溯性
- Checkpoint 任务用于增量验证功能正确性
- 属性测试验证系统的通用正确性属性
