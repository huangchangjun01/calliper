# Tasks — 预测功能缺陷全量修复

## Task 1: 评估口径与时域修复（Go 后端）
- [x] `getActualChange`：改为查询 `[predicted_at, valid_until)` 窗口的**首尾两条**日线，返回 `(close_end - close_start) / close_start * 100`；无端侧数据或价格无效时 `ok=false`（无行情顺延，保持待验证）
  - 子项：保留窗口内首条按 `time ASC`、末条按 `time DESC`；同一 SQL 批次内完成避免两次往返
- [x] `periodDays`：medium 30→**10**、long 180→**30**（与模型 horizon 对齐）
- [x] `judgePrediction(pred.Direction, ...)` 增加周期参数，flat 判定阈值按周期：short ±0.5 / medium ±2 / long ±5（百分数）；`EvaluateDaily` 调用处传入 `pred.Period`
- [x] 验证：`go build ./...` 通过；grep 确认 `getActualChange` 不再使用单日 `(close-open)/open` 口径（端到端：构造 10% 窗口涨跌判为 correct ✓；无行情窗口保持待验证 ✓）

## Task 2: 短期预测数据源与重试修复（ML Python）
- [x] `_build_features` 短期周期改 `interval="1d"`、窗口 60 天（保证 >=30 根日线），`medium` 保持 1d、`long` 保持 365 天 1d
- [x] `_load_with_timeout`：同一 symbol 取数失败后重试 1 次（共 2 次尝试，单次超时仍 8s）
- [x] 验证：000001 短期特征经 TSDB `stock_prices_daily` 取到 42 行真实日线（无外网依赖）；generate 时 short 预测正常产出

## Task 3: 模型输出补全与接口并发化（ML Python）
- [x] `EnsemblePredictor.predict` 输出增加 `target_price`（按 LABELS 类别映射当前价 × 周期涨跌：跌 0.98/平 1.00/涨 1.02 参考 short/long 风格）与 `factors`（probabilities 转列表）；`_get_current_price_from_data` 逻辑复用
- [x] `GET /predictions/{symbol}` 支持 `period` 查询参数；未指定时按 short→medium→long 顺序返回首个非空周期（当前仅 short 的逻辑替换）
- [x] `POST /predictions/batch`：改用 `ThreadPoolExecutor(max_workers=8)` 并发预测，保持结果按请求顺序返回
- [x] 验证：medium `predict` 返回 `target_price=38.71>0` 与 `factors` 列表；`GET /{symbol}?period=medium_term` 返回 medium 结果；generate 结果顺序与请求一致（修复了 factors 由 dict 改 list 后 `.items()` 崩溃的回归 bug）

## Task 4: 模型健康状态真实化（ML Python）
- [x] `train_models.py`：训练后对每个模型计算验证集准确率并连同 `trained_at`（UTC ISO）写入 `versions.json`（替代 accuracy 恒 0）
- [x] `model_manager.py load_all`：加载时记录各模型最后训练时间（来自 versions.json 或权重文件 mtime）；`models.py status` 接口 `is_healthy` = 文件存在且加载成功；`last_trained` 使用记录值，`accuracy` 使用 versions.json 值
- [x] 验证：`GET /api/v1/models/status` 三模型均返回 `is_healthy=true`、`last_trained` 非空（取权重 mtime）

## Task 5: Kafka 死链路移除（Go 后端，BREAKING）
- [x] 删除 `internal/services/kafka_consumer.go` 与 `internal/services/kafka_producer.go`
- [x] 移除调用点：`market_data_service.go` 中 Kafka 发布与 `kafkaProd` 字段、`history_backfill.go` 中 Kafka 发布；`cmd/gateway/main.go` 中 Kafka brokers 配置与注入
- [x] 移除 `config.go` 的 `KafkaBrokers` 字段与 `KAFKA_BROKERS` 读取；`admin_handler.go` 的 `KafkaLag` 字段与展示值；前端 `admin.ts`/`SystemMonitor` 的 kafkaLag 展示；`go.mod`/`go.sum` 移除 `github.com/segmentio/kafka-go`
- [x] 验证：`go build ./...` 通过；grep 无 `kafka` 残留；新版后端启动日志无 Kafka 输出、`[TSDBPersist] Persisted N tick records` 持续可见；前端 `tsc --noEmit` 通过

## Task 6: 全链路回归测试（端到端）
- [x] 环境准备：启动 ML(8000, venv) 与后端(8080)；`/models/status` 三模型健康
- [x] 数据/模型：确认 TSDB 有真实日线；复用现有 v2.0.0-real 权重（medium 输出变化无需重训，预测逻辑兼容）
- [x] 生成：`POST /predictions/generate`（000001/600036/600519）→ 前两者三周期落库 6 条、600519 返回 no_data 0 条；medium 记录 `target_price` 非 0、`factors` 非空；valid_until 时域 3/10/30 天正确
- [x] 评估：构造到期预测 + 精确窗口日线（10% 累计涨跌）触发 `EvaluateDaily` → `success=true`、`actual=bullish`；无行情窗口 → 保持待验证、不写 accuracies
- [x] 查询：`/predictions/history`（分页+筛选）、`/predictions/stats`（day 断点 null）、`/predictions/accuracy`（断点序列 + 评估结果回流）均正常
- [x] 构建与收尾：后端 `go build ./...`、前端 `tsc --noEmit` 通过；ML/后端测试服务已关闭、临时编译产物已删除、测试伪数据（测试预测/TSDB 测试日线/测试用户/accuracies）已清理

# Task Dependencies
- Task 1（Go 评估）与 Task 2-4（ML）无依赖，可并行
- Task 5（Kafka 移除，Go）与 Task 1 同在后端，建议同一实现批次串行执行避免文件冲突
- Task 6 依赖 Task 1–5 全部完成

# 可并行执行的任务组
- 组 A：Task 2、Task 3、Task 4（ML 侧，互相独立）
- 组 B：Task 1 + Task 5（后端侧，串行）
- 组 C：Task 6（全链路回归，最后执行）