# Tasks — 预测历史记录管理与真实数据强制

## Task 1: ML 移除合成兜底（预测路径）
- [x] 删除 `prediction_task.py` 中 `_synthetic_features` 及全部调用点；`_build_features` 真实数据不可用时返回 None（该 symbol 停止预测）
- [x] `predict_single_stock` 对特征缺失的周期跳过并在结果中标注 no_data（no_data_periods）
- [x] 验证：代码中无合成数据生成路径（grep 确认）；ML 服务可正常启动（app import OK）

## Task 2: ML 真实数据源接入（TSDB 优先）
- [x] `data_loader.py` 支持双连接：主库（DATABASE_URL，symbol→stock_id）+ 时序库（TSDB_URL）；读取表修正为 `stock_prices_daily`/`stock_prices_1min`（按 time+stock_id）
- [x] `load_stock_data` 顺序：TSDB → 真实行情 API（Yahoo/新浪）→ 失败返回空（不合成）
- [x] ML `.env` 增加 DATABASE_URL/TSDB_URL 配置（本地：calliper_trading/calliper_tsdb @ localhost:5432）
- [x] 验证：TSDB 冒烟测试 000001 返回 200 行真实日线（open/high/low/close/volume 均 float）

## Task 3: 训练仅用真实数据并重训
- [x] `train_models.py` 删除 `synth_ohlcv` 兜底；改为从 TSDB（经 DataLoader）读取真实日线构造训练集
- [x] 样本不足（< 120 行）跳过该 symbol；全部不足则 sys.exit(1)，不产出权重
- [x] 用真实数据重训 short/medium/long，替换 `.ml-models` 权重与 versions.json（在 Task 9 端到端阶段执行）
- [x] 验证：新权重可被服务加载；训练日志显示真实数据条数来源（6 股×200 行，v2.0.0-real 加载确认）

## Task 4: 每日预测调度与后端生成状态
- [x] ML `main.py` 启动时注册每日预测任务（15:30，load_dotenv + schedule_daily_prediction），仅对有真实数据的股票预测
- [x] 后端 `GenerateAndPersist` 返回逐 symbol 状态：predicted（条数）/ no_data；不因个别失败整体报错
- [x] 验证：调用 POST /predictions/generate，无数据 symbol 返回 no_data 且不落库（Task 9 运行时验证：000001/600036=predicted 各 3 条，600519=no_data 0 条）

## Task 5: 预测历史记录管理 API
- [x] 后端新增 `GET /predictions/history`：参数 symbol、period、status（pending/correct/wrong）、from、to、offset/limit；返回分页列表与 total
- [x] 复用/扩展 evaluation_service 查询（predictions 表 join stocks）
- [x] 验证：带筛选条件请求返回正确记录集合与总数（Task 9 运行时验证：无筛选 total=6 分页正常；symbol=600036&period=short_term total=1；status=correct/wrong 筛选正确）

## Task 6: 分时段统计 API（含断点）
- [x] 后端新增 `GET /predictions/stats`：参数 bucket（day/week/month）、from、to；按桶聚合 total/correct/wrong/pending、accuracy；无预测桶 accuracy=null（断点）
- [x] 验证：混合有/无预测日期的区间，无预测日返回 null 而非 0（Task 9 运行时验证：day 桶 08-20~08-27 空 accuracy=null、08-26 total=6；week 桶 W33/W34 null、W35 有值）

## Task 7: 准确率趋势含断点
- [x] 修改 `GetAccuracyTrendData`：返回区间内**全部日期**序列，无数据日 accuracy=null（*float64）
- [x] 验证：区间内缺失日的趋势项为 null（Task 9 运行时验证：days=7 全 null；模拟评估后 08-26=50/100/0、其余日 null；period=medium 无记录全 null）

## Task 8: 前端历史管理、统计与断点曲线
- [x] 预测页新增"历史记录"管理区：日期范围选择、symbol/周期/状态筛选、分页列表（含"生成预测"入口，逐 symbol 展示 predicted/no_data）
- [x] 新增"分时段统计"展示：日/周/月切换的统计表（accuracy=null 显示"—（断点）"，空桶淡显）
- [x] 准确率趋势图改为断点渲染（ECharts connectNulls=false；null 日不连线，tooltip 显示"无数据"）
- [x] `tsc --noEmit` 通过（exit 0）

## Task 9: 数据清理与端到端验证
- [x] 清理 predictions 表中此前基于合成模型生成的历史预测记录（一次性删除 6 条）
- [x] 端到端验证：回填真实数据 → 真实数据重训 → 生成预测（有数据 symbol 落库、无数据 no_data）→ 历史查询 → 分时段统计 → 断点曲线
- [x] 验证后关闭所有测试启动的服务（ML 8000 / 后端 8080 均已关闭；模拟评估数据已还原为 NULL、测试用户已删除）

# Task Dependencies
- Task 3 依赖 Task 2（真实数据读取）
- Task 4 依赖 Task 1、Task 2
- Task 5、6、7 相互独立（可并行，均只依赖现有表结构）
- Task 8 依赖 Task 5、6、7（接口就绪）
- Task 9 依赖 Task 1–8（全链路收尾）

# 可并行执行的任务组
- 组 A：Task 5、Task 6、Task 7（后端查询/统计侧，可并行）
- 组 B：Task 1+2 串行后，Task 3 与 Task 4 可并行