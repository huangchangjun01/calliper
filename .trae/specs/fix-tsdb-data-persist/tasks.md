# Tasks — 修复行情数据未写入时序库

## Task 1: 实现时序落库公共助手
- [x] 在 `backend/internal/services/` 新增持久化助手 `tsdb_persist.go`，提供：
  - symbol → stock_id 解析（查 `stocks` 表，主库）
  - 将 `MarketData` 快照转换为 `models.StockPriceTick` 记录
  - 将交易日 OHLCV 记录转换为 `models.StockPriceDaily`
  - OnConflict upsert（幂等写入）
  - 未知 symbol 跳过并记日志
- [x] `go build ./...` 编译通过

## Task 2: 实时行情采集直写时序库
- [x] 修改 `CollectMarketDataForSymbols`：清洗后调用 `persist.PersistTicks` 直写 `stock_prices_tick`
- [x] 保留 Kafka 发布旁路：Kafka 可用时发布；失败仅记日志，不阻断落库
- [x] 修复 `StockPriceTick` 唯一索引（`idx_tick_stock_time` 改为 uniqueIndex），删表后由 AutoMigrate 重建
- [x] 验证：无 Kafka 启动后端，`stock_prices_tick` 持续写入（实测 65→115 行增长，每 5 秒 +5）
- [x] `go build ./...` + 运行验证通过

## Task 3: 历史数据回填直写时序库
- [x] 修改 `HistoryBackfill.backfill`：拿到日线后调用 `persist.PersistDaily` 直写 `stock_prices_daily`
- [x] 修复回填使用 `c.Request.Context()` 的缺陷：改用独立后台 context（`context.Background()`+30min 超时），避免请求返回后 ctx 取消导致落库失败
- [x] 修复 `StockPriceDaily` 唯一索引为 (time, stock_id) 组合，幂等写入防重复
- [x] 验证：触发回填，`stock_prices_daily` 出现 200 行真实日线；二次回填仍为 200 行（无重复）
- [x] `go build ./...` + 运行验证通过

# Task Dependencies
- Task 2、Task 3 依赖 Task 1（共用持久化助手）
- Task 2 与 Task 3 可并行开发（共用 Task 1 产物）

# 可并行执行的任务组
- Task 2、Task 3 并行（Task 1 完成后）