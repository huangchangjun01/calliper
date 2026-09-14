# Checklist — 预测功能缺陷全量修复

## 评估正确性
- [x] getActualChange 使用预测日→到期日的累计涨跌（非单日日内涨跌）
- [x] 窗口任一侧无真实行情时预测保持待验证（无行情顺延，不误判）
- [x] valid_until 与模型 horizon 对齐（3/10/30）
- [x] judgePrediction 的 flat 阈值按周期区分（±0.5/±2/±5）

## 数据源与模型
- [x] 短期预测从 TSDB stock_prices_daily 取近 60 天日线构造特征（42 行实测），无外网依赖
- [x] 特征取数失败重试 1 次，仍失败返回 no_data（绝不合成）
- [x] medium predict 输出 target_price（38.71）与 factors，字段与其他周期一致
- [x] /models/status 在权重可加载时 is_healthy=true 且 last_trained 非空

## 接口与并发
- [x] GET /predictions/{symbol} 支持 period 参数并按优先级返回首个非空周期
- [x] POST /predictions/batch 并发执行，结果顺序与请求一致，无 180s 超时风险

## Kafka 移除
- [x] kafka_consumer.go / kafka_producer.go 已删除
- [x] 全仓库（非 .venv）grep 无 kafka 引用，go.mod 无 kafka-go 依赖
- [x] 后端启动无 Kafka 相关日志，TSDB 直写日志正常

## 全链路回归
- [x] 生成预测含 no_data 分支，medium 记录含 target_price/factors
- [x] 评估按累计涨跌判定（构造 +10% 窗口数据验证 success/accuracies 符合预期）
- [x] 历史查询、分时段统计、断点趋势接口正常
- [x] 后端 go build ./...、前端 tsc --noEmit 通过
- [x] 测试完成后 ML/后端服务已关闭，模拟数据已清理