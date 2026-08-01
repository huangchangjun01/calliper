# Checklist

## 后端响应包装统一
- [x] `GetRealtime` 返回 `{code:0, message:"success", data:{data:{symbol,price,...}}}`
- [x] `GetRealtimeBatch` 返回 `{code:0, message:"success", data:{data:[...], count:N}}`
- [x] `GetKline` 返回 `{code:0, message:"success", data:{symbol, interval, data:[...]}}`
- [x] `GetDepth` 返回 `{code:0, message:"success", data:{symbol, bid_prices, ...}}`（成功路径已包装）
- [x] `GetMarketStatistics` 返回前端期望的字段名（`limitUpCount`/`upCount`/`downCount`/`flatCount`/`totalAmount`）
- [x] `GetFundamentals` 返回包含 `eps` 和 `dividendYield` 字段
- [x] 后端编译通过，无语法错误

## WebSocket 行情推送
- [x] `PushQuote` 的 Data 字段为 `MarketData` 对象而非 `[]byte`
- [x] `hub.go` 的 `Message.Data` 类型改为 `interface{}` 支持直接传对象
- [x] 前端 WS 收到的 `message.data` 是 JSON 对象（含 `symbol`/`price`/`change_percent` 等字段）

## useStockQuote hook
- [x] REST 回退从 `data.data` 取行情数组（data 为 `{data:[...], count}` 对象）
- [x] WebSocket 分支调用 `mapMarketData(message.data)` 做字段映射
- [x] 返回的 stocks Map 中 `changePercent`/`preClose` 等 camelCase 字段有值

## StockChart 组件
- [x] API 路径为 `/market/kline/:symbol`（非 `/stocks/:symbol/kline`）
- [x] 参数名为 `interval`（非 `period`）
- [x] 后端 `price` 字段映射为前端 `close`，`timestamp` ISO 字符串转 number
- [x] 图表渲染蜡烛图主图 + 成交量副图（双 grid + 双 yAxis + bar series）

## 股票检索页面
- [x] 行业字段正确读取（来自 stocks 表 `industry`）
- [x] 市值字段正确读取（来自 stocks 表 `market_cap`）
- [x] 最新价有值（来自 useStockQuote 实时行情）
- [x] 涨跌幅有值（来自 useStockQuote 实时行情）

## 交易面板
- [x] 模拟交易 API 路径为 `/trading/sim/*`（非 `/sim/*`）
- [x] 订单列表正确取 `data.orders` 数组
- [x] 持仓列表正确取 `data.positions` 数组
- [x] 账户信息字段 snake_case→camelCase 映射正确
- [x] `todayProfitPercent`/`riskLevel` 等字段安全访问不崩溃
- [x] OrderForm 下单字段对齐后端（`action`/`order_type`/`trade_password`）
- [x] OrderForm 股票搜索参数名 `q`，解包 `data.stocks`

## 个股详情页
- [x] 基本面字段安全访问（`?.toFixed(2) ?? '--'`），不因 `eps`/`dividendYield` 缺失崩溃
- [x] 盘口深度数据正确映射为 `bids/asks` 对象数组
- [x] 实时行情正常显示（依赖 useStockQuote 修复）
- [x] K线图正常显示（依赖 StockChart 修复）

## 仪表盘
- [x] 自选股名称从 `item.stock?.name || item.symbol` 取值
- [x] 统计字段映射正确（后端已改为前端期望的字段名）
- [x] MarketOverview 移除不支持的 `N225` 指数
- [x] 自选股实时行情列有数据（依赖 useStockQuote 修复）

## 额外修复
- [x] TencentCollector.FetchHistoricalData 使用 Sina API 获取K线数据（之前返回 error）
- [x] K线 API 验证返回 22 条日K数据（含 OHLCV）

## 集成验证
- [x] 后端服务正常启动
- [x] 前端服务正常启动（http://localhost:3000）
- [x] admin/admin123 登录成功
- [x] Realtime Batch API 返回真实行情数据（3 只股票含价格和涨跌幅）
- [x] Kline API 返回 22 条日K数据（含 OHLCV）
- [x] Market Statistics 返回正确字段名
- [x] Fundamentals 返回含 eps/dividendYield
- [x] Trading Account 返回账户数据
- [x] Sim Status 返回模拟交易状态
- [x] Stocks Search 返回 200 只股票
