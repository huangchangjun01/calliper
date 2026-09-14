# 预测功能缺陷全量修复 Spec

## Why
代码审查发现预测模块存在 4 个 Blocker 级问题：评估用"单日涨跌"而非"窗口累计涨跌"判定对错、评估时点（valid_until 3/30/180 天）与模型预测时域（3/10/30 天）错位、短期预测的 TSDB 数据源未打通（依赖空表/外网）、Kafka 消费端是含脏数据隐患的死代码。另有 5 个 Suggestion 级问题影响展示与并发。本轮全量修复并做全链路回归。

## What Changes
- **评估口径**：`getActualChange` 改为按"预测日 → 到期日"两条日线收盘价计算**累计涨跌幅**，不再用窗口内最新一根日线的日内涨跌
- **评估时域**：`periodDays` 与模型 horizon 对齐（short=3, medium=10, long=30），修正评估时点与预测目标不一致（**BREAKING**：存量 predictions 的 valid_until 语义变化，评估将按新时域执行）
- **短期数据源**：`_build_features` 短期改读 30 天日线（interval="1d"），不再依赖空表 `stock_prices_1min` 或外网 API；特征加载失败加 1 次重试
- **Kafka 死链路移除**（**BREAKING**）：删除 `kafka_consumer.go`、producer 及两个发布调用点、配置项与 admin 的 `KafkaLag` 展示；行情落库统一走 TSDB 直写闭环
- **medium 预测补全**：`EnsemblePredictor.predict` 输出 `target_price`（按类别涨跌映射当前价）与 `factors`（多空概率），与 short/long 对齐
- **单股接口**：`GET /predictions/{symbol}` 支持 `period` 参数选择周期，缺省按 short→其余 优先返回非空周期
- **batch 并发**：`/api/v1/predictions/batch` 复用预测任务并发池，避免串行导致后端 180s 超时
- **评估阈值**：`judgePrediction` 的 flat 判定阈值按周期区分（short ±0.5，medium ±2，long ±5），与训练标签口径一致
- **模型健康状态**：`versions.json` 写入训练时间与验证集准确率；`is_healthy` 基于模型文件存在且可加载

## Impact
- Affected specs: 预测链路（生成/评估/历史/统计/趋势）
- Affected code:
  - 后端：`evaluation_service.go`、`prediction_service.go`、`market_data_service.go`、`history_backfill.go`、`cmd/gateway/main.go`、`internal/config/config.go`、`internal/handlers/admin_handler.go`、`internal/services/kafka_producer.go`(删除)、`internal/services/kafka_consumer.go`(删除)、`go.mod`(移除 kafka-go 依赖)
  - ML：`app/tasks/prediction_task.py`、`app/utils/data_loader.py`、`app/api/predictions.py`、`app/models/medium_term_model.py`、`app/models/model_manager.py`、`app/models/models.py`、`train_models.py`
  - 数据库：无 schema 变更

## ADDED Requirements
### Requirement: 窗口累计涨跌评估
评估服务 SHALL 按预测日 `predicted_at` 与到期日 `valid_until` 两条日线收盘价计算累计涨跌幅 `(close_end - close_start) / close_start * 100` 判定预测方向，任何一侧缺失真实行情时跳过该预测（保持待验证、不写入准确率）。

#### Scenario: 30 天中期预测评估
- **WHEN** 一条 medium_term 预测到期且窗口两端均有真实日线
- **THEN** 按两端收盘累计涨跌判定对错，写入 `predictions.success` 且 `prediction_accuracies` 记录实际方向

### Requirement: 评估时域与模型一致
预测落库时 SHALL 按模型 horizon 设置有效窗口：short_term=3 天、medium_term=10 天、long_term=30 天。

### Requirement: 短期预测真实数据源
短期预测 SHALL 从 TSDB `stock_prices_daily` 读取最近 30 个交易日的日线构造窗口特征；取数失败时允许 1 次重试，仍失败则该周期返回 no_data。

#### Scenario: 短期预测无数据
- **WHEN** TSDB 无该股票日线且重试/外部数据源均不可用
- **THEN** 该周期标记 no_data，不落库、不生成任何合成数据

### Requirement: 模型健康状态真实化
模型状态接口 SHALL 返回 `is_healthy=true` 当且仅当对应权重文件存在且可成功加载；`last_trained` 取权重文件修改时间，`accuracy` 取训练时记录的验证集准确率。

## MODIFIED Requirements
### Requirement: 预测接口可用性
单股预测接口 SHALL 支持 `period` 参数（short_term/medium_term/long_term），缺省返回可用的首选周期（优先级 short→medium→long）；批量预测 SHALL 并发执行以缩短响应时间。

### Requirement: 评估阈值按周期
`judgePrediction` SHALL 对 flat 判定采用与训练一致的周期阈值：short ±0.5%、medium ±2%、long ±5%。

### Requirement: medium 预测输出完整
medium_term 预测 SHALL 返回 `target_price`（基于当前价格 × 周期涨跌映射）与 `factors`（方向概率分布），字段结构与其他周期一致。

## REMOVED Requirements
### Requirement: Kafka 行情管道
**Reason**: 生产者虽配置但消费者从未启动（main.go 无调用点），Kafka 不参与任何实际数据流；消费者代码含 `StockID: 0` 硬编码，一旦启用会向时序库写入脏数据。行情落库已由 TSDB 直写（TSDBPersist）闭环。
**Migration**: 删除 `kafka_consumer.go`、`kafka_producer.go` 及发布调用点、`KAFKA_BROKERS` 配置项、admin 面板 `KafkaLag` 展示与 `kafka-go` 依赖；docker-compose 中的 Kafka/ZK 服务保留由用户自行决定启停，不影响后端运行。