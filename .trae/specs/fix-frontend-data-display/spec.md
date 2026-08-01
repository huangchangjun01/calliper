# 前端数据展示修复 Spec

## Why
系统前端多个页面存在数据展示空白或页面崩溃问题：股票检索页面行业/市值/最新价/涨跌幅为空、交易面板无展示、个股详情页无展示、仪表盘无数据、K线图无法显示。根因是后端多个接口未统一响应包装、前后端字段命名不一致（snake_case vs camelCase）、API 路径不匹配、以及前端取值逻辑错误。

## What Changes
- 统一后端 `market_handler.go` 所有接口的响应包装为 `{code, message, data}` 格式
- 修复 `useStockQuote` hook 的 REST 回退取值逻辑和 WebSocket 字段映射
- 修复 `StockChart` 组件的 API 路径和参数名
- 修复 `StockSearch` 页面字段映射，补充实时行情数据
- 修复 `TradingPanel` 页面的 API 路径、响应结构适配和字段映射
- 修复 `StockDetail` 页面的基本面字段安全访问、深度数据适配
- 修复 `Dashboard` 页面的自选股名称取值和统计字段映射
- 修复 `StockDetail` 基本面 `eps`/`dividendYield` 缺失导致的渲染崩溃

## Impact
- Affected specs: quant-trading-system（前端页面展示部分）
- Affected code:
  - 后端: `market_handler.go`, `quote_push_service.go`
  - 前端: `useStockQuote.ts`, `StockChart/index.tsx`, `StockSearch.tsx`, `TradingPanel.tsx`, `StockDetail.tsx`, `Dashboard.tsx`, `AccountOverview/index.tsx`, `OrderList/index.tsx`, `PositionList/index.tsx`, `OrderForm/index.tsx`

---

## MODIFIED Requirements

### Requirement: 统一后端响应包装
`market_handler.go` 中所有 HTTP 接口 SHALL 使用统一的 `success()` 助手函数包装响应，返回 `{code: 0, message: "success", data: <payload>}` 格式。

#### 受影响接口
- `GetRealtime` — `GET /market/realtime/:symbol`
- `GetRealtimeBatch` — `POST /market/realtime/batch`
- `GetKline` — `GET /market/kline/:symbol`
- `GetDepth` — `GET /market/depth/:symbol`
- `GetIndices` — `GET /market/indices`（已包装，保持不变）
- `GetMarketStatistics` — `GET /market/statistics`
- `GetFundamentals` — `GET /market/fundamentals/:symbol`

#### Scenario: 前端请求批量行情
- **WHEN** 前端 POST `/market/realtime/batch` 请求多只股票行情
- **THEN** 后端返回 `{code: 0, message: "success", data: {data: [...], count: N}}`，前端 `json.data` 取到 `{data: [...], count: N}` 对象

#### Scenario: 前端请求K线数据
- **WHEN** 前端 GET `/market/kline/:symbol?interval=1d`
- **THEN** 后端返回 `{code: 0, message: "success", data: {symbol, interval, data: [...]}}`，前端 `json.data.data` 取到K线数组

---

### Requirement: 实时行情数据获取（useStockQuote）
`useStockQuote` hook SHALL 通过 REST API 回退机制正确获取实时行情数据，并正确映射后端 snake_case 字段为前端 camelCase 字段。

#### Scenario: REST 回退正确取值
- **WHEN** useStockQuote 调用 `POST /market/realtime/batch`
- **THEN** 前端从 `json.data`（即 `{data: [...], count}`）中取 `data.data` 获取行情数组，遍历映射每个 item

#### Scenario: 字段映射正确
- **WHEN** 后端返回 `{symbol, name, price, change_percent, pre_close, ...}`
- **THEN** 前端通过 `mapMarketData` 映射为 `{symbol, name, price, changePercent, preClose, ...}`

---

### Requirement: K线图组件（StockChart）
`StockChart` 组件 SHALL 调用正确的 API 端点和参数获取K线数据，并正确解析响应。

#### Scenario: 获取日K线数据
- **WHEN** 用户在仪表盘或个股详情页查看K线图
- **THEN** 组件请求 `GET /market/kline/:symbol?interval=1d`，从响应 `data.data` 取K线数组，渲染蜡烛图和成交量副图

#### Scenario: K线数据包含交易量
- **WHEN** K线数据返回
- **THEN** 图表展示蜡烛图主图 + 成交量副图，成交量数据来自 K线 item 的 `volume` 字段

---

### Requirement: 股票检索页面数据展示
`StockSearch` 页面 SHALL 正确展示股票的行业、市值，并通过实时行情 API 补充最新价和涨跌幅。

#### Scenario: 展示股票基本信息
- **WHEN** 用户访问股票检索页面
- **THEN** 表格展示代码、名称、交易所、行业（来自 stocks 表）、市值（来自 stocks 表）

#### Scenario: 展示实时行情
- **WHEN** 股票列表加载完成
- **THEN** 页面通过 `useStockQuote` hook 获取列表中股票的实时行情，展示最新价、涨跌幅、涨跌额、成交量、成交额

---

### Requirement: 交易面板数据展示
`TradingPanel` 页面 SHALL 正确展示账户信息、订单列表、持仓列表和模拟交易状态。

#### Scenario: 展示账户概览
- **WHEN** 用户访问交易面板的真实交易 Tab
- **THEN** 页面展示总资产、可用资金、持仓市值、今日盈亏、总盈亏，字段从后端 snake_case 映射为前端 camelCase

#### Scenario: 展示订单和持仓列表
- **WHEN** 用户访问交易面板
- **THEN** 订单列表从 `data.orders` 取数组，持仓列表从 `data.positions` 取数组，正确渲染

#### Scenario: 模拟交易状态
- **WHEN** 用户切换到模拟交易 Tab
- **THEN** 页面请求 `/trading/sim/status`（正确路径），展示运行状态、今日交易数、今日盈亏

---

### Requirement: 个股详情页数据展示
`StockDetail` 页面 SHALL 正确展示实时行情、K线图、盘口深度和基本面数据，且不因字段缺失而崩溃。

#### Scenario: 安全渲染基本面
- **WHEN** 后端返回的基本面数据缺少某些字段（如 eps、dividendYield）
- **THEN** 页面对每个字段做安全访问（`value?.toFixed(2) ?? '--'`），不崩溃，缺失字段显示 '--'

#### Scenario: 展示盘口深度
- **WHEN** 后端返回 `{bid_prices: [], bid_volumes: [], ask_prices: [], ask_volumes: []}`
- **THEN** 前端将其映射为 `bids: [{price, volume}], asks: [{price, volume}]` 结构并展示

---

### Requirement: 仪表盘数据展示
`Dashboard` 页面 SHALL 正确展示自选股列表（含名称）、实时行情和 market 统计数据。

#### Scenario: 自选股名称展示
- **WHEN** 后端返回自选股列表，每项包含 `stock.name`
- **THEN** 前端从 `item.stock?.name` 或平铺的 `item.name` 取到名称并展示

#### Scenario: 统计字段映射
- **WHEN** 后端返回 `{limitUp, limitDown, advancing, declining, unchanged, totalAmount}`
- **THEN** 前端映射为 `{limitUpCount: limitUp, limitDownCount: limitDown, upCount: advancing, downCount: declining, flatCount: unchanged, totalAmount}` 并展示

---

### Requirement: WebSocket 行情推送数据格式
`QuotePushService` 的 `PushQuote` 方法 SHALL 将 `MarketData` 对象（而非 `[]byte`）作为 WebSocket 消息的 Data 字段发送。

#### Scenario: 前端正确解析 WS 消息
- **WHEN** 后端通过 WebSocket 推送行情数据
- **THEN** 前端 `message.data` 收到的是 JSON 对象（含 symbol, price, change_percent 等字段），经 `mapMarketData` 映射后可正确使用
