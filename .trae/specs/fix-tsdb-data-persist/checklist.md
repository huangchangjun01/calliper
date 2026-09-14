# Checklist — 修复行情数据未写入时序库

## Task 1: 落库公共助手
- [x] 时序落库助手存在：symbol→stock_id 解析、MarketData→tick 转换、日线→daily 转换、OnConflict upsert、未知 symbol 跳过
- [x] （验证方式：编译通过；代码审查）

## Task 2: 实时行情直写
- [x] 实时采集链路在清洗后直写 `stock_prices_tick`（无 Kafka 也可落库）
- [x] Kafka 发布保留为旁路且失败不阻断落库
- [x] 运行时无 Kafka 启动后端，`stock_prices_tick` 有数据（验证方式：实测表内行数 65→115 持续增长，日志 `[TSDBPersist] Persisted 5 tick records`）

## Task 3: 历史回填直写
- [x] 回填日线数据直写 `stock_prices_daily`，checkpoint 逻辑仍生效
- [x] 重复回填幂等（OnConflict+唯一索引），不产生重复行
- [x] 运行时触发回填，`stock_prices_daily` 有数据且二次回填无重复（验证方式：两次回填均为 200 行）

## 总体验证
- [x] 后端 `go build ./...` 编译通过
- [x] 时序库（calliper_tsdb）出现行情数据：stock_prices_daily=200 行（真实日线）、stock_prices_tick 持续增长，预测评估链路（EvaluateDaily）恢复数据源