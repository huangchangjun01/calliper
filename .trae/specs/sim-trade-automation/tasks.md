# Tasks — 模拟交易自动化

> 依据《模拟交易自动化 Spec》（.trae/specs/sim-trade-automation/spec.md）编制。

## Task 1: 数据模型扩展（backend）
- [x] 为 `SimulatedTrade` 新增 `Profit`（已实现盈亏，decimal(20,2) 默认 0）与 `ProfitRate`（盈亏率，decimal(10,4) 默认 0）字段
- [x] 为 `SimAccount` 新增 `InitialCapital`（初始资金，decimal(20,2) 默认 1000000）字段
- [x] 依赖 AutoMigrate 自动建表/加列，无手工 SQL 脚本
- [x] Validation：启动后数据库 `simulated_trades` 含 profit/profit_rate 列，`sim_accounts` 含 initial_capital 列

## Task 2: 账户初始资金兜底（backend）
- [x] 初始化模拟账户时写入 `InitialCapital = TotalAssets = 初始资金`
- [x] 查询既有账户时，`InitialCapital <= 0` 的旧账户以当前 TotalAssets 兜底回填
- [x] Validation：新建账户 initial_capital=1000000；旧账户查询不报错且 initial_capital 有值

## Task 3: 决策引擎按预测分析结果执行（backend）
- [x] `getPredictions` 优先查询本地 `predictions` 表：`valid_until > now() AND success IS NULL`，每只股票取最新一条（row_number 按 predicted_at 降序、同批次优先级 short>medium>long）
- [x] 本地无有效预测时降级调用 ML 预测服务（原逻辑保留）
- [x] 依据实时价计算每只股票的预期收益率 `(目标价/现价−1)*100`，补全行业用于仓位约束
- [x] 无有效预测时本轮返回 0 条决策（"无信号不交易"）
- [x] Validation：有 up/down 有效预测时生成对应买卖决策；仅有低置信度/无预测时不产生决策

## Task 4: 调度器自动常驻（backend）
- [x] 移除"账户停止即退出"的自动停止逻辑（checkTicker 分支）
- [x] 启动即立即执行一轮 `runDecisionCycle`
- [x] 交易时段每 15 分钟自动决策一次；收盘后自动执行 `SettleDaily` 盘后结算
- [x] Validation：后端启动日志出现"模拟交易调度器已启动"与"启动后立即执行首轮模拟交易决策"

## Task 5: 完整状态接口与手动触发（backend）
- [x] `GET /trading/sim/status` 返回完整 SimStatus（running/account 含 total_profit\total_profit_percent\initial_capital/decisions/records/risk_control）
- [x] 新增 `POST /trading/sim/trigger` 手动触发一轮决策周期
- [x] 路由注册于 `cmd/gateway/main.go`
- [x] Validation：状态接口返回字段完整；trigger 接口可手动驱动一轮决策

## Task 6: 卖出盈亏落库（backend）
- [x] 卖出成交时，按卖出前持仓均价计算 `Profit=(卖出价−均价)×数量` 与 `ProfitRate=(卖出价/均价−1)*100` 并写入交易记录
- [x] 买入记录 Profit/ProfitRate 为 0
- [x] Validation：卖出后的交易记录含正确盈亏金额与盈亏率

## Task 7: 前端收益展示（frontend）
- [x] `TradingPanel.tsx` 将后端 snake_case 状态映射为前端 SimStatus（含 totalProfit/totalProfitPercent），15s 轮询刷新
- [x] `SimTradePanel` 账户卡片新增"累计收益"（金额+收益率，红涨绿跌）
- [x] 交易记录表增加"盈亏/盈亏率"列（红涨绿跌）
- [x] Validation：`tsc --noEmit` 通过；浏览器可见收益卡片与盈亏列

## Task 8: 全链路验证
- [x] 后端 `go build ./...` 通过
- [x] 启动后端验证自动调度与状态接口；启动前端验证交易面板模拟交易页展示完整
- [x] 验证完成后关闭服务，交由用户自行启动验收

# Task Dependencies
- Task 1 → Task 2、Task 3、Task 6（字段先行）
- Task 3 依赖 Task 1（决策记录关联预测）
- Task 5 依赖 Task 1、Task 2（状态含账户收益字段）
- Task 7 依赖 Task 5（前端数据来自完整状态接口）
- Task 8 依赖全部任务

# 可并行执行的任务组
- 组 1：Task 1、Task 2（模型与账户初始化，可并行）
- 组 2：Task 3、Task 4、Task 6（决策引擎/调度器/盈亏落库，依赖 Task 1，可并行）
- 组 3：Task 7（前端，依赖 Task 5）