# Checklist — 预测历史记录管理与真实数据强制

## 真实数据强制
- [x] ML 预测路径无任何合成数据生成（_synthetic_features 已删除，无数据返回 None/no_data）
- [x] ML 训练脚本无合成兜底；样本不足报错退出而非合成
- [x] ML 特征数据源：TSDB 优先（stock_prices_daily/1min，symbol→stock_id），失败走真实 API，均失败停止该 symbol 预测
- [x] .ml-models 权重已用真实数据重训替换（v2.0.0-real，6 股×200 行真实日线）

## 历史记录管理
- [x] GET /predictions/history 支持 symbol/period/status/from/to 筛选 + 分页
- [x] 前端历史记录管理区可用（日期范围、筛选、分页）

## 分时段统计与断点
- [x] GET /predictions/stats 按 day/week/month 聚合（total/correct/wrong/pending/accuracy）
- [x] 无预测时段 accuracy=null（断点），非 0
- [x] GET /predictions/accuracy 返回全日期序列，缺失日为 null
- [x] 前端趋势图断点渲染（connectNulls=false，null 日不连线、无假值）
- [x] 前端分时段统计展示可用并标注断点日

## 生成状态与调度
- [x] POST /predictions/generate 返回逐 symbol 状态（predicted/no_data），无数据不落库
- [x] ML 注册每日定时预测任务（仅真实数据股票）

## 数据清理与端到端
- [x] 旧的合成预测记录已从 predictions 表清除
- [x] 端到端链路验证：真实数据回填 → 重训 → 生成（含 no_data 分支）→ 历史查询 → 统计 → 断点曲线
- [x] 后端 `go build ./...`、前端 `tsc --noEmit` 通过
- [x] 验证完成后所有测试服务已关闭
