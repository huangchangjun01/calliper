# 前端性能基线（PERF_BASELINE）

> 用途：记录 PRD §5.3 定义的前端性能目标，以及统一测量方法，作为后续优化的对照基准。
> 本文件仅记录**目标与测量方法**，不声称已测得数据；实测数据需在迭代中补充。

## 性能目标（来自 PRD §5.3）

| 指标 | 目标值 |
| --- | --- |
| 首屏加载时间（p75） | ≤ 2000ms |
| 20 票并发实时行情推送端到端延迟 | ≤ 200ms |
| 长列表滚动帧率 | ≥ 50fps |

## 测量方法

### 首屏加载时间（P75）
- 工具：浏览器 DevTools → Network 面板，读取首屏关键资源（JS/CSS/首屏接口）的加载瀑布。
- 采集口径：取 ≥ 20 次采样后的第 75 百分位（P75），在无缓存、Network throttling=Fast 3G 下进行。
- 参考：Lighthouse Performance 跑分（移动端模拟）作为补充校验。

### 实时推送端到端延迟（20 票并发）
- 工具：浏览器 DevTools → Performance 录制 + 应用内埋点。
- 口径：从后端推送到达（WebSocket `onmessage`）到页面完成一次渲染（`requestAnimationFrame` 回调触发）的时间差；构造 20 个股票的并发推送（或模拟 50ms 窗口内的批量到达），采样取 p95。
- 预期优化手段已落地：`useWebSocket` 内对消息做 rAF 批量合并，避免逐条 `setState` 造成的多次渲染。

### 长列表滚动帧率（≥50fps）
- 工具：DevTools → Rendering → Frame Rendering Stats / FPS meter，或 `Performance` 录制滚动事件。
- 口径：连续滚动预测列表满表高度，统计平均/最低 fps。
- 预期优化手段已落地：`PredictionTable` 采用窗口化（虚拟滚动）渲染，仅渲染可视区域 ± overscan 的行，显著降低 DOM 节点数与滚动耗时。

## 测量记录占位

| 日期 | 指标 | 实测值 | 工具 | 备注 |
| --- | --- | --- | --- | --- |
| - | - | - | - | - |