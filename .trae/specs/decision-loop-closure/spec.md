# 决策支持闭环 Spec

## Why
系统各功能模块（预测、评估、模拟交易）已存在但彼此割裂，无法形成"预测 → 评估自验证 → 模拟交易"的可信决策支持链路，导致对研究与策略验证用户的高价值产出有限。

## What Changes
- 打通"预测 → 评估自校验 → 模拟交易"决策闭环，链路每日自动产出并在页面可见
- 已到期预测 100% 完成"待验证 → 已对/已错"状态回填，且状态仅由评估模块写入
- 新增决策支持仪表盘，聚合今日高置信度预测、昨准率摘要、风险提示、系统状态
- 成功率自评估、失败归因、模拟交易指标（超额收益/夏普/最大回撤/权益曲线）可视化
- 前端统一交互规范（三态、防重复、断线重连、筛选状态保持）与性能 SLA 落地
- **BREAKING**: 无（在既有模块上补齐闭环与可视化）

## Impact
- Affected specs: quant-trading-system（prediction/evaluation/sim-trade/前端展示）
- Affected code:
  - backend: evaluation_service.go、prediction_handler.go、evaluation_handler.go、sim_trade_service.go、sim_trade_handler.go、dashboard 聚合接口（新增）、scheduler 配置
  - frontend: Dashboard.tsx、Predictions.tsx、StockAccuracyChart/AccuracyChart/index.tsx、FailureAnalysis/index.tsx、SimTradePanel/index.tsx、投资组合图表（新增）、通用交互与性能公共组件

---

## ADDED Requirements

### Requirement: 决策支持仪表盘
系统 SHALL 提供决策支持仪表盘，聚合今日高置信度预测、昨日预测准确率摘要、风险提示、系统状态（数据同步/模型/数据源）。

#### Scenario: 开市前查看今日关注
- **WHEN** 用户进入仪表盘
- **THEN** 展示高置信度标的（方向+置信度≥阈值）、7/30/累计准确率、连续失败/回撤触发等风险提示，以及数据同步时间、模型与数据源健康状态

#### Scenario: 局部容错
- **WHEN** 某数据源超时或 ML 服务不可用
- **THEN** 对应卡片显示"数据暂缺/降级"提示，不影响其余卡片加载

### Requirement: 预测面板"待验证/已验证"状态
系统 SHALL 为每条预测维护状态：未到期=待验证；到期后由评估回填为 已对/已错。该状态仅由评估模块写入，用户不可改。

#### Scenario: 预测到期自动回填
- **WHEN** 预测到期且当日有实际行情
- **THEN** 评估任务将预测状态回填为 已对/已错 并高亮展示

#### Scenario: 无行情顺延
- **WHEN** 到期的预测当日无实际行情
- **THEN** 状态保持"待验证"，不误判失败

### Requirement: 模拟交易指标可视化
系统 SHALL 在模拟交易面板可视化 权益曲线、超额收益（对比基准）、夏普比率、最大回撤，并提供交易决策日志（含预测依据展开）。

#### Scenario: 查看策略表现
- **WHEN** 用户查看模拟交易面板
- **THEN** 展示账户总览、持仓、权益曲线、超额收益对比、夏普、最大回撤与决策日志

### Requirement: 前端统一交互规范与性能
系统 SHALL 在所有数据区块实现 加载态/内容态/空态 三态；写操作防重复提交；WebSocket 断线自动重连并增量刷新；筛选/Tab 状态在路由间保持；列表渲染流畅。

#### Scenario: 断线重连
- **WHEN** WebSocket 连接断开
- **THEN** 前端展示连接状态并自动重连，恢复后增量刷新数据

#### Scenario: 性能达标
- **WHEN** 监控 20 只票并发刷新
- **THEN** 实时推送数据产生到前端可见 ≤ 200ms，列表滚动无卡顿（≥50fps）

---

## MODIFIED Requirements

### Requirement: 成功率自评估可视化
原评估仅将指标存库无可视化。本版本 SHALL 在页面展示 近7日/30日/累计 方向准确率、分周期准确率、整体胜率排行，并支持下钻历史对错时间线。

#### Scenario: 胜率排行
- **WHEN** 用户查看评估面板
- **THEN** 展示按准确率降序的胜率排行，样本不足（<阈值）标注"样本不足"