# 修复行情数据未写入时序库 Spec

## Why
`calliper_tsdb`（TimescaleDB）中没有任何行情数据。当前行情采集"只采不存"：实时采集与历史回填获取数据后仅发布到 Kafka，而写时序库的唯一消费者 `KafkaConsumer` 从未被启动，且本地无 Kafka broker（Producer 走 no-op）。导致预测评估（`EvaluateDaily`）、准确率回填、模拟交易等依赖时序数据的整条链路无法生效。

## What Changes
- 实时行情采集：采集并清洗后**直接落库到时序库**（tick/分钟），不再依赖 Kafka 中转
- 历史数据回填：回填器获取日线数据后**直接落库**（而非仅发布 Kafka）
- 保留 Kafka 发布作为可选的旁路，但与直写时序库互不阻塞（Kafka 缺失时数据不再丢）
- **BREAKING**: 无（新增直写路径，不改动数据库表结构；不改动既有 Kafka 路径）

## Impact
- Affected specs: quant-trading-system（数据采集与存储）、decision-loop-closure（评估/回填依赖时序数据）
- Affected code:
  - backend/internal/services/market_data_service.go — 采集循环落库
  - backend/internal/services/history_backfill.go — 回填落库
  - backend/internal/services/market_data_collector.go 或新增持久化助手 — 落库公共逻辑（symbol→stock_id 解析、OnConflict upsert）
  - backend/cmd/gateway/main.go — 若需要可选的 KafkaConsumer 启动判定（保持现状亦可）

---

## ADDED Requirements

### Requirement: 实时行情直写时序库
系统 SHALL 在每次行情采集并清洗后，将数据直接写入 TimescaleDB，而不依赖 Kafka。

#### Scenario: 无 Kafka 时数据仍入库
- **WHEN** 采集器返回实时行情数据且 `KAFKA_BROKERS` 为空
- **THEN** 系统将数据写入 `stock_prices_tick`（或 `stock_prices_1min`）时序表，且不报错

#### Scenario: 重复采集不产生重复行
- **WHEN** 同一股票同一时刻的行情被多次采集
- **THEN** 落库使用 upsert（OnConflict），不产生重复记录

### Requirement: 历史回填直写时序库
系统 SHALL 在历史数据回填获取到日线/分钟数据后，直接写入 TimescaleDB。

#### Scenario: 回填日线数据落库
- **WHEN** 对某股票执行日线回填且数据源返回日线记录
- **THEN** 系统将各交易日的 OHLCV 写入 `stock_prices_daily`，重复回填可幂等覆盖

### Requirement: 落库公共逻辑
系统 SHALL 提供统一的时序落库辅助：按 symbol 解析 stock_id、按表构造记录、幂等写入。

#### Scenario: 未知 symbol
- **WHEN** 采集/回填的数据中某 symbol 不在 `stocks` 表
- **THEN** 该条数据被跳过并记录日志，不影响其余数据落库

---

## MODIFIED Requirements

### Requirement: 数据采集链路（原仅 Kafka）
原采集链路为 采集 → Kafka 发布（依赖 broker）。现改为 采集 → 清洗 → **直写时序库**，Kafka 发布保留为可选旁路。当 `KafkaProducer` 可用时才发布，失败仅记日志，不阻断落库。

### Requirement: 数据采集与存储规范（Spec 原稿）
原 spec 以 Kafka 为唯一入库通道，本变更补充"无 Kafka 时直写 TSDB"的降级路径，保证本地/无中间件环境数据可用。