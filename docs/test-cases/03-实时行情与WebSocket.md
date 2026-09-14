# 03-实时行情与 WebSocket 测试用例

> 本文档为智能量化交易系统「实时行情（MKT）」与「WebSocket 实时推送（WS）」模块的完整测试用例，逐条落实 `docs/.trae/specs/complete-test-case-suite/spec.md` 中 MKT-001~025、WS-001~011 基准用例，并补充必要的边界、异常与前端交互用例（序号从 MKT-026 / WS-012 继续）。
> 覆盖三端：后端 Go API、前端 React 页面、WebSocket 实时链路。

## 文档信息

| 项 | 内容 |
|---|---|
| 覆盖模块 | 实时行情（MKT）、WebSocket 实时推送（WS） |
| 用例总数 | 58 条（MKT 38 条、WS 20 条） |
| 编号范围 | MKT-001 ~ MKT-038；WS-001 ~ WS-020 |
| 对应文档 | `backend/internal/handlers/market_handler.go`、`backend/internal/handlers/ws_handler.go`、`backend/internal/services/market_data_service.go`、`backend/internal/services/quote_push_service.go`、`backend/internal/services/history_backfill.go`、`backend/internal/websocket/`、`frontend/src/services/websocket.ts`、`frontend/src/hooks/useWebSocket.ts`、`frontend/src/hooks/useStockQuote.ts`、`frontend/src/pages/Market.tsx` |

## 覆盖端点

| 端点 | 方法 | 鉴权 | 说明 |
|---|---|---|---|
| `/api/v1/market/realtime/:symbol` | GET | Bearer token | 单只实时行情 |
| `/api/v1/market/realtime/batch` | POST | Bearer token | 批量实时行情（上限 50） |
| `/api/v1/market/kline/:symbol` | GET | Bearer token | K 线（interval/from/to） |
| `/api/v1/market/depth/:symbol` | GET | Bearer token | 盘口深度 |
| `/api/v1/market/backfill` | POST | Bearer token | 触发历史数据回填 |
| `/api/v1/market/backfill/progress` | GET | Bearer token | 回填进度 |
| `/api/v1/market/indices` | GET | Bearer token | 市场指数 |
| `/api/v1/market/statistics` | GET | Bearer token | 市场统计 |
| `/api/v1/market/fundamentals/:symbol` | GET | Bearer token | 基本面 |
| `/ws` | GET(Upgrade) | `?token=<JWT>` 查询参数 | WebSocket 实时推送 |

> 说明：
> - 所有 `/api/v1/market/*` 端点在路由中注册于 `protected` 组，必须携带 `Authorization: Bearer <token>`，未认证返回 401。
> - `/ws` 不走 HTTP 中间件鉴权，由 `WsHandler` 从 URL 查询参数 `token` 校验 JWT（缺失 → 401 `missing token query parameter`；无效/过期 → 401 `invalid or expired token`）。
> - 市场推导规则（`detectMarketCode`）：`.SH/.SZ/.BJ`→CN、`.HK`→HK、`.T`→JP、`.L`→UK、`.PA/.DE/.AS`→EU、6 位纯数字→CN、5 位纯数字→HK、其余（纯字母或混合字符）→US。
> - 成功响应统一包装：`{"code":0,"message":"success","data":{...}}`；错误响应统一为 `{"error":"<message>"}`。

## 测试数据约定

- 测试账号：`admin/admin123`（admin）、`testuser/pass1234`（user）。
- 测试股票：A 股 `600519.SH`、`000001.SZ`、`000001`（6 位数字）；港股 `00700.HK`、`00700`（5 位数字）；美股 `AAPL`、`TSLA`；日股 `7203.T`；欧股 `SIE.DE`。
- 无效 symbol：`""`、`NONEXIST123`、`123`（3 位数字）、`A1B2`、`600519SH`（无后缀）、超长字符串。
- WebSocket 握手地址：`ws://localhost:8080/ws?token=<token>`（前端经 Vite 代理同源 `/ws`，token 取 `localStorage.auth_token`）。
- WS 频道格式：`stock:{symbol}`（如 `stock:600519.SH`）；服务端推送消息 `type` 为 `quote`；客户端命令 `type` 为 `subscribe` / `unsubscribe` / `ping`。

---

# 一、实时行情（MKT）

## 单只实时行情

### MKT-001 单只行情-A股
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 功能 |
| 优先级 | P0 |
| 前置条件 | 已登录（admin/admin123）；后端已启动；网络可访问行情数据源 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/realtime/600519.SH`，请求头携带 `Authorization: Bearer <token>` |
| 预期结果 | HTTP 200，响应 `{"code":0,"message":"success","data":{...}}`；data 含 `symbol="600519.SH"`、`name`、`price`（>0）、`open`、`high`、`low`、`pre_close`、`change`、`change_percent`、`volume`（>=0）、`amount`、`timestamp`、`market_code` 等字段；数值合理：`change = price - pre_close`、`change_percent ≈ change/pre_close*100` |

### MKT-002 单只行情-美股
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 功能 |
| 优先级 | P1 |
| 前置条件 | 已登录；网络可访问行情数据源 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/realtime/AAPL`，携带 Bearer token |
| 预期结果 | HTTP 200；市场推导为 US（纯字母→US）并成功采集；data 含 `symbol="AAPL"`、`price`、`change`、`volume` 等字段 |

### MKT-003 纯数字 A股
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 功能 |
| 优先级 | P1 |
| 前置条件 | 已登录；网络可访问行情数据源 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/realtime/000001`（6 位数字），携带 Bearer token |
| 预期结果 | HTTP 200；市场推导为 CN（6 位纯数字→CN）；data 含 `symbol="000001"` 及行情字段 |

### MKT-004 纯数字 港股
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 功能 |
| 优先级 | P1 |
| 前置条件 | 已登录；网络可访问行情数据源 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/realtime/00700`（5 位数字），携带 Bearer token |
| 预期结果 | HTTP 200；市场推导为 HK（5 位纯数字→HK）；data 含 `symbol="00700"` 及行情字段 |

### MKT-005 symbol 为空
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 异常 |
| 优先级 | P1 |
| 前置条件 | 已登录 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/realtime/`（symbol 为空），携带 Bearer token |
| 预期结果 | HTTP 400，响应 `{"error":"symbol is required"}` |

### MKT-006 symbol 不存在
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 异常 |
| 优先级 | P1 |
| 前置条件 | 已登录 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/realtime/NONEXIST123`，携带 Bearer token |
| 预期结果 | HTTP 404，响应 `{"error":"symbol not found"}`；不返回 500 |

## 批量实时行情

### MKT-007 批量行情
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 功能 |
| 优先级 | P0 |
| 前置条件 | 已登录；网络可访问行情数据源 |
| 测试步骤 | 1. 调用 `POST /api/v1/market/realtime/batch`，携带 Bearer token，请求体 `{"symbols":["600519.SH","AAPL"]}` |
| 预期结果 | HTTP 200，响应 `data` 为数组、`count=2`；每个元素含 `symbol`、`price`、`change` 等字段；按市场分组采集（CN/US）均无异常 |

### MKT-008 批量缺 symbols
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 异常 |
| 优先级 | P1 |
| 前置条件 | 已登录 |
| 测试步骤 | 1. 调用 `POST /api/v1/market/realtime/batch`，携带 Bearer token，请求体 `{"symbols":[]}`；2. 再发缺 symbols 字段的请求体 `{}` |
| 预期结果 | 两种情况均返回 HTTP 400，响应 `{"error":"invalid request"}`（`binding:"required"` 校验） |

### MKT-009 批量超 50
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 边界 |
| 优先级 | P1 |
| 前置条件 | 已登录 |
| 测试步骤 | 1. 构造 60 个 symbol 的数组（如 `["600519.SH",...,"AAPL"]`），调用 `POST /api/v1/market/realtime/batch`，携带 Bearer token |
| 预期结果 | HTTP 200；symbols 被截断到前 50 个，`count<=50`；不报错、不 500 |

## K 线

### MKT-010 K线默认
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 功能 |
| 优先级 | P1 |
| 前置条件 | 已登录；网络可访问行情数据源 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/kline/600519.SH`（默认 `interval=1d`，from 默认近 1 个月），携带 Bearer token |
| 预期结果 | HTTP 200；响应含 `symbol="600519.SH"`、`interval="1d"`、`from`、`to`、`data`（K 线数组）、`count`；data 每项含 `open/high/low/close/price/volume/amount/timestamp` 等字段，`count = len(data)` |

### MKT-011 K线非法 interval
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 异常 |
| 优先级 | P1 |
| 前置条件 | 已登录 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/kline/600519.SH?interval=2h`，携带 Bearer token |
| 预期结果 | HTTP 400，响应 `{"error":"invalid interval, supported: 1m,5m,15m,30m,60m,1d"}` |

### MKT-012 K线日期格式错
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 异常 |
| 优先级 | P1 |
| 前置条件 | 已登录 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/kline/600519.SH?from=2024/01/01`，携带 Bearer token |
| 预期结果 | HTTP 400，响应 `{"error":"invalid from date format, use YYYY-MM-DD"}` |

### MKT-013 K线 to<from
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 异常 |
| 优先级 | P2 |
| 前置条件 | 已登录 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/kline/600519.SH?from=2024-01-01&to=2023-01-01`，携带 Bearer token |
| 预期结果 | HTTP 400 或返回 200 空数据（`count=0`/`data=[]`）；不崩溃、不 500 |

## 盘口深度

### MKT-014 盘口深度
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 功能 |
| 优先级 | P1 |
| 前置条件 | 已登录；网络可访问行情数据源 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/depth/600519.SH`，携带 Bearer token |
| 预期结果 | HTTP 200；data 含 `symbol="600519.SH"`、`bid_prices`、`bid_volumes`、`ask_prices`、`ask_volumes`、`timestamp` 字段；数组长度一致，`ask_prices` 单调不减、`bid_prices` 单调不增 |

### MKT-015 盘口未命中
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 异常 |
| 优先级 | P1 |
| 前置条件 | 已登录 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/depth/NONEXIST123`，携带 Bearer token |
| 预期结果 | HTTP 404，响应 `{"error":"symbol not found"}`；不 500 |

## 历史数据回填

### MKT-016 触发回填
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 功能 |
| 优先级 | P1 |
| 前置条件 | 已登录；TSDB 可用；网络可访问行情数据源 |
| 测试步骤 | 1. 调用 `POST /api/v1/market/backfill`，携带 Bearer token，请求体 `{"symbols":["600519.SH"],"data_type":"daily","years":3}` |
| 预期结果 | HTTP 202，响应 `{"message":"backfill task started","data_type":"daily","symbols":1}`；任务后台异步执行，随后 `GET /api/v1/market/backfill/progress` 可见该 symbol 进度 |

### MKT-017 回填缺 symbols
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 异常 |
| 优先级 | P1 |
| 前置条件 | 已登录 |
| 测试步骤 | 1. 调用 `POST /api/v1/market/backfill`，携带 Bearer token，请求体 `{"symbols":[]}` |
| 预期结果 | HTTP 400，响应 `{"error":"invalid request"}` |

### MKT-018 回填进度
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 功能 |
| 优先级 | P2 |
| 前置条件 | 已登录；至少触发过一次回填 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/backfill/progress`，携带 Bearer token |
| 预期结果 | HTTP 200，响应 `{"progress":{...}}`；每个 symbol 的进度对象含 `symbol`、`status`（pending/running/completed/failed）、`progress`（0-100）、`records`、`error`（可选）、`updated_at` 字段 |

## 指数 / 统计 / 基本面

### MKT-019 指数默认
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 功能 |
| 优先级 | P1 |
| 前置条件 | 已登录；网络可访问新浪行情源 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/indices`（不带 symbols 参数），携带 Bearer token |
| 预期结果 | HTTP 200，响应 `{"code":0,"message":"success","data":[...]}`；data 为默认 9 大指数（`000001.SH`、`399001.SZ`、`399006.SZ`、`HSI`、`DJI`、`IXIC`、`SPX`、`DAX`、`CAC`）中可用者；每项含 `symbol`、`name`、`price`（>0）、`change`、`changePercent`、`volume`、`timestamp` |

### MKT-020 指数指定
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 功能 |
| 优先级 | P1 |
| 前置条件 | 已登录；网络可访问新浪行情源 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/indices?symbols=000001.SH,DJI`，携带 Bearer token |
| 预期结果 | HTTP 200；data 仅返回指定 2 个指数（`000001.SH` 与 `DJI`），不含其它默认指数 |

### MKT-021 指数非法 symbols
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 异常 |
| 优先级 | P2 |
| 前置条件 | 已登录 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/indices?symbols=NONEXIST`，携带 Bearer token |
| 预期结果 | HTTP 200；无匹配项时回退返回默认全量指数或空数组；不 500 |

### MKT-022 市场统计
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 功能 |
| 优先级 | P2 |
| 前置条件 | 已登录 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/statistics`，携带 Bearer token |
| 预期结果 | HTTP 200，响应 `{"code":0,"message":"success","data":{...}}`；data 含 `limitUpCount`、`limitDownCount`、`upCount`、`downCount`、`flatCount`、`totalAmount` 字段（数值可为 0，但结构必须完整） |

### MKT-023 基本面命中
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 功能 |
| 优先级 | P1 |
| 前置条件 | 已登录；网络可访问行情数据源 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/fundamentals/600519.SH`，携带 Bearer token |
| 预期结果 | HTTP 200；data 含 `marketCap`、`pe`、`pb`、`eps`、`roe`、`debtRatio`、`currentRatio`、`dividendYield` 字段；`pe>0` 时 `eps ≈ price/pe` |

### MKT-024 基本面未命中
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 异常 |
| 优先级 | P2 |
| 前置条件 | 已登录 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/fundamentals/NONEXIST123`，携带 Bearer token |
| 预期结果 | HTTP 200；data 各字段全为 0（降级返回，不报错、不 500） |

## 依赖故障

### MKT-025 行情依赖故障
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 异常 |
| 优先级 | P1 |
| 前置条件 | 已登录；断开外网或停用行情数据源（Tencent/EastMoney 均不可达） |
| 测试步骤 | 1. 调用 `GET /api/v1/market/realtime/600519.SH`，携带 Bearer token |
| 预期结果 | HTTP 500，响应 `{"error":"failed to fetch market data"}`；服务不崩溃；前端展示错误态并支持重试 |

---

# 二、实时行情（MKT）— 补充用例

## 权限门禁与市场推导

### MKT-026 未认证访问行情接口
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 异常 |
| 优先级 | P0 |
| 前置条件 | 未登录或持有无效 token |
| 测试步骤 | 1. 不带 Authorization 调用 `GET /api/v1/market/realtime/600519.SH`；2. 携带无效 token（如 `Bearer badtoken`）再调用一次 |
| 预期结果 | 情况 1：HTTP 401，响应 `{"error":"missing authorization token"}`；情况 2：HTTP 401，响应 `{"error":"invalid or expired token"}`；均不返回行情数据 |

### MKT-027 市场推导-港股后缀
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 功能 |
| 优先级 | P1 |
| 前置条件 | 已登录 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/realtime/00700.HK`，携带 Bearer token |
| 预期结果 | detectMarketCode 依据 `.HK` 后缀推导为 HK；按 spec 预期应返回 200 并成功采集 `symbol="00700.HK"`；若当前未接入 HK 采集，降级返回 404 `{"error":"symbol not found"}` 亦可接受，但不得 500 |

### MKT-028 市场推导-日股/欧股后缀
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 功能 |
| 优先级 | P1 |
| 前置条件 | 已登录 |
| 测试步骤 | 1. 依次调用 `GET /api/v1/market/realtime/7203.T`、`GET /api/v1/market/realtime/SIE.DE`，携带 Bearer token |
| 预期结果 | 市场推导分别为 JP（`.T`）、EU（`.DE`）；请求不崩溃；已接入市场返回 200 数据，未接入市场降级 404/400，不得 500 |

### MKT-029 市场推导非法组合
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 异常 |
| 优先级 | P2 |
| 前置条件 | 已登录 |
| 测试步骤 | 1. 依次调用 `GET /api/v1/market/realtime/123`（3 位纯数字）、`GET /api/v1/market/realtime/A1B2`（混合字符）、`GET /api/v1/market/realtime/600519SH`（无后缀），携带 Bearer token |
| 预期结果 | 均不 500；detectMarketCode 对 3 位纯数字、混合字符、无后缀代码一律回退推导为 US；返回 404 `{"error":"symbol not found"}` 或 200（取决于该市场采集是否接入），不得崩溃 |

## K 线补充

### MKT-030 K线自定义日期范围
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 功能 |
| 优先级 | P1 |
| 前置条件 | 已登录；网络可访问行情数据源 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/kline/600519.SH?interval=1d&from=2024-06-01&to=2024-06-30`，携带 Bearer token |
| 预期结果 | HTTP 200；响应 `from="2024-06-01"`、`to="2024-06-30"`；data 内每条 K 线的 `timestamp` 均在 [from, to] 区间内；`count` 与实际条数一致 |

### MKT-031 K线 to 日期非法
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 异常 |
| 优先级 | P1 |
| 前置条件 | 已登录 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/kline/600519.SH?to=2024/06/30`，携带 Bearer token |
| 预期结果 | HTTP 400，响应 `{"error":"invalid to date format, use YYYY-MM-DD"}` |

### MKT-032 K线多合法周期
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 功能 |
| 优先级 | P1 |
| 前置条件 | 已登录；网络可访问行情数据源 |
| 测试步骤 | 1. 依次调用 `GET /api/v1/market/kline/600519.SH?interval=5m`、`?interval=60m`、`?interval=1h`，携带 Bearer token |
| 预期结果 | 均返回 HTTP 200；响应 `interval` 与请求一致（`5m`/`60m`/`1h`）；data 为对应周期 K 线，`count>=0` |

## 回填补充

### MKT-033 回填并发触发
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 异常 |
| 优先级 | P1 |
| 前置条件 | 已登录；正在执行一次回填任务 |
| 测试步骤 | 1. 触发一次回填后，立即再次 `POST /api/v1/market/backfill`（相同或不同 symbols），均携带 Bearer token |
| 预期结果 | 两次请求均返回 HTTP 202；新任务接管后台上下文（旧任务被 cancel），旧 worker 退出；服务不崩溃、无死锁；最终 progress 中每个 symbol 收敛到 `completed` 或 `failed` |

### MKT-034 回填进度状态机
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 功能 |
| 优先级 | P2 |
| 前置条件 | 已登录；已触发一次回填 |
| 测试步骤 | 1. 触发 `POST /api/v1/market/backfill` 后，每 1s 轮询 `GET /api/v1/market/backfill/progress`，观察某 symbol 的 status 变化 |
| 预期结果 | 状态按 `pending → running → completed` 流转；`completed` 时 `progress=100` 且 `records>0`；失败时 `status="failed"` 且 `error` 字段非空；轮询期间接口始终 200 |

### MKT-035 回填非法 data_type
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 异常 |
| 优先级 | P1 |
| 前置条件 | 已登录 |
| 测试步骤 | 1. 调用 `POST /api/v1/market/backfill`，携带 Bearer token，请求体 `{"symbols":["600519.SH"],"data_type":"hourly","years":1}` |
| 预期结果 | HTTP 202，响应 `{"message":"backfill task started","data_type":"hourly","symbols":1}`；服务端按 default 分支以 daily 逻辑执行，不 500 |

## 指数与缓存补充

### MKT-036 指数参数大小写不敏感
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 功能 |
| 优先级 | P2 |
| 前置条件 | 已登录；网络可访问新浪行情源 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/indices?symbols=dji,dax`（小写），携带 Bearer token |
| 预期结果 | HTTP 200；匹配对大小写不敏感（EqualFold），data 含 `DJI` 与 `DAX`，且不含未指定的指数 |

### MKT-037 行情 Redis 缓存写入
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 衔接 |
| 优先级 | P1 |
| 前置条件 | 已登录；Redis 可用 |
| 测试步骤 | 1. 调用 `GET /api/v1/market/realtime/600519.SH`，携带 Bearer token；2. 用 `redis-cli GET market:realtime:600519.SH` 查询缓存 |
| 预期结果 | 接口返回 200；Redis 中存在 key `market:realtime:600519.SH`，值为最新价字符串，TTL≈30s；行情采集路径同时完成 TSDB 写入与回调通知（供 WebSocket 推送），三处数据一致 |

### MKT-038 批量含空串/非法符号
| 字段 | 内容 |
|---|---|
| 所属模块 | 实时行情 |
| 测试维度 | 异常 |
| 优先级 | P1 |
| 前置条件 | 已登录 |
| 测试步骤 | 1. 调用 `POST /api/v1/market/realtime/batch`，携带 Bearer token，请求体 `{"symbols":["600519.SH","","AAPL"]}` |
| 预期结果 | HTTP 200，不 500；空串与无法采集的符号被按市场分组（空串/混合字符回退 US），仅返回可采集项；`count` 与实际成功返回条数一致 |

---

# 三、WebSocket 实时推送（WS）

> WebSocket 测试通用约定：
> - 握手 URL：`ws://localhost:8080/ws?token=<JWT>`；鉴权通过 URL 查询参数 `token` 完成。
> - 客户端命令消息 JSON：`{"type":"subscribe","channel":"stock:600519.SH"}`、`{"type":"unsubscribe","channel":"stock:600519.SH"}`、`{"type":"ping"}`。
> - 服务端推送消息 JSON：`{"type":"quote","channel":"stock:600519.SH","data":{...MarketData...}}`（data 含 symbol/price/change/change_percent/volume 等）。
> - 服务端每 30s 下发协议级 Ping 帧；读超时 60s（无 Pong 则断开）；单条消息读取上限 4096 字节。
> - 前端（`frontend/src/services/websocket.ts`）连接成功后自动重发历史订阅；每 30s 发 `{"type":"heartbeat"}`；断线指数退避重连（1s→2s→…最大 30s）；40s 无任何消息触发主动重连兜底。

## 连接与鉴权

### WS-001 建立连接
| 字段 | 内容 |
|---|---|
| 所属模块 | WebSocket 实时推送 |
| 测试维度 | 功能 |
| 优先级 | P0 |
| 前置条件 | 已登录并取得 JWT token；后端已启动 |
| 测试步骤 | 1. 用 `ws://localhost:8080/ws?token=<token>` 发起 WebSocket 连接 |
| 预期结果 | 握手成功（HTTP 101 Switching Protocols），连接进入 OPEN 状态；30s 内收到服务端协议级 Ping 帧（连接确认/心跳）；持续空闲下连接不中断 |

### WS-002 未授权连接
| 字段 | 内容 |
|---|---|
| 所属模块 | WebSocket 实时推送 |
| 测试维度 | 异常 |
| 优先级 | P0 |
| 前置条件 | 后端已启动 |
| 测试步骤 | 1. 不带 token 连接 `ws://localhost:8080/ws`；2. 带无效 token 连接 `ws://localhost:8080/ws?token=badtoken` |
| 预期结果 | 情况 1：握手被拒，HTTP 401，响应 `{"error":"missing token query parameter"}`，不升级；情况 2：HTTP 401，响应 `{"error":"invalid or expired token"}`，不升级；两种均无法建立连接 |

## 订阅与推送

### WS-003 订阅行情
| 字段 | 内容 |
|---|---|
| 所属模块 | WebSocket 实时推送 |
| 测试维度 | 功能 |
| 优先级 | P0 |
| 前置条件 | 已建立合法连接 |
| 测试步骤 | 1. 发送 `{"type":"subscribe","channel":"stock:600519.SH"}` |
| 预期结果 | 收到 `{"type":"quote","channel":"stock:600519.SH","data":{...}}` 消息；data 含 `symbol="600519.SH"`、`price`、`change`、`change_percent`、`volume` 等字段，数值合理 |

### WS-004 行情更新推送
| 字段 | 内容 |
|---|---|
| 所属模块 | WebSocket 实时推送 |
| 测试维度 | 功能 |
| 优先级 | P0 |
| 前置条件 | 已订阅 `stock:600519.SH`；行情数据源有更新（采集周期默认 5s） |
| 测试步骤 | 1. 等待一个采集周期或手动触发一次采集；2. 观察收到的 quote 消息 |
| 预期结果 | 价格变化后 100ms 内收到该股票 `quote` 推送，`data.price` 与最新采集值一致；前端 `useStockQuote` 收到后更新最新价展示，无重复推送风暴 |

### WS-005 多股票订阅
| 字段 | 内容 |
|---|---|
| 所属模块 | WebSocket 实时推送 |
| 测试维度 | 功能 |
| 优先级 | P1 |
| 前置条件 | 已建立合法连接 |
| 测试步骤 | 1. 依次发送 20 条 `{"type":"subscribe","channel":"stock:<symbol>"}`（不同 symbol） |
| 预期结果 | 全部 20 个频道均能收到对应 `quote` 推送；无丢包/无明显延迟；前端批量合并渲染无卡顿 |

### WS-006 取消订阅
| 字段 | 内容 |
|---|---|
| 所属模块 | WebSocket 实时推送 |
| 测试维度 | 功能 |
| 优先级 | P1 |
| 前置条件 | 已订阅 `stock:600519.SH` 且能收到推送 |
| 测试步骤 | 1. 发送 `{"type":"unsubscribe","channel":"stock:600519.SH"}`；2. 等待 1-2 个采集周期，观察是否还有该频道消息 |
| 预期结果 | 取消后不再收到 `stock:600519.SH` 频道的 `quote` 消息；其它已订阅频道仍正常接收 |

## 心跳 / 重连 / 恢复

### WS-007 断线重连
| 字段 | 内容 |
|---|---|
| 所属模块 | WebSocket 实时推送 |
| 测试维度 | 异常 |
| 优先级 | P0 |
| 前置条件 | 前端页面已连接 WS 并订阅了若干频道 |
| 测试步骤 | 1. 模拟网络断开（如断网或服务端主动断开连接）；2. 恢复网络 |
| 预期结果 | 客户端触发 onclose；按指数退避自动重连（1s→2s→…最大 30s）；重连成功后（onopen）自动重发历史订阅频道；`useStockQuote` 重连后增量拉取一次最新行情（REST 回填），表格刷新无整页闪烁 |

### WS-008 心跳保活
| 字段 | 内容 |
|---|---|
| 所属模块 | WebSocket 实时推送 |
| 测试维度 | 功能 |
| 优先级 | P1 |
| 前置条件 | 已建立合法连接 |
| 测试步骤 | 1. 建立连接后保持空闲（不订阅任何频道），观察 90s 以上 |
| 预期结果 | 服务端每 30s 下发 Ping 帧，浏览器自动回 Pong，服务端 60s 读超时被持续刷新；前端每 30s 发送 `{"type":"heartbeat","timestamp":...}`；连接在 90s 空闲期内不被断开 |

### WS-009 暂停/恢复
| 字段 | 内容 |
|---|---|
| 所属模块 | WebSocket 实时推送 |
| 测试维度 | 功能 |
| 优先级 | P1 |
| 前置条件 | 打开市场页，实时表格已有数据 |
| 测试步骤 | 1. 点击"暂停刷新"按钮；2. 观察表格 5-10s；3. 点击"恢复刷新"按钮 |
| 预期结果 | 暂停期间（`paused=true`）不消费/不渲染实时消息（不触发 setState/onMessage），表格停止变化，页面显示"已暂停"Tag；恢复后立即执行一次全量同步（REST 拉取最新行情）并继续接收 WS 推送，表格同步到最新数据 |

### WS-010 服务端重启
| 字段 | 内容 |
|---|---|
| 所属模块 | WebSocket 实时推送 |
| 测试维度 | 异常 |
| 优先级 | P1 |
| 前置条件 | 前端已连接并订阅频道 |
| 测试步骤 | 1. 连接建立后重启后端进程；2. 等待客户端自动恢复 |
| 预期结果 | 客户端检测到断开（onclose）→ 自动重连 → 重连成功后重新订阅原频道 → 重新收到 `quote` 推送；整个恢复过程无需手动刷新页面 |

## 前端交互

### WS-011 行情表格实时刷新
| 字段 | 内容 |
|---|---|
| 所属模块 | WebSocket 实时推送 |
| 测试维度 | 交互 |
| 优先级 | P0 |
| 前置条件 | 打开市场页（`/market`），已加载股票列表 |
| 测试步骤 | 1. 观察"最新价/涨跌幅/涨跌额/成交量/成交额/最高/最低"各列；2. 等待行情推送到来 |
| 预期结果 | 表格随 WS `quote` 推送实时变化；变化的单元格出现闪烁高亮（约 500ms `cell-flash`）；上涨红色/下跌绿色/平盘默认配色正确；无行情数据的单元格显示 `--`；行点击可跳转个股详情页 |

---

# 四、WebSocket 实时推送（WS）— 补充用例

### WS-012 WS 消息 JSON 结构校验
| 字段 | 内容 |
|---|---|
| 所属模块 | WebSocket 实时推送 |
| 测试维度 | 异常 |
| 优先级 | P1 |
| 前置条件 | 已建立合法连接 |
| 测试步骤 | 1. 依次发送：合法订阅 `{"type":"subscribe","channel":"stock:600519.SH"}`、非法 JSON `{bad json`、未知类型 `{"type":"foo","channel":"stock:600519.SH"}` |
| 预期结果 | 非法 JSON 与未知 type 消息被服务端忽略（日志记录后继续读循环），连接保持不中断；此前的合法订阅仍生效，继续收到 `quote` 推送 |

### WS-013 客户端 ping→pong
| 字段 | 内容 |
|---|---|
| 所属模块 | WebSocket 实时推送 |
| 测试维度 | 功能 |
| 优先级 | P1 |
| 前置条件 | 已建立合法连接 |
| 测试步骤 | 1. 发送 `{"type":"ping"}` |
| 预期结果 | 收到服务端 `{"type":"pong"}`（channel 为空/data 为 null 时可能被 omitempty 省略）；连接保持活跃 |

### WS-014 订阅空频道
| 字段 | 内容 |
|---|---|
| 所属模块 | WebSocket 实时推送 |
| 测试维度 | 异常 |
| 优先级 | P2 |
| 前置条件 | 已建立合法连接 |
| 测试步骤 | 1. 发送 `{"type":"subscribe","channel":""}`；2. 再发送 `{"type":"subscribe","channel":"stock:600519.SH"}` |
| 预期结果 | 空频道被服务端忽略（不订阅、不报错），连接保持；后续合法订阅仍正常生效 |

### WS-015 订阅不存在标的
| 字段 | 内容 |
|---|---|
| 所属模块 | WebSocket 实时推送 |
| 测试维度 | 异常 |
| 优先级 | P1 |
| 前置条件 | 已建立合法连接 |
| 测试步骤 | 1. 发送 `{"type":"subscribe","channel":"stock:NONEXIST123"}`；2. 观察一段时间 |
| 预期结果 | 连接不崩溃；该频道无有效行情数据时不推送（或仅在有数据时推送）；同一连接上其它合法订阅不受影响 |

### WS-016 消息超限断开
| 字段 | 内容 |
|---|---|
| 所属模块 | WebSocket 实时推送 |
| 测试维度 | 异常 |
| 优先级 | P2 |
| 前置条件 | 已建立合法连接 |
| 测试步骤 | 1. 发送一条超过 4096 字节的文本消息（如超大 JSON 字符串） |
| 预期结果 | 服务端读上限（4096）触发，`ReadMessage` 返回错误，该连接被关闭（readPump 退出）；服务端其它连接与订阅不受影响 |

### WS-017 连接状态指示
| 字段 | 内容 |
|---|---|
| 所属模块 | WebSocket 实时推送 |
| 测试维度 | 交互 |
| 优先级 | P1 |
| 前置条件 | 打开任意使用 WS 的页面（如市场页） |
| 测试步骤 | 1. 正常连接时观察布局头部状态；2. 断开后端观察状态；3. 手动 `disconnect` 后观察状态 |
| 预期结果 | 连接正常显示"已连接"；断线后显示"重连中"（`reconnecting`）；手动断开后显示"已断开"（`disconnected`）；状态切换及时、无闪烁 |

### WS-018 断线失败提示
| 字段 | 内容 |
|---|---|
| 所属模块 | WebSocket 实时推送 |
| 测试维度 | 交互 |
| 优先级 | P1 |
| 前置条件 | 打开市场页 |
| 测试步骤 | 1. 停掉后端进程；2. 观察页面 30-60s；3. 重启后端 |
| 预期结果 | 断线期间页面不白屏、无未捕获异常，显示"重连中"状态；恢复后自动重连、重新订阅并同步最新行情，无需手动刷新 |

### WS-019 非法 Origin 握手拒绝
| 字段 | 内容 |
|---|---|
| 所属模块 | WebSocket 实时推送 |
| 测试维度 | 异常 |
| 优先级 | P1 |
| 前置条件 | 后端已配置 `allowedOrigins` 白名单（非空） |
| 测试步骤 | 1. 携带非白名单 `Origin` 头连接 `ws://localhost:8080/ws?token=<token>`；2. 携带白名单内 Origin 再连接一次 |
| 预期结果 | 非白名单 Origin 握手被拒（Upgrade 失败/HTTP 403），不建立连接；白名单 Origin 或无 Origin（同源/curl 客户端）握手成功，正常建立连接 |

### WS-020 多客户端并发
| 字段 | 内容 |
|---|---|
| 所属模块 | WebSocket 实时推送 |
| 测试维度 | 功能 |
| 优先级 | P2 |
| 前置条件 | 两个客户端均已建立合法连接 |
| 测试步骤 | 1. 两个客户端同时订阅 `stock:600519.SH`；2. 关闭其中一个客户端，观察另一个 |
| 预期结果 | 两个客户端均收到相同的 `quote` 推送，互不影响；关闭其一后，另一个仍持续接收推送，服务端 Hub 正确清理退出的客户端 |

---

# 附：用例清单汇总

| 编号 | 维度 | 标题 | 优先级 |
|---|---|---|---|
| MKT-001 | 功能 | 单只行情-A股 | P0 |
| MKT-002 | 功能 | 单只行情-美股 | P1 |
| MKT-003 | 功能 | 纯数字 A股 | P1 |
| MKT-004 | 功能 | 纯数字 港股 | P1 |
| MKT-005 | 异常 | symbol 为空 | P1 |
| MKT-006 | 异常 | symbol 不存在 | P1 |
| MKT-007 | 功能 | 批量行情 | P0 |
| MKT-008 | 异常 | 批量缺 symbols | P1 |
| MKT-009 | 边界 | 批量超 50 | P1 |
| MKT-010 | 功能 | K线默认 | P1 |
| MKT-011 | 异常 | K线非法 interval | P1 |
| MKT-012 | 异常 | K线日期格式错 | P1 |
| MKT-013 | 异常 | K线 to<from | P2 |
| MKT-014 | 功能 | 盘口深度 | P1 |
| MKT-015 | 异常 | 盘口未命中 | P1 |
| MKT-016 | 功能 | 触发回填 | P1 |
| MKT-017 | 异常 | 回填缺 symbols | P1 |
| MKT-018 | 功能 | 回填进度 | P2 |
| MKT-019 | 功能 | 指数默认 | P1 |
| MKT-020 | 功能 | 指数指定 | P1 |
| MKT-021 | 异常 | 指数非法 symbols | P2 |
| MKT-022 | 功能 | 市场统计 | P2 |
| MKT-023 | 功能 | 基本面命中 | P1 |
| MKT-024 | 异常 | 基本面未命中 | P2 |
| MKT-025 | 异常 | 行情依赖故障 | P1 |
| MKT-026 | 异常 | 未认证访问行情接口 | P0 |
| MKT-027 | 功能 | 市场推导-港股后缀 | P1 |
| MKT-028 | 功能 | 市场推导-日股/欧股后缀 | P1 |
| MKT-029 | 异常 | 市场推导非法组合 | P2 |
| MKT-030 | 功能 | K线自定义日期范围 | P1 |
| MKT-031 | 异常 | K线 to 日期非法 | P1 |
| MKT-032 | 功能 | K线多合法周期 | P1 |
| MKT-033 | 异常 | 回填并发触发 | P1 |
| MKT-034 | 功能 | 回填进度状态机 | P2 |
| MKT-035 | 异常 | 回填非法 data_type | P1 |
| MKT-036 | 功能 | 指数参数大小写不敏感 | P2 |
| MKT-037 | 衔接 | 行情 Redis 缓存写入 | P1 |
| MKT-038 | 异常 | 批量含空串/非法符号 | P1 |
| WS-001 | 功能 | 建立连接 | P0 |
| WS-002 | 异常 | 未授权连接 | P0 |
| WS-003 | 功能 | 订阅行情 | P0 |
| WS-004 | 功能 | 行情更新推送 | P0 |
| WS-005 | 功能 | 多股票订阅 | P1 |
| WS-006 | 功能 | 取消订阅 | P1 |
| WS-007 | 异常 | 断线重连 | P0 |
| WS-008 | 功能 | 心跳保活 | P1 |
| WS-009 | 功能 | 暂停/恢复 | P1 |
| WS-010 | 异常 | 服务端重启 | P1 |
| WS-011 | 交互 | 行情表格实时刷新 | P0 |
| WS-012 | 异常 | WS 消息 JSON 结构校验 | P1 |
| WS-013 | 功能 | 客户端 ping→pong | P1 |
| WS-014 | 异常 | 订阅空频道 | P2 |
| WS-015 | 异常 | 订阅不存在标的 | P1 |
| WS-016 | 异常 | 消息超限断开 | P2 |
| WS-017 | 交互 | 连接状态指示 | P1 |
| WS-018 | 交互 | 断线失败提示 | P1 |
| WS-019 | 异常 | 非法 Origin 握手拒绝 | P1 |
| WS-020 | 功能 | 多客户端并发 | P2 |
