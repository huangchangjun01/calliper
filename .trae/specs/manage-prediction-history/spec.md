# 预测历史记录管理与真实数据强制 Spec

## Why
预测系统需要可跟踪的历史记录与分时段统计，但当前预测与模型训练存在"合成数据兜底"（`_synthetic_features`、合成训练数据），一旦真实行情不可用就会产出**假预测**，污染统计与决策。需求要求：预测与训练**只允许真实数据**；真实数据获取不到时**停止预测**，统计曲线出现**断点**而非假值。

## What Changes
- **BREAKING**（行为变更）：移除 ML 预测与训练的一切合成数据兜底——真实数据不可用时预测直接失败（不产出记录）
- ML 特征数据源改为真实渠道：优先读时序库（TSDB），其次真实行情 API；两者都不可用则该股票停止预测
- ML 注册每日定时预测任务（全链路：数据→预测→落库→评估→统计）
- 用真实数据重训三个模型，替换当前基于合成数据训练的权重
- 后端：生成接口返回逐 symbol 状态（predicted / no_data），不落任何假数据
- 后端：新增预测历史记录管理 API（按 symbol/周期/状态/日期范围筛选 + 分页）
- 后端：新增分时段统计 API（按日/周/月聚合预测数量与对错，无预测日为断点）
- 后端：准确率趋势改为"全日期序列"，无数据日 accuracy=null（断点）
- 前端：预测历史记录管理视图（日期范围筛选等）+ 分时段统计展示 + 断点曲线渲染
- 一次性清理：删除此前用合成模型生成的预测记录（当前 predictions 表中的历史测试数据）

## Impact
- Affected specs: quant-trading-system（ML 预测引擎）、decision-loop-closure（预测落库/评估）、fix-tsdb-data-persist（真实数据源已就绪，本 spec 依赖它）
- Affected code:
  - ml-service/app/tasks/prediction_task.py — 移除合成兜底、注册每日预测、真实数据判定
  - ml-service/app/utils/data_loader.py — 支持 TSDB 真实数据读取（修正表名/连接）
  - ml-service/train_models.py — 仅真实数据训练
  - ml-service/app/main.py — 注册每日定时预测任务
  - backend/internal/services/prediction_service.go — 生成结果逐 symbol 状态
  - backend/internal/services/evaluation_service.go — 历史记录查询、分时段统计、含断点趋势
  - backend/internal/handlers/prediction_handler.go — 新增 history/stats 路由处理
  - backend/cmd/gateway/main.go — 注册新路由
  - frontend 预测页与相关组件 — 历史管理、统计、断点曲线

---

## ADDED Requirements

### Requirement: 预测仅使用真实数据
系统 SHALL 在真实行情数据不可用时停止预测，绝不使用合成/伪造数据产出预测。

#### Scenario: 数据源全部不可用
- **WHEN** ML 对某 symbol 无法从 TSDB 或真实行情 API 获取足量特征数据
- **THEN** 该 symbol 本轮不产出任何预测（无落库记录），并返回明确的 no_data 状态

#### Scenario: 合成兜底移除
- **WHEN** 检查 ML 预测与训练代码路径
- **THEN** 不存在任何合成/随机数据生成预测或训练权重的路径

### Requirement: 训练仅使用真实数据
系统 SHALL 仅用真实行情数据训练模型；数据不足时训练失败并报错，而非用合成数据代替。

#### Scenario: 重训模型
- **WHEN** 执行模型训练
- **THEN** 训练数据来自时序库或真实 API；样本不足时训练报错退出，权重文件不被合成数据污染

### Requirement: 预测历史记录管理
系统 SHALL 提供预测历史的查询管理能力：按 symbol、周期、状态（待验证/对/错）、日期范围筛选，分页返回。

#### Scenario: 按日期范围查询
- **WHEN** 用户指定 from/to 日期查询预测历史
- **THEN** 返回该范围内的预测记录列表与总数，支持分页

### Requirement: 分时段预测统计
系统 SHALL 按日/周/月聚合预测情况：总数、对/错/待验证数量、准确率；无预测的时段为断点（null）而非 0。

#### Scenario: 统计含断点
- **WHEN** 某日因真实数据缺失未产生预测
- **THEN** 该日统计项的准确率为 null（断点），当日预测数为 0

### Requirement: 断点曲线展示
系统 SHALL 在准确率趋势图中如实呈现断点：无数据日不连线、不补 0。

#### Scenario: 趋势渲染
- **WHEN** 统计区间内存在无预测数据的日期
- **THEN** 曲线在该日期处断开（connectNulls=false），无假值填充

---

## MODIFIED Requirements

### Requirement: ML 特征数据源（原 _load_from_api 优先 Yahoo）
特征构建优先读时序库（stock_prices_daily/stock_prices_1min，按 symbol 解析 stock_id），失败再走真实行情 API（Yahoo/新浪）；两者都失败返回"无数据"，不再生成合成特征。

### Requirement: 预测生成接口（POST /predictions/generate）
生成结果增加逐 symbol 状态：predicted（已落库条数）或 no_data（真实数据不可用，未落库）。接口整体不因个别 symbol 无数据而失败。

## REMOVED Requirements

### Requirement: 合成特征兜底（_synthetic_features）
**Reason**: 违反"预测必须基于真实数据"的核心要求，会在数据缺失时产出假预测污染统计。
**Migration**: 删除该方法及全部调用点；真实数据不可用路径改为返回 None/错误。

### Requirement: 合成训练数据（train_models.py 的 synth_ohlcv 兜底）
**Reason**: 合成随机游走训练出的权重不具市场意义。
**Migration**: 训练脚本改为从 TSDB/真实 API 读取；不足即失败。