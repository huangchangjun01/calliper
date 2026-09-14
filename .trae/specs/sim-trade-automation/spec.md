# 模拟交易自动化 Spec

## Why
当前交易面板中的模拟交易依赖用户手动点击"启动/停止"，决策依据不透明，且面板不展示模拟账户的收益情况，无法体现"预测 → 模拟交易 → 收益"的价值闭环。需要让模拟交易**自动进行**、策略**按预测分析结果执行**，并在交易面板**展示收益情况**。

## What Changes
- **自动运行**：模拟交易调度器随后端启动自动常驻运行，启动即立即执行首轮决策；交易时段每 15 分钟自动决策，收盘后自动结算；移除"账户停止即退出"的自动停止逻辑
- **策略按预测结果**：决策引擎优先从本地 `predictions` 表读取有效期内（`valid_until > now()` 且未验证）的预测记录作为决策依据（每只股票取最新、同批次优先 short>medium>long），仅当本地无有效预测时降级调用 ML 预测服务
- **收益计算**：`simulated_trades` 记录新增 `profit`（已实现盈亏）与 `profit_rate`（盈亏率）字段，卖出成交时按持仓均价计算并落库（GORM AutoMigrate 自动迁移）
- **完整状态接口**：`GET /api/v1/trading/sim/status` 返回完整 `SimStatus`（running/account/decisions/records/risk_control），account 含累计收益与累计收益率、初始资金；新增 `POST /api/v1/trading/sim/trigger` 手动触发一轮决策
- **前端收益展示**：交易面板"模拟交易"标签页账户卡片展示 总资产/可用资金/持仓市值/今日盈亏/累计收益（含累计收益率），交易记录表展示每笔盈亏与盈亏率
- **BREAKING**: 无（在既有模拟交易模块上增强）

## Impact
- Affected specs: decision-loop-closure（模拟交易联动与可视化）、quant-trading-system（sim-trade 能力）
- Affected code:
  - backend: `models/models.go`（SimulatedTrade+Profit/ProfitRate、SimAccount+InitialCapital）、`services/account_service.go`、`services/sim_trade_service.go`（决策数据源/调度器/GetStatus）、`handlers/sim_trade_handler.go`、`cmd/gateway/main.go`（/trigger 路由）
  - frontend: `pages/TradingPanel.tsx`（SimStatusResponse 映射与轮询）、`components/SimTradePanel/index.tsx`（收益卡片/盈亏列）、`pages/TradingPanel.css`、`types/index.ts`

---

## ADDED Requirements

### Requirement: 模拟交易自动运行
系统 SHALL 在后端启动时自动启动模拟交易调度器并保持常驻运行，无需用户手动开启；启动后立即执行首轮决策周期，交易时段按固定间隔自动决策，收盘后自动执行当日结算。

#### Scenario: 后端启动即自动开跑
- **WHEN** 后端服务启动完成
- **THEN** 调度器自动启动并立即执行一轮决策，日志输出"启动后立即执行首轮模拟交易决策"

#### Scenario: 交易日自动决策
- **WHEN** 处于 A 股交易时段（周一至周五 9:30-11:30 / 13:00-15:00）
- **THEN** 每 15 分钟自动执行一轮决策周期，依据当前有效预测生成买卖指令

#### Scenario: 收盘自动结算
- **WHEN** 当日收盘后
- **THEN** 自动执行一次盘后结算，更新持仓市值、今日盈亏、总资产与收益率

### Requirement: 模拟交易策略按预测分析结果执行
系统 SHALL 以本地 `predictions` 表中有效期内、未验证的预测记录作为模拟交易决策的主要数据源；上涨（up）且置信度≥60% 触发买入、下跌（down）且置信度≥60% 触发卖出；置信度<55% 的预测不参与排序选股；本地无有效预测时降级调用 ML 预测服务；仍无结果则本轮不生成交易决策（无信号不交易）。

#### Scenario: 依据预测信号买入
- **WHEN** 某股票存在有效期内 direction=up 且置信度≥60% 的预测，且资金/仓位/行业约束均满足
- **THEN** 生成买入决策并按 100 股整手成交，决策日志关联该预测记录 ID

#### Scenario: 依据预测信号卖出
- **WHEN** 某已持仓股票出现有效期内 direction=down 且置信度≥60% 的预测
- **THEN** 生成卖出决策（默认减半仓），卖出时按持仓均价计算已实现盈亏并落库

#### Scenario: 无有效预测
- **WHEN** 本地与 ML 服务均无满足条件的有效预测
- **THEN** 本轮生成 0 条决策，不进行任何交易

### Requirement: 模拟交易收益展示
系统 SHALL 在交易面板模拟交易页展示模拟账户收益情况：总资产、可用资金、持仓市值、今日盈亏与今日收益率、累计收益（总资产−初始资金）与累计收益率；交易记录表 SHALL 展示每笔交易的盈亏金额与盈亏率，按红涨绿跌着色。

#### Scenario: 查看模拟账户收益
- **WHEN** 用户进入交易面板"模拟交易"标签页
- **THEN** 展示账户收益卡片（含累计收益与累计收益率）与包含盈亏列的交易记录表

### Requirement: 完整状态接口与手动触发
系统 SHALL 提供 `GET /api/v1/trading/sim/status` 返回完整模拟交易状态（运行状态、账户含收益指标、决策列表、交易记录、风险控制），并提供 `POST /api/v1/trading/sim/trigger` 支持手动触发一轮决策周期（便于演示与联调）。

#### Scenario: 查询完整状态
- **WHEN** 前端请求模拟交易状态
- **THEN** 接口返回 running/account（含 total_profit、total_profit_percent、initial_capital）/decisions/records/risk_control 完整结构

---

## MODIFIED Requirements

### Requirement: 模拟交易记录盈亏字段
原 `simulated_trades` 仅记录成交信息无盈亏。本版本 SHALL 为每条卖出记录计算并落库 `profit`（卖出价−持仓均价）× 卖出数量）与 `profit_rate`（盈亏率百分比），买入记录盈亏为 0。

#### Scenario: 卖出成交落库盈亏
- **WHEN** 模拟交易引擎执行卖出成交
- **THEN** 交易记录中写入本次卖出的已实现盈亏金额与盈亏率，前端交易记录表可见

### Requirement: 决策数据源优先级
原决策引擎只从 ML 预测服务获取结果。本版本 SHALL 优先读取本地 `predictions` 表仍有效的预测记录，ML 服务仅作降级兜底，确保策略严格按"预测分析结果"执行。

#### Scenario: 本地预测优先
- **WHEN** 本地 predictions 表存在仍有效的预测记录
- **THEN** 决策引擎直接使用本地记录，不调用 ML 服务

## REMOVED Requirements
无