# Checklist — 模拟交易自动化

## 数据模型与账户
- [x] simulated_trades 包含 profit、profit_rate 列；sim_accounts 包含 initial_capital 列（AutoMigrate 生效）
- [x] 初始化账户 initial_capital=1000000；旧账户以 TotalAssets 兜底回填

## 决策引擎
- [x] 决策数据源优先本地 predictions 表（valid_until > now() AND success IS NULL，每票最新、short>medium>long）
- [x] 本地无有效预测时降级 ML 服务；两者皆无则 0 决策
- [x] 上涨 up 且置信度≥60% 买入；下跌 down 且置信度≥60% 卖出；置信度<55% 不参与选股

## 自动调度
- [x] 后端启动即自动启动调度器并立即执行首轮决策（日志可见）
- [x] 交易时段每 15 分钟自动决策；收盘后自动盘后结算
- [x] 无"账户停止即退出"的自动停止逻辑

## 收益
- [x] 卖出成交时计算并落库 profit / profit_rate；买入为 0
- [x] 状态接口返回 account 含 total_profit、total_profit_percent、initial_capital
- [x] 前端账户卡片与交易记录表展示收益（今日/累计盈亏、盈亏率，红涨绿跌）

## 接口与路由
- [x] GET /trading/sim/status 返回完整 SimStatus（running/account/decisions/records/risk_control）
- [x] POST /trading/sim/trigger 注册并可手动触发一轮决策

## 验收
- [x] 后端 go build 与启动日志符合预期（自动调度+首轮决策）
- [x] 前端 tsc --noEmit 通过；交易面板模拟交易页收益展示完整
- [x] 验证后关闭服务，交由用户自行启动验收