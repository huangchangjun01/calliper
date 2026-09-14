# 智能量化交易系统全量测试用例设计 Spec

## Why
系统包含认证、股票检索、实时行情、WebSocket 推送、机器学习预测、预测评估、账户管理、真实/模拟交易、管理后台、ML 服务等多个模块，后端(Go)/前端(React)/ML服务(Python) 三端联动。目前缺少一整套完整、可执行的测试用例，无法系统验证各功能是否符合预期、全流程是否贯通、跨模块数据衔接是否准确、异常场景是否被正确处理、前端交互是否友好。本 spec 在需求/设计层将测试方案细化到**每个 API 端点、每个分支、每个边界值**，确保最终产出可直接执行的用例文档。

## What Changes
- 新增一套覆盖全部业务模块与全部 API 端点的测试用例文档，存放于 `docs/test-cases/`
- 每个用例按 5 个维度设计：功能符合预期 / 全流程贯通 / 跨模块数据衔接 / 异常输入·异常场景 / 前端友好交互
- 用例字段模板（强制）：`编号 | 所属模块 | 测试维度 | 标题 | 前置条件 | 测试步骤(含具体输入) | 预期结果(含具体状态码/响应字段) | 优先级`
- 每个 API 端点至少覆盖：正常路径、参数边界、非法输入、依赖服务不可用、权限门禁（如适用）
- 新增跨模块专项文档：全流程贯通（端到端用户旅程）、跨模块数据衔接（模块间数据流转校验）
- **BREAKING**: 无（纯新增测试文档，不改动任何业务代码）

## Impact
- Affected specs: 无既有 spec 直接受影响；基于 quant-trading-system 及后续各 fix 模块的功能约定
- Affected code: 无代码改动；新增 `docs/test-cases/` 目录下文档

---

# 测试体系约定（将写入 README.md）

## 编号规则
`<模块前缀>-<3位序号>`，模块前缀映射：
| 前缀 | 模块 |
|---|---|
| AUTH | 认证与账户管理 |
| STOCK | 股票检索与自选股 |
| MKT | 实时行情 |
| WS | WebSocket 实时推送 |
| PRED | 预测模块 |
| EVAL | 评估模块 |
| TRADE | 交易模块 |
| SIM | 模拟交易 |
| ADM | 管理后台 |
| ML | ML 服务 |
| DASH | 决策支持仪表盘 |
| E2E | 全流程贯通 |
| LINK | 跨模块数据衔接 |

## 测试维度标签
| 维度 | 标识 |
|---|---|
| 功能符合预期 | 功能 |
| 全流程贯通 | 流程 |
| 跨模块数据衔接 | 衔接 |
| 异常输入·异常场景 | 异常 |
| 前端友好交互 | 交互 |

## 优先级
- P0：核心链路，阻断上线（认证、行情、预测主链路、评估、下单、模拟交易启停、管理后台登录权限）
- P1：主要功能边界与异常，影响用户体验但不阻断主链路
- P2：次要边界、展示细节、文案类

## 统一测试数据约定
- 测试账号：`admin/admin123`（admin 角色）、`testuser/pass1234`（user 角色，若不存在由注册用例创建）
- 测试股票：A股 `600519.SH`、`000001.SZ`；港股 `00700.HK`；美股 `AAPL`、`TSLA`；日股 `7203.T`；欧股 `SIE.DE`（或后端 stocks 表实际存在的代码）
- 无效 symbol：`"", "NONEXIST123"`、`123`（位数不匹配）、超长字符串

---

## ADDED Requirements

### Requirement: 认证模块（auth）测试用例

端点：`POST /api/v1/auth/login`、`POST /api/v1/auth/register`、`POST /api/v1/auth/refresh`

| 编号 | 维度 | 标题 | 步骤(输入) | 预期结果 | 优先级 |
|---|---|---|---|---|---|
| AUTH-001 | 功能 | 登录成功 | `POST /login` body `{"username":"admin","password":"admin123"}` | 200，响应含 `token`(非空)、`user_id`、`role="admin"` | P0 |
| AUTH-002 | 异常 | 密码错误 | `{"username":"admin","password":"wrong"}` | 401 `{"error":"invalid username or password"}`，不泄漏用户名是否存在 | P0 |
| AUTH-003 | 异常 | 用户不存在 | `{"username":"nobody","password":"x"}` | 401，同上（与 002 一致，防枚举） | P1 |
| AUTH-004 | 异常 | 用户被停用 | 构造 is_active=false 用户后登录 | 401 | P1 |
| AUTH-005 | 异常 | 缺少字段 | `{"username":"admin"}`（无 password） | 400 `{"error":"invalid request"}` | P1 |
| AUTH-006 | 异常 | 非法 JSON | body=`{bad json` | 400 | P1 |
| AUTH-007 | 异常 | 数据库不可用 | 停库后登录 | 503 `{"error":"authentication service unavailable"}` | P2 |
| AUTH-008 | 功能 | 注册成功 | `POST /register` body `{"username":"u_test","password":"pass1234","email":"u_test@x.com"}` | 201，响应 `user_id`、`message` | P0 |
| AUTH-009 | 异常 | 用户名重复 | 已存在 username 再注册 | 409 `{"error":"username or email already exists"}` | P1 |
| AUTH-010 | 异常 | 邮箱重复 | 同邮箱不同用户名 | 409 | P1 |
| AUTH-011 | 异常 | 注册缺字段 | 缺 email | 400 | P1 |
| AUTH-012 | 异常 | 注册弱输入 | username 超长(>255)、password 空串 | 400（binding required） | P2 |
| AUTH-013 | 功能 | 刷新令牌 | 带有效 token 调 `POST /auth/refresh` | 200，新 `token`、`user_id`、`role` | P1 |
| AUTH-014 | 异常 | 刷新无 token | 不带 Authorization | 401 | P1 |
| AUTH-015 | 交互 | 登录页校验 | 前端空表单点登录 | 前端提示用户名/密码必填，不发请求 | P1 |
| AUTH-016 | 交互 | 登录成功跳转 | 登录成功 | 跳转 `from` 目标页或 `/`，顶部显示用户名 | P0 |
| AUTH-017 | 交互 | 登录失败提示 | 错误密码登录 | 展示后端错误信息，不清空密码框 | P1 |
| AUTH-018 | 交互 | 注册成功引导 | 注册成功 | 跳转登录页并提示"注册成功，请登录" | P1 |
| AUTH-019 | 交互 | 退出登录 | 点击退出 | 清 token/user，断开 WebSocket，跳 /login | P0 |

### Requirement: 账户管理模块（account）测试用例

端点：`GET /account/me`、`PUT /account/me`、`PUT /account/me/password`

| 编号 | 维度 | 标题 | 步骤(输入) | 预期结果 | 优先级 |
|---|---|---|---|---|---|
| AUTH-020 | 功能 | 查询个人信息 | 登录后 `GET /account/me` | 200，返回当前用户 id/username/email/role | P1 |
| AUTH-021 | 功能 | 更新资料 | `PUT /account/me` `{"email":"new@x.com"}` | 200，返回更新后 DTO，DB 同步 | P1 |
| AUTH-022 | 异常 | 邮箱已存在 | 更新为他人已用邮箱 | 400/409，明确提示 | P1 |
| AUTH-023 | 异常 | 邮箱为空 | `{"email":""}` | 400 | P1 |
| AUTH-024 | 功能 | 修改密码成功 | `PUT /account/me/password` `{"old_password":"pass1234","new_password":"newpass123"}` | 200，旧密码登录失败、新密码登录成功 | P1 |
| AUTH-025 | 异常 | 旧密码错误 | 旧密码填错 | 400，提示旧密码错误 | P1 |
| AUTH-026 | 异常 | 新密码过短 | 新密码 <6 位 | 400 | P1 |
| AUTH-027 | 异常 | 未登录访问 | 无 token 调 `GET /account/me` | 401 | P0 |

### Requirement: 股票检索模块（stocks）测试用例

端点：`GET /stocks/search`、`GET /stocks/market/:code`、`GET /stocks/:symbol`、`POST /stocks/sync/:market`、`GET /stocks/health`

| 编号 | 维度 | 标题 | 步骤(输入) | 预期结果 | 优先级 |
|---|---|---|---|---|---|
| STOCK-001 | 功能 | 关键词搜索 | `GET /stocks/search?q=apple&market=US&page=1&page_size=10` | 200，返回匹配股票列表+分页字段，命中 Apple 相关 | P0 |
| STOCK-002 | 功能 | 市场分组搜索 | `GET /stocks/search?q=600` | 按市场返回含"600"的 A 股 | P1 |
| STOCK-003 | 功能 | 按市场查询 | `GET /stocks/market/US` | 200，返回美股列表 | P1 |
| STOCK-004 | 异常 | 非法市场代码 | `GET /stocks/market/XX` | 400/空列表，不崩溃 | P1 |
| STOCK-005 | 异常 | 空关键词 | `GET /stocks/search?q=` | 200 空列表或分页全量（按实现），不 500 | P1 |
| STOCK-006 | 异常 | 分页越界 | `page=99999` | 200 空列表，不报错 | P2 |
| STOCK-007 | 功能 | 按代码查详情 | `GET /stocks/600519.SH` | 200，返回该股票信息 | P1 |
| STOCK-008 | 异常 | 代码不存在 | `GET /stocks/NONEXIST123` | 404/空，不 500 | P1 |
| STOCK-009 | 功能 | 手动同步 | `POST /stocks/sync/US` | 202/200，stocks 表更新，返回同步结果 | P1 |
| STOCK-010 | 异常 | 非法同步市场 | `POST /stocks/sync/XX` | 400/明确错误 | P1 |
| STOCK-011 | 功能 | 健康检查 | `GET /stocks/health` | 200，返回服务健康状态 | P2 |
| STOCK-012 | 交互 | 市场 Tab 切换 | 前端切"美股"Tab | URL 参数与列表刷新，选中态正确 | P1 |
| STOCK-013 | 交互 | 搜索防抖 | 连续输入关键词 | 仅触发最终关键词查询，无重复请求风暴 | P2 |
| STOCK-014 | 交互 | 空结果提示 | 搜索无匹配 | 展示"无匹配股票"空态，非白屏 | P1 |

### Requirement: 自选股（watchlist）测试用例

端点：`GET /stocks/watchlist`、`POST /stocks/watchlist/:symbol`、`DELETE /stocks/watchlist/:symbol`

| 编号 | 维度 | 标题 | 步骤(输入) | 预期结果 | 优先级 |
|---|---|---|---|---|---|
| STOCK-015 | 功能 | 添加自选 | `POST /stocks/watchlist/600519.SH` | 200，加入后 GET watchlist 可见 | P1 |
| STOCK-016 | 异常 | 重复添加 | 再次 POST 同一 symbol | 幂等成功或 409 明确提示，不产生重复记录 | P1 |
| STOCK-017 | 功能 | 删除自选 | `DELETE /stocks/watchlist/600519.SH` | 200，GET 不再包含 | P1 |
| STOCK-018 | 异常 | 删除不存在 | `DELETE /stocks/watchlist/NONEXIST123` | 200 幂等或 404 明确提示 | P2 |
| STOCK-019 | 异常 | 未登录操作自选 | 无 token | 401 | P0 |
| STOCK-020 | 功能 | 用户隔离 | 用户 A 添加，用户 B 查询 | B 的 watchlist 不含 A 的数据 | P1 |
| STOCK-021 | 交互 | 自选页展示 | 前端自选列表 | 展示代码/名称/实时价，空态提示"暂无自选" | P1 |

### Requirement: 实时行情模块（market）测试用例

端点：`GET /market/realtime/:symbol`、`POST /market/realtime/batch`、`GET /market/kline/:symbol`、`GET /market/depth/:symbol`、`POST /market/backfill`、`GET /market/backfill/progress`、`GET /market/indices`、`GET /market/statistics`、`GET /market/fundamentals/:symbol`

| 编号 | 维度 | 标题 | 步骤(输入) | 预期结果 | 优先级 |
|---|---|---|---|---|---|
| MKT-001 | 功能 | 单只行情-A股 | `GET /market/realtime/600519.SH` | 200 `data` 含 symbol/price/change/volume 等字段 | P0 |
| MKT-002 | 功能 | 单只行情-美股 | `GET /market/realtime/AAPL` | 200，市场推导为 US 并成功采集 | P1 |
| MKT-003 | 功能 | 纯数字 A股 | `GET /market/realtime/000001`（6位数字） | 200，推导 CN | P1 |
| MKT-004 | 功能 | 纯数字 港股 | `GET /market/realtime/00700`（5位数字） | 200，推导 HK | P1 |
| MKT-005 | 异常 | symbol 为空 | `GET /market/realtime/` | 400 `{"error":"symbol is required"}` | P1 |
| MKT-006 | 异常 | symbol 不存在 | `GET /market/realtime/NONEXIST123` | 404 `{"error":"symbol not found"}` | P1 |
| MKT-007 | 功能 | 批量行情 | `POST /market/realtime/batch` `{"symbols":["600519.SH","AAPL"]}` | 200 `data`+`count=2` | P0 |
| MKT-008 | 异常 | 批量缺 symbols | `{"symbols":[]}` 或缺字段 | 400 `{"error":"invalid request"}` | P1 |
| MKT-009 | 边界 | 批量超 50 | 传 60 个 symbol | 截断到 50，count≤50，不报错 | P1 |
| MKT-010 | 功能 | K线默认 | `GET /market/kline/600519.SH`（默认 interval=1d） | 200 `data` K线数组+count | P1 |
| MKT-011 | 异常 | K线非法 interval | `?interval=2h` | 400 `invalid interval, supported: 1m,5m,15m,30m,60m,1d` | P1 |
| MKT-012 | 异常 | K线日期格式错 | `?from=2024/01/01` | 400 `invalid from date format` | P1 |
| MKT-013 | 异常 | K线 to<from | `?from=2024-01-01&to=2023-01-01` | 400 或空数据（不崩溃） | P2 |
| MKT-014 | 功能 | 盘口深度 | `GET /market/depth/600519.SH` | 200 `bid_prices/bid_volumes/ask_prices/ask_volumes` | P1 |
| MKT-015 | 异常 | 盘口未命中 | `GET /market/depth/NONEXIST123` | 404 `{"error":"symbol not found"}` | P1 |
| MKT-016 | 功能 | 触发回填 | `POST /market/backfill` `{"symbols":["600519.SH"],"data_type":"daily","years":3}` | 202 `backfill task started` | P1 |
| MKT-017 | 异常 | 回填缺 symbols | `{"symbols":[]}` | 400 | P1 |
| MKT-018 | 功能 | 回填进度 | `GET /market/backfill/progress` | 200 返回进度对象 | P2 |
| MKT-019 | 功能 | 指数默认 | `GET /market/indices` | 200 默认 9 大指数数据 | P1 |
| MKT-020 | 功能 | 指数指定 | `GET /market/indices?symbols=000001.SH,DJI` | 200 仅返回指定指数 | P1 |
| MKT-021 | 异常 | 指数非法 symbols | `?symbols=NONEXIST` | 200 返回默认全量或空（不 500） | P2 |
| MKT-022 | 功能 | 市场统计 | `GET /market/statistics` | 200 返回涨跌家数/成交额字段 | P2 |
| MKT-023 | 功能 | 基本面命中 | `GET /market/fundamentals/600519.SH` | 200 marketCap/pe/pb/eps 字段 | P1 |
| MKT-024 | 异常 | 基本面未命中 | `GET /market/fundamentals/NONEXIST123` | 200 全 0 字段（降级不报错） | P2 |
| MKT-025 | 异常 | 行情依赖故障 | 停外网/停数据源 | 500 `failed to fetch market data`，前端显示错误态可重试 | P1 |

### Requirement: WebSocket 实时推送（ws）测试用例

端点：`GET /ws`（握手带 JWT），订阅频道 `stock:{symbol}`，消息类型 `quote`

| 编号 | 维度 | 标题 | 步骤(输入) | 预期结果 | 优先级 |
|---|---|---|---|---|---|
| WS-001 | 功能 | 建立连接 | 携带有效 token 连接 `/ws` | 握手成功，收到连接确认/心跳 | P0 |
| WS-002 | 异常 | 未授权连接 | 无 token / 无效 token 连接 | 握手拒绝或立即关闭，返回鉴权错误 | P0 |
| WS-003 | 功能 | 订阅行情 | 订阅 `stock:600519.SH` | 收到 `quote` 类型消息，含 price/change 等 | P0 |
| WS-004 | 功能 | 行情更新推送 | 该股票价格变化 | 前端 100ms 内收到推送并更新展示 | P0 |
| WS-005 | 功能 | 多股票订阅 | 同时订阅 20 只 | 全部收到推送，前端无卡顿 | P1 |
| WS-006 | 功能 | 取消订阅 | unsubscribe 后 | 不再收到该频道消息 | P1 |
| WS-007 | 异常 | 断线重连 | 模拟网络断开/服务重启 | 客户端自动重连，重连后恢复订阅并增量同步 | P0 |
| WS-008 | 功能 | 心跳保活 | 空闲连接 | 定时心跳，连接不因超时被断 | P1 |
| WS-009 | 功能 | 暂停/恢复 | 前端暂停刷新 | 暂停期间不消费消息，恢复后同步最新数据 | P1 |
| WS-010 | 异常 | 服务端重启 | 重启后端 | 客户端检测断开→重连→重新订阅成功 | P1 |
| WS-011 | 交互 | 行情表格实时刷新 | 打开市场页 | 表格最新价/涨跌幅随 WS 实时变化，数字闪烁/高亮可选 | P0 |

### Requirement: 预测模块（predictions）测试用例

端点：`GET /predictions/summaries`、`GET /predictions/details`、`GET /predictions/history`、`GET /predictions/stats`、`GET /predictions/accuracy`、`GET /predictions/stock-accuracy`、`GET /predictions/failures`、`GET /predictions/accuracy/:symbol`、`GET /predictions/:symbol/history`、`GET /predictions/:symbol`、`POST /predictions/batch`、`POST /predictions/generate`、`POST /predictions/run`(admin)

| 编号 | 维度 | 标题 | 步骤(输入) | 预期结果 | 优先级 |
|---|---|---|---|---|---|
| PRED-001 | 功能 | 预测概览 | `GET /predictions/summaries` | 200 按周期返回数量/方向统计 | P1 |
| PRED-002 | 功能 | 明细全量 | `GET /predictions/details` | 200 `{items,total,limit:20,offset:0}` | P0 |
| PRED-003 | 功能 | 明细过滤 | `?period=short_term&direction=up&confidence_min=60&expired=false` | 200 仅返回满足条件记录 | P1 |
| PRED-004 | 边界 | 明细 limit 上限 | `?limit=500` | limit 钳制到 100 | P1 |
| PRED-005 | 异常 | confidence_min 非数字 | `?confidence_min=abc` | 忽略该参数，不 500 | P2 |
| PRED-006 | 功能 | 历史分页 | `?symbol=600519.SH&period=short_term&status=pending` | 200 分页历史列表 | P1 |
| PRED-007 | 功能 | 统计 day/week/month | `?bucket=week` | 200 各桶统计（无评估桶 accuracy 为 null） | P1 |
| PRED-008 | 异常 | 非法 bucket | `?bucket=hour` | 回退 day，不 500 | P2 |
| PRED-009 | 功能 | 准确率趋势 | `?period=short&days=30` | 200 每日 accuracy 序列，缺口为 null | P1 |
| PRED-010 | 边界 | days 越界 | `?days=0` / `?days=999` | 回退 30，不 500 | P2 |
| PRED-011 | 功能 | 个股准确率排行 | `GET /predictions/stock-accuracy` | 200 按准确率排序 | P1 |
| PRED-012 | 功能 | 失败归因 | `GET /predictions/failures?limit=20` | 200 返回 wrong 预测及实际方向 | P1 |
| PRED-013 | 功能 | 单只预测 | `GET /predictions/600519.SH` | 200 返回该股预测结果 | P0 |
| PRED-014 | 异常 | 预测 symbol 空 | `GET /predictions/` | 400 `symbol is required` | P1 |
| PRED-015 | 功能 | 批量预测 | `POST /predictions/batch` `{"symbols":["600519.SH","AAPL"]}` | 200 每只股票预测结果 | P0 |
| PRED-016 | 异常 | 批量空 symbols | `{"symbols":[]}` | 400 `symbols list is required` | P1 |
| PRED-017 | 功能 | 生成并持久化 | `POST /predictions/generate` `{"symbols":["600519.SH"]}` | 200 `status:success`，`persisted` 计数，DB 落库 | P0 |
| PRED-018 | 异常 | generate 空 symbols | `{"symbols":[]}` | 400 | P1 |
| PRED-019 | 异常 | ML 服务不可用 | 停 ML 服务后请求预测 | 500/503，前端提示"预测服务暂不可用"可重试 | P1 |
| PRED-020 | 功能 | admin 触发预测 | admin 调 `POST /predictions/run` | 200 `Daily prediction task triggered` | P1 |
| PRED-021 | 交互 | 预测页筛选 | 前端选周期/方向/置信度 | 表格按条件刷新，URL 参数同步 | P1 |
| PRED-022 | 交互 | 预测详情跳转 | 点击某预测行 | 跳转 `/stocks/{symbol}` 详情页 | P1 |

### Requirement: 评估模块（evaluation）测试用例

端点：`GET /evaluation/accuracy/:symbol/stats`、`GET /evaluation/accuracy/:symbol`、`GET /evaluation/ranking`、`GET /evaluation/metrics/:symbol`、`GET /evaluation/failure/:symbol`、`POST /evaluation/run`(admin)

| 编号 | 维度 | 标题 | 步骤(输入) | 预期结果 | 优先级 |
|---|---|---|---|---|---|
| EVAL-001 | 功能 | 个股准确率统计 | `GET /evaluation/accuracy/600519.SH/stats` | 200 近7/近30/累计准确率 | P1 |
| EVAL-002 | 异常 | symbol 空 | `GET /evaluation/accuracy//stats` | 400 | P1 |
| EVAL-003 | 功能 | 准确率排行 | `GET /evaluation/ranking` | 200 按准确率降序，含 total_predictions | P1 |
| EVAL-004 | 功能 | 评估指标 | `GET /evaluation/metrics/600519.SH` | 200 方向准确率/夏普/回撤等指标 | P1 |
| EVAL-005 | 功能 | 失败归因分析 | `GET /evaluation/failure/600519.SH` | 200 失败次数与影响因素 | P1 |
| EVAL-006 | 功能 | admin 触发评估 | admin `POST /evaluation/run` | 200 触发成功 | P1 |
| EVAL-007 | 异常 | 非 admin 触发 | user 调 `POST /evaluation/run` | 403 | P0 |
| EVAL-008 | 阈值 | 低准确率挂起 | 构造某股 30 日准确率<45 | 该股不再出现在高置信度推荐；accuracy 接口返回低值 | P1 |
| EVAL-009 | 阈值 | 连续低准确率重训 | 构造连续多日 <60 | 评估调度标记需重训（versions.json/状态接口体现） | P2 |
| EVAL-010 | 交互 | 评估面板空态 | 无评估数据时 | 展示"暂无评估数据"空态，不白屏 | P1 |

### Requirement: 交易模块（trading）测试用例

端点：`POST /trading/order`、`DELETE /trading/order/:id`、`GET /trading/orders`、`GET /trading/order/:id`、`GET /trading/positions`、`GET /trading/account`

| 编号 | 维度 | 标题 | 步骤(输入) | 预期结果 | 优先级 |
|---|---|---|---|---|---|
| TRADE-001 | 功能 | 模拟买入成功 | `POST /trading/order` `{"symbol":"600519.SH","action":"buy","order_type":"limit","price":"1500.00","quantity":100,"trade_password":"pass1234"}` | 200 返回订单，成交/挂单状态正确 | P0 |
| TRADE-002 | 异常 | 交易密码为空 | 同上但 `trade_password:""` | 400 错误码 40002 `交易密码不能为空` | P1 |
| TRADE-003 | 异常 | 交易密码错误 | 填错误密码 | 401 错误码 40103 `交易密码错误` | P1 |
| TRADE-004 | 异常 | 价格格式非法 | `price:"abc"` | 400 错误码 40003 `invalid price format` | P1 |
| TRADE-005 | 异常 | 价格负数/零 | `price:"-1"` / `price:"0"` | 400/明确拒绝 | P1 |
| TRADE-006 | 异常 | quantity 非法 | `quantity:0` / `quantity:-5` | 400/明确拒绝 | P1 |
| TRADE-007 | 异常 | 缺必填字段 | 缺 action | 400 `invalid request` | P1 |
| TRADE-008 | 门禁 | 非 admin 真实交易 | user 提交 `is_real:true`，REAL_TRADING_ENABLED=true | 强制降级为 simulated，绝不真实成交 | P0 |
| TRADE-009 | 门禁 | 开关关闭 | admin 提交 `is_real:true`，REAL_TRADING_ENABLED=false | 仍为 simulated | P0 |
| TRADE-010 | 门禁 | 开关开启+admin | admin 提交 `is_real:true`，REAL_TRADING_ENABLED=true | 允许 real（mock broker） | P1 |
| TRADE-011 | 异常 | 余额不足 | 资金不足以覆盖买入 | 拒绝下单，返回明确错误 | P1 |
| TRADE-012 | 异常 | 单票仓位超限 | 买入后单票 >20% | 拒绝，提示仓位上限 | P1 |
| TRADE-013 | 功能 | 撤单 | `DELETE /trading/order/{id}`（挂单状态） | 200 `订单已撤单`，状态变 canceled | P1 |
| TRADE-014 | 异常 | 撤不存在订单 | 随机 id | 500/404 明确错误，不崩溃 | P1 |
| TRADE-015 | 功能 | 订单查询 | `GET /trading/orders?status=all&limit=20` | 200 `{orders,total,limit,offset}` | P1 |
| TRADE-016 | 功能 | 持仓查询 | `GET /trading/positions` | 200 positions 列表 | P1 |
| TRADE-017 | 功能 | 账户查询 | `GET /trading/account` | 200 余额/可用/冻结 | P1 |
| TRADE-018 | 交互 | 交易表单校验 | 前端提交空表单/负数 | 前端即时校验提示，不提交 | P1 |
| TRADE-019 | 交互 | 下单成功反馈 | 下单成功 | 成功提示+订单/持仓列表刷新 | P1 |
| TRADE-020 | 交互 | 真实/模拟标识 | 前端交易面板 | 明确区分"真实交易/模拟交易"标识与开关状态 | P1 |

### Requirement: 模拟交易模块（sim）测试用例

端点：`GET /trading/sim/status`、`POST /trading/sim/start`、`POST /trading/sim/stop`、`POST /trading/sim/trigger`、`GET /trading/sim/decisions`、`GET /trading/sim/history/dates`、`GET /trading/sim/history`、`GET /trading/sim/account`、`GET /trading/sim/positions`、`GET /trading/sim/trades`

| 编号 | 维度 | 标题 | 步骤(输入) | 预期结果 | 优先级 |
|---|---|---|---|---|---|
| SIM-001 | 功能 | 查询状态 | `GET /trading/sim/status` | 200 运行中/已停止/未初始化 | P0 |
| SIM-002 | 功能 | 启动模拟交易 | `POST /trading/sim/start` | 200 启动成功，状态变 running，调度器启动 | P0 |
| SIM-003 | 异常 | 重复启动 | 已 running 再 start | 幂等/明确提示"已在运行"，不重复调度 | P1 |
| SIM-004 | 功能 | 停止模拟交易 | `POST /trading/sim/stop` | 200 停止成功，状态变 stopped | P0 |
| SIM-005 | 异常 | 未启动即停止 | 已 stopped 再 stop | 幂等/明确提示，不报错 | P1 |
| SIM-006 | 功能 | 手动触发决策 | `POST /trading/sim/trigger` | 200 触发一轮决策 | P1 |
| SIM-007 | 功能 | 决策生成 | 有预测数据时 trigger | 生成 buy/sell/hold 决策，置信度过滤(≥55)生效 | P1 |
| SIM-008 | 衔接 | 决策消费预测 | 无有效预测时 trigger | 降级调用 ML 预测；ML 不可用时决策为空不崩溃 | P1 |
| SIM-009 | 功能 | 查询决策 | `GET /trading/sim/decisions` | 200 决策列表 | P1 |
| SIM-010 | 功能 | 历史日期 | `GET /trading/sim/history/dates` | 200 有成交的日期列表 | P1 |
| SIM-011 | 功能 | 按日期查历史 | `GET /trading/sim/history?date=YYYY-MM-DD` | 200 当日成交记录 | P1 |
| SIM-012 | 异常 | 非法日期 | `?date=abc` | 400/明确错误 | P1 |
| SIM-013 | 功能 | 模拟账户 | `GET /trading/sim/account` | 200 模拟资金/盈亏 | P1 |
| SIM-014 | 功能 | 模拟持仓 | `GET /trading/sim/positions` | 200 持仓列表 | P1 |
| SIM-015 | 功能 | 模拟成交 | `GET /trading/sim/trades` | 200 成交记录 | P1 |
| SIM-016 | 风险 | 单日亏损>5%暂停 | 构造模拟账户单日亏损>5% | 当日自动暂停，状态/日志记录风险事件 | P1 |
| SIM-017 | 风控 | 单票仓位上限 | 决策超 20% 单票 | 自动调整到上限内 | P1 |
| SIM-018 | 风控 | 行业暴露上限 | 行业超 40% | 自动调整 | P2 |
| SIM-019 | 交互 | 启停按钮状态 | 前端模拟交易面板 | 运行中显示"停止"，停止显示"启动"，禁用态正确，无全局 loading 串扰 | P1 |
| SIM-020 | 交互 | 决策/成交展示 | 前端 | 决策列表、成交记录、模拟账户数据正常渲染 | P1 |

### Requirement: 管理后台模块（admin）测试用例

端点：`/admin/users` CRUD、`/admin/audit-log`、`/admin/system/status`、`/admin/predictions/run`、`/admin/models/status`、`/admin/evaluation/run`、`/admin/datasources`、`/admin/health`、`/admin/errors`、`/admin/latency`、`/admin/models`、`/admin/training/history`、`/admin/training/run`、`/admin/training/schedule`、`/admin/training/rollback`、`/admin/models/:id/params`(GET/PUT)、`/admin/models/:id/evaluate`、`/admin/models/:id/predict`

| 编号 | 维度 | 标题 | 步骤(输入) | 预期结果 | 优先级 |
|---|---|---|---|---|---|
| ADM-001 | 门禁 | 未登录访问 | 无 token 调 `GET /admin/users` | 401 | P0 |
| ADM-002 | 门禁 | 普通用户访问 | user token 调 `GET /admin/users` | 403 | P0 |
| ADM-003 | 功能 | 用户列表 | admin `GET /admin/users` | 200 用户列表 | P1 |
| ADM-004 | 功能 | 创建用户 | `POST /admin/users` 合法 body | 201/200 创建成功 | P1 |
| ADM-005 | 异常 | 创建重复用户 | 已存在用户名 | 409 | P1 |
| ADM-006 | 功能 | 更新用户 | `PUT /admin/users/:id`（改角色/停用） | 200，权限即时生效 | P1 |
| ADM-007 | 功能 | 删除用户 | `DELETE /admin/users/:id` | 200，该用户无法再登录 | P1 |
| ADM-008 | 异常 | 删除不存在用户 | 随机 id | 404/明确错误 | P1 |
| ADM-009 | 功能 | 审计日志 | `GET /admin/audit-log` | 200 关键操作有审计记录 | P1 |
| ADM-010 | 功能 | 系统状态 | `GET /admin/system/status` | 200 服务/数据延迟/任务状态 | P1 |
| ADM-011 | 功能 | 模型状态 | `GET /admin/models/status` | 200 三周期模型版本/准确率/健康 | P0 |
| ADM-012 | 功能 | 触发训练 | `POST /admin/training/run` | 202/200 训练异步启动，训练历史可见 | P0 |
| ADM-013 | 边界 | 训练大耗时不超时 | 全量训练 | 后端 HTTP 超时 7200s，训练期间其它接口可用 | P1 |
| ADM-014 | 功能 | 训练历史 | `GET /admin/training/history` | 200 历史记录含版本/准确率 | P1 |
| ADM-015 | 功能 | 训练调度 | `GET /admin/training/schedule` | 200 调度配置 | P2 |
| ADM-016 | 功能 | 模型回滚 | `POST /admin/training/rollback` 指定版本 | 200 回滚成功，模型状态版本更新 | P1 |
| ADM-017 | 异常 | 回滚非法版本 | 不存在版本 | 400/404 明确错误 | P1 |
| ADM-018 | 功能 | 读取模型参数 | `GET /admin/models/:id/params` | 200 参数对象 | P1 |
| ADM-019 | 功能 | 更新模型参数 | `PUT /admin/models/:id/params` 合法参数 | 200 合并后完整参数，持久化 | P1 |
| ADM-020 | 异常 | 参数越界 | 传超出约束范围的参数 | 400 明确错误 | P1 |
| ADM-021 | 功能 | 模型评估 | `POST /admin/models/:id/evaluate` | 200 返回 accuracy，versions.json 回写 | P0 |
| ADM-022 | 功能 | 模型预测 | `POST /admin/models/:id/predict` | 200 预测结果 | P1 |
| ADM-023 | 功能 | 数据源/健康/错误/延迟 | 对应 GET 端点 | 200 各返回结构正确 | P2 |
| ADM-024 | 交互 | 模型按钮独立 loading | 前端点某周期"训练" | 仅该行 loading 禁用，其它行不受影响，完成后自动恢复 | P0 |
| ADM-025 | 交互 | 管理表格渲染 | 前端 admin 页 | 用户/模型/训练表结构对齐，数值列右对齐 | P1 |

### Requirement: ML 服务模块（ml-service）测试用例

预测端点：`GET /api/v1/predictions/{symbol}`、`POST /api/v1/predictions/batch`、`POST /api/v1/predictions/run`、`GET /api/v1/predictions/history`
特征端点：`GET /api/v1/features/{symbol}`、`POST /api/v1/features/compute`、`GET /api/v1/features/history`
模型端点：`GET /api/v1/models/status`、`POST /api/v1/models/train/{period}`、`GET /api/v1/models/schedule`、`POST /api/v1/models/{period}/rollback`、`POST /api/v1/models/{period}/evaluate`、`GET/PUT /api/v1/models/{period}/params`、`POST /api/v1/models/evaluate`、`GET /api/v1/models/health`

| 编号 | 维度 | 标题 | 步骤(输入) | 预期结果 | 优先级 |
|---|---|---|---|---|---|
| ML-001 | 鉴权 | 缺 API Key | 不传 `X-ML-API-Key` | 401 | P0 |
| ML-002 | 鉴权 | 错误 API Key | 传错误 key | 401 | P0 |
| ML-003 | 功能 | 单只预测 | `GET /api/v1/predictions/600519.SH?period=short_term` | 200 预测结果（方向/置信度/目标价） | P0 |
| ML-004 | 功能 | 周期自动选择 | 不带 period | 按 short→medium→long 自动选择 | P1 |
| ML-005 | 异常 | 无预测记录 | 无模型的 symbol | 404 | P1 |
| ML-006 | 异常 | 非法 period | `?period=xyz` | 400（run 接口校验） | P1 |
| ML-007 | 功能 | 批量预测 | `POST /api/v1/predictions/batch` `{"symbols":[...]}` | 200 并发返回各股结果 | P1 |
| ML-008 | 功能 | 手动触发预测 | `POST /api/v1/predictions/run` `{"period":"short_term"}` | 202 accepted，异步执行 | P1 |
| ML-009 | 功能 | 特征计算 | `GET /api/v1/features/600519.SH` | 200 特征快照 | P1 |
| ML-010 | 功能 | 批量特征 | `POST /api/v1/features/compute` | 200 成功数量 | P1 |
| ML-011 | 功能 | 模型状态 | `GET /api/v1/models/status` | 200 三周期版本/健康 | P1 |
| ML-012 | 功能 | 模型训练成功 | `POST /api/v1/models/train/short_term` | 200 返回 new_version/accuracy | P0 |
| ML-013 | 异常 | 无真实数据拒训 | 清空真实数据后训练 | 404 `No real training data available; refusing to train on synthetic data` | P0 |
| ML-014 | 异常 | 数据不足拒训 | 数据量不足 | 404 `No enough real data to train {period}` | P1 |
| ML-015 | 异常 | 非法训练周期 | `POST /api/v1/models/train/xyz` | 400 | P1 |
| ML-016 | 功能 | 单周期评估 | `POST /api/v1/models/short_term/evaluate` | 200 返回 accuracy 并回写 versions.json | P0 |
| ML-017 | 功能 | 全量评估 | `POST /api/v1/models/evaluate` | 200 三周期结果数组 | P1 |
| ML-018 | 功能 | 参数读取 | `GET /api/v1/models/short_term/params` | 200 默认/当前参数 | P1 |
| ML-019 | 功能 | 参数更新 | `PUT /api/v1/models/short_term/params` 合法参数 | 200 合并结果并持久化 | P1 |
| ML-020 | 异常 | 参数越界 | 传越界参数 | 400 | P1 |
| ML-021 | 功能 | 模型回滚 | `POST /api/v1/models/short_term/rollback` `{"version":"v1.0.0"}` | 200 回滚成功 | P1 |
| ML-022 | 异常 | 回滚快照不存在 | 传不存在版本 | 404 `Snapshot not found` | P1 |
| ML-023 | 功能 | 模型健康检查 | `GET /api/v1/models/health` | 200 overall_healthy + recommendations | P1 |
| ML-024 | 功能 | 调度状态 | `GET /api/v1/models/schedule` | 200 jobs（未启动返回空数组） | P2 |
| ML-025 | 性能 | 训练不阻塞 | 长训练期间并发请求 status/health | 事件循环不阻塞，即时响应 | P1 |
| ML-026 | 数据 | 评估用最近 20% 验证集 | 触发评估 | 验证集为时序最近 20% 样本，无前视偏差 | P1 |
| ML-027 | 数据 | 准确率=三类方向命中率 | 查看评估结果 | accuracy 为 up/震荡/down 三分类命中率，基线 33% | P1 |

### Requirement: 决策支持仪表盘（dashboard）测试用例

端点：`GET /dashboard`

| 编号 | 维度 | 标题 | 步骤(输入) | 预期结果 | 优先级 |
|---|---|---|---|---|---|
| DASH-001 | 功能 | 聚合数据 | `GET /dashboard` | 200 高置信度标的/自选股/市场统计/实时行情 | P0 |
| DASH-002 | 衔接 | 高置信度过滤 | 构造置信度≥60 预测 | 高置信度卡片仅含达标标的 | P1 |
| DASH-003 | 阈值 | 低准确率不展示 | 某股 30 日准确率<45 | 该股不出现在高置信度推荐 | P1 |
| DASH-004 | 异常 | ML 不可用 | 停 ML 服务 | 预测区空态/错误态，其余区域正常 | P1 |
| DASH-005 | 交互 | 点击跳详情 | 点击高置信度卡片 | 跳转 `/stocks/{symbol}` | P1 |
| DASH-006 | 交互 | 定时刷新 | 停留页面 60s | dashboard 数据自动刷新 | P2 |
| DASH-007 | 交互 | 加载/空态 | 新用户无自选 | 空态提示，不白屏 | P1 |

### Requirement: 全流程贯通（E2E）测试用例

| 编号 | 维度 | 标题 | 步骤(输入) | 预期结果 | 优先级 |
|---|---|---|---|---|---|
| E2E-001 | 流程 | 新用户主链路 | 注册→登录→搜索"600519"→进入详情看实时行情/K线→生成预测→查看预测页→模拟交易启动→查看模拟账户 | 每步前置数据被下一步消费，全链路无断点 | P0 |
| E2E-002 | 流程 | 老用户回访 | 登录→自选股列表→看实时价→查看历史预测→查看评估 | 自选/历史/评估数据持续存在且正确 | P1 |
| E2E-003 | 流程 | 管理员运维链路 | admin 登录→后台看系统状态→查看模型状态→触发训练→查看训练历史→触发评估→查看准确率 | 全流程状态一致 | P0 |
| E2E-004 | 流程 | 行情→WebSocket→交易链路 | 打开行情页→实时价推送→下单买入→持仓/账户更新→模拟结算 | 价格、成交、资金一致 | P1 |
| E2E-005 | 流程 | 预测→评估→重训闭环 | 生成预测→每日评估→准确率回落→触发重训→版本更新 | 闭环状态机正确 | P1 |

### Requirement: 跨模块数据衔接（LINK）测试用例

| 编号 | 维度 | 标题 | 步骤(输入) | 预期结果 | 优先级 |
|---|---|---|---|---|---|
| LINK-001 | 衔接 | 行情三写一致 | 触发行情采集 | TSDB、Redis、WebSocket 三处数据一致 | P0 |
| LINK-002 | 衔接 | 预测持久化→评估回写 | 生成预测后触发评估 | 预测入库；评估基于真实行情回算并回写 versions.json；前端读取一致 | P0 |
| LINK-003 | 衔接 | 网关→ML 调用链 | 前端调预测接口 | 网关带 `X-ML-API-Key` 头调 ML；ML 不可用降级/超时可控 | P0 |
| LINK-004 | 衔接 | 股票同步→检索/行情 | 启动后端 stocks 为空 | 自动同步，检索与行情复用同一 stocks 数据 | P1 |
| LINK-005 | 衔接 | 模拟交易→决策→成交→持仓/资金 | 启动模拟交易并 trigger | 决策→成交→持仓/资金/历史全链路金额勾稽一致 | P0 |
| LINK-006 | 衔接 | 自选股→行情推送 | 添加自选后 | 自选列表收到实时推送 | P1 |
| LINK-007 | 衔接 | Redis 缓存一致性 | 行情更新后查实时接口 | 返回缓存/最新一致值，无脏读 | P1 |
| LINK-008 | 衔接 | Kafka 旁路 | 配置 KAFKA_BROKERS 时 | 采集数据可选投递 Kafka；未配置时 no-op 不阻塞 | P2 |
| LINK-009 | 衔接 | 24h 股票再同步 | 启动超 24h | stocks 表自动再同步（可配 STOCK_SYNC_INTERVAL_HOURS） | P2 |
| LINK-010 | 衔接 | 预测周期有效期 | 生成短期预测后 3 天 | 3 天后标记过期，前端显示"已过期" | P1 |
| LINK-011 | 衔接 | 模型参数→训练生效 | 更新参数后重训 | 新训练使用新参数，versions.json 记录 | P1 |

---

## REMOVED Requirements
无（纯新增测试文档，无移除需求）
