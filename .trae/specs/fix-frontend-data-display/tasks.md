# Tasks

## 阶段一：后端响应包装统一（阻塞所有前端修复）

- [x] Task 1: 统一 market_handler.go 响应包装
  - [x] `GetRealtime` 改用 `success(c, gin.H{"data": data[0]})` 包装
  - [x] `GetRealtimeBatch` 改用 `success(c, gin.H{"data": allData, "count": len(allData)})` 包装
  - [x] `GetKline` 改用 `success(c, gin.H{"symbol": symbol, "interval": interval, "data": cleaned})` 包装
  - [x] `GetDepth` 改用 `success(c, gin.H{...})` 包装
  - [x] `GetMarketStatistics` 字段名改为前端期望的 `limitUpCount/limitDownCount/upCount/downCount/flatCount/totalAmount`
  - [x] `GetFundamentals` 补充 `eps`/`dividendYield` 字段（值可为 0），确保前端不崩溃
  - [x] 验证：编译通过，curl 测试每个接口返回 `{code:0, message:"success", data:{...}}`

- [x] Task 2: 修复 WebSocket 行情推送数据格式
  - [x] `quote_push_service.go` 的 `PushQuote` 方法将 `[]byte` 改为直接传 `MarketData` 对象
  - [x] `hub.go` 的 `Message.Data` 字段类型从 `json.RawMessage` 改为 `interface{}`
  - [x] 验证：WebSocket 客户端收到的 `message.data` 是 JSON 对象而非 base64 字符串

## 阶段二：前端核心组件修复（可并行）

- [x] Task 3: 修复 useStockQuote hook
  - [x] REST 回退：确认从 `data.data` 取行情数组正确
  - [x] WebSocket 分支：改为调用 `mapMarketData(message.data)` 做字段映射
  - [x] 验证：hook 返回的 stocks Map 中包含正确的 symbol/price/changePercent 等字段

- [x] Task 4: 修复 StockChart 组件
  - [x] API 路径从 `/stocks/:symbol/kline` 改为 `/market/kline/:symbol`
  - [x] 参数名从 `period` 改为 `interval`
  - [x] 后端 `price` 字段映射为前端 `close`，`timestamp` ISO 字符串转 number
  - [x] 添加成交量副图（双 grid + 双 yAxis + bar series）
  - [x] 验证：K线图正常渲染蜡烛图 + 成交量副图

## 阶段三：页面级修复（可并行，依赖阶段一和二）

- [x] Task 5: 修复 StockSearch 页面
  - [x] 接入 `useStockQuote` hook，传入股票列表 symbols
  - [x] 表格中最新价、涨跌幅从 `useStockQuote` 返回的 stocks Map 取值
  - [x] 行业、市值从 stocks 表数据取值（已有）
  - [x] 验证：页面展示行业、市值、最新价、涨跌幅均有数据

- [x] Task 6: 修复 TradingPanel 页面
  - [x] 模拟交易 API 路径从 `/sim/*` 改为 `/trading/sim/*`
  - [x] 订单列表从 `data.orders` 取数组
  - [x] 持仓列表从 `data.positions` 取数组
  - [x] 账户信息字段从 snake_case 映射为 camelCase
  - [x] 对 `todayProfitPercent`/`riskLevel` 等可能缺失的字段做安全访问
  - [x] OrderForm 下单字段对齐后端（`action`/`order_type`/`trade_password`/price 转 string）
  - [x] OrderForm 股票搜索参数名 `q`，解包 `data.stocks`
  - [x] 验证：交易面板三个 Tab 均有展示，无崩溃

- [x] Task 7: 修复 StockDetail 页面
  - [x] 基本面字段做安全访问（`?.toFixed(2) ?? '--'`），防止 undefined 崩溃
  - [x] 盘口深度数据适配：将平铺数组映射为 `bids/asks` 对象数组
  - [x] 确保 `useStockQuote` 和 `StockChart` 修复后该页面实时行情和K线图正常
  - [x] 验证：详情页展示实时行情、K线图、盘口深度

- [x] Task 8: 修复 Dashboard 页面
  - [x] 自选股名称从 `item.stock?.name || item.symbol` 取值
  - [x] 统计字段映射确认（后端已改为前端期望的字段名）
  - [x] MarketOverview 的 `DEFAULT_INDICES` 移除后端不支持的 `N225`
  - [x] 验证：仪表盘展示自选股列表（含名称）、实时行情、市场统计、指数概览

## 阶段四：集成验证

- [x] Task 9: 全流程验证
  - [x] 启动后端和前端服务
  - [x] 登录系统（admin/admin123）
  - [x] 验证 Realtime Batch API 返回真实行情数据
  - [x] 验证 Kline API 返回 22 条日K数据（含 OHLCV）
  - [x] 验证 Market Statistics 返回正确字段名
  - [x] 验证 Fundamentals 返回含 eps/dividendYield
  - [x] 验证 Trading Account 返回账户数据
  - [x] 验证 Sim Status 返回模拟交易状态
  - [x] 验证 Stocks Search 返回 200 只股票
  - [x] 额外：修复 TencentCollector FetchHistoricalData 使用 Sina API 获取K线数据

# Task Dependencies
- Task 3, 4, 5, 6, 7, 8 依赖 Task 1（后端响应包装统一）
- Task 3, 4 可并行
- Task 5, 6, 7, 8 依赖 Task 3 和 Task 4（useStockQuote 和 StockChart 修复）
- Task 5, 6, 7, 8 之间可并行
- Task 9 依赖 Task 1~8 全部完成
