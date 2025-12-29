# Implementation Plan: Pivot Enhancements

## Overview

本实现计划将功能增强分为后端和前端两个主要部分，按照依赖关系顺序实现。后端先完成 API 和监控逻辑，前端再实现渲染和交互功能。

## Tasks

- [x] 1. 后端：全点位触发推送
  - [x] 1.1 扩展 Monitor.checkPeriod 检查全部 11 个点位
    - 修改 `internal/monitor/monitor.go` 的 `checkPeriod` 方法
    - 添加 PP, R1, R2, S1, S2 点位检查
    - _Requirements: 1.1, 1.5_
  - [x] 1.2 编写点位穿越检测的属性测试
    - **Property 1: Level Crossing Detection**
    - **Validates: Requirements 1.1, 1.7, 1.9**
  - [x] 1.3 编写冷却隔离的属性测试
    - **Property 2: Cooldown Isolation**
    - **Validates: Requirements 1.4, 1.6**
  - [x] 1.4 编写首次价格基线的属性测试
    - **Property 8: First Price Baseline**
    - **Validates: Requirements 1.8**

- [x] 2. 后端：信号存储扩容
  - [x] 2.1 修改 Signal History Query 限制为 4000
    - 修改 `internal/signal/history.go` 的 `Query` 方法
    - _Requirements: 2.1, 2.4_
  - [x] 2.2 编写历史容量限制的属性测试
    - **Property 3: Signal History Capacity**
    - **Validates: Requirements 2.1, 2.2**

- [x] 3. 后端：枢轴点 API
  - [x] 3.1 实现 /api/pivots/{symbol} 端点
    - 在 `internal/httpapi/server.go` 添加 `handlePivots` 方法
    - 支持 period 参数 (1d/1w)
    - 返回完整的枢轴点数据
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_
  - [ ] 3.2 编写 API 完整性的属性测试
    - **Property 4: Pivot API Completeness**
    - **Validates: Requirements 3.5**

- [x] 4. Checkpoint - 后端测试验证
  - 运行 `go test ./...` 确保所有测试通过
  - 如有问题请询问用户

- [x] 5. 前端：枢轴点缓存和 API 客户端
  - [x] 5.1 实现 pivotCache Map 和 getPivotData 函数
    - 修改 `internal/httpapi/static/app.js`
    - 实现 5 分钟缓存逻辑
    - _Requirements: 6.5, 7.3_

- [x] 6. 前端：信号项枢轴点位渲染
  - [x] 6.1 实现 findNearestLevels 和 formatPivotLevel 函数
    - 计算当前价格上下方最近的点位
    - 格式化显示：日:R4(-0.5%) 周:S5(1%)
    - _Requirements: 4.1, 4.2, 4.3_
  - [x] 6.2 实现 Intersection Observer 优化
    - 只为可视区域内的项目渲染枢轴点位
    - 离开可视区域时停止更新
    - _Requirements: 4.7, 4.8_
  - [x] 6.3 集成到 renderSignalItem 和 updateVisibleItems
    - 在信号项中添加枢轴点位行
    - 实时更新百分比
    - _Requirements: 4.4, 4.5, 4.6_

- [x] 7. 前端：枢轴点预览 Modal
  - [x] 7.1 实现 renderPivotPreview 函数
    - 按价格降序排列所有点位
    - 计算相对于当前价格的百分比
    - _Requirements: 5.2, 5.3, 5.4_
  - [x] 7.2 实现 Modal HTML 和 CSS
    - 显示所有点位和当前价格指示器
    - 支持日/周切换
    - _Requirements: 5.5, 5.6, 5.7, 5.10_
  - [x] 7.3 集成到操作菜单
    - 点击 "Pivot Levels" 显示枢轴点预览
    - _Requirements: 5.1_

- [x] 8. 前端：成交额过滤
  - [x] 8.1 添加成交额过滤 UI 组件
    - 输入框 + 单位选择器 (K/M/B)
    - 紧凑设计，放在过滤行中
    - _Requirements: 8.1, 8.5_
  - [x] 8.2 实现 parseVolumeInput 和过滤逻辑
    - 修改 matchSignal 函数
    - _Requirements: 8.2, 8.3_
  - [x] 8.3 实现 localStorage 持久化
    - 保存和加载成交额过滤设置
    - _Requirements: 8.4_

- [x] 9. 前端：Level 过滤按钮扩展
  - [x] 9.1 添加 PP, R1, R2, S1, S2 过滤按钮
    - 修改 `internal/httpapi/static/index.html` 的 filterLevels 和 soundLevels
    - _Requirements: 1.1_

- [x] 10. Checkpoint - 核心功能测试验证
  - 所有后端测试通过
  - 枢轴点预览 Modal 功能完成
  - Level 过滤按钮扩展完成

## Notes

- 所有任务均为必需，包括测试任务
- 后端任务 (1-4) 应先完成，前端任务 (5-9) 依赖后端 API
- 属性测试使用 Go 的 testing/quick 或 gopter
- 前端测试可以使用简单的 console 测试或 Jest
