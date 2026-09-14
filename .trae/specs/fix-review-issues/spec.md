# 修复代码审查全量问题 Spec

## Why

全仓库代码审查发现一批会直接破坏业务闭环、误导交易决策或构成安全/运维隐患的问题：
预测落库标签与 DB CHECK、评估查询三套约定不一致（预测可能静默落库失败、准确率恒 0）；WebSocket 三处并发写同一 conn（帧损坏/连接被误断）+ 前端心跳假死检测永不复位（每 40s 强制断连）；个股详情页失败时展示伪造盘口/基本面；交易密码形同虚设；凭据明文硬编码、CORS 通配带凭证、ML 服务 0 鉴权裸奔；docker 编排凭据三处不一致。本 Spec 一次性修复全部 review 问题并做全链路回归。

## What Changes

- **预测数据链路**：统一标签规范（落库 `period∈{short,medium,long}`、`direction∈{up,down,flat}`），写入端归一化、评估端查询对齐、DB CHECK 约束与存量数据通过新 migration 对齐
- **WebSocket 生命周期**：单写者模型（心跳仅由 WritePump 发出、pong 走 send 队列、删除 hub ticker 重复 Ping、subscriptions 并发访问加锁）；前端心跳按"收到数据即复位"存活判定
- **数据真实性**：移除个股详情页失败时的伪造盘口/基本面，改走空态+重试；数据源网络失败不再回落并写入硬编码假股票列表
- **安全基线**：JWT 算法固定 HS256 断言、CORS/WS Origin 白名单化、JWT 默认密钥启动告警、交易密码真实校验（bcrypt 与账户密码一致）、`is_real` 由服务端控制（模拟平台固定 false）、ML 服务 API Key 服务间鉴权
- **部署编排**：docker-compose 数据库凭据与代码统一、ml-service 注入 `DATABASE_URL`/`TSDB_URL`、healthcheck 库名修正、`.dockerignore`/`.gitignore` 补全、requirements 锁版本、Makefile 失效目标修复
- **正确性与健壮性**：历史回填改独立后台 context、MockBroker rand 加锁、TSDB symbol→id N+1 缓存化、股票同步批量 upsert+翻页、采集合集由订阅/全量驱动并应用交易时段节流、模拟交易资金/持仓链路事务化并移除冗余冻结、风控限额原子化（Redis Lua）、评估口径修复（AVG+LIMIT 子查询、行业归因语义、基准股主库解析）、分页参数钳制、接口错误脱敏、401 同步登出态、角色级路由守卫、登出断开 WS、useStockQuote 稳定引用、OrderForm 选股价格回填与防抖
- **ML 数据链路**：medium 预测取用最新样本（`preds[-1]`）、推断滑窗与训练一致、StandardScaler 落盘并接入预测、`/api/v1/features/*` 补齐缺失方法、`/models/train` 加载真实数据、末期标签不再误标、调度器注册周训练/每日评估、预测任务互斥+时区统一

## Impact

- Affected specs：全部。后端 API/中间件/服务层/WebSocket/模型/配置/迁移、前端页面/组件/hooks/services/stores、ML 服务 api/features/models/tasks、基础设施（docker-compose、Makefile、Dockerfile、.gitignore）
- Affected code：
  - backend：`prediction_service.go`、`evaluation_service.go`、`websocket/hub.go|client.go`、`handlers/ws_handler.go|trade_handler.go|*_handler.go`、`middleware/cors.go|auth.go`、`config/config.go`、`services/history_backfill.go|mock_broker.go|tsdb_persist.go|stock_service.go|market_data_service.go|account_service.go|sim_trade_service.go|risk_manager.go|akshare_client.go|yahoo_client.go|audit_service.go`、`migrations/000002_*.sql`（新增）
  - frontend：`services/websocket.ts|api.ts`、`stores/authStore.ts`、`pages/StockDetail.tsx|Login.tsx|AdminPanel.tsx`、`components/AuthGuard.tsx|Layout/*|OrderForm/*`、`hooks/useStockQuote.ts`、`types/index.ts`（如需）
  - ml-service：`app/api/features.py|models.py|predictions.py`、`app/main.py`、`app/models/model_manager.py|medium_term_model.py`、`app/tasks/prediction_task.py|scheduler.py`、`app/features/feature_pipeline.py`、`train_models.py`、`requirements.txt`
  - 根目录：`docker-compose.yml`、`.gitignore`、`.dockerignore`（新增）、`Makefile`

## 关键设计决策

1. **标签规范统一为 `short/medium/long` + `up/down/flat`**（应用层与前端既有约定）：写入端把 ML 返回的 `_term` 归一化后落库；`normalizeDirection` 已产出 `flat` 保持不变；新增 migration 把 DB CHECK 从 `('up','down','neutral')` 改为 `('up','down','flat')` 并回填存量 `_term`→`short|medium|long`、`neutral`→`flat`；评估端周期查询（含趋势图）统一映射。
2. **WS 单写者**：`Client.WritePump` 已每 30s 发 Ping；移除 `Hub.Run` 的 ticker 分支（冗余第二写者）；`ws_handler.readPump` 收到前端 ping 不再直写 conn，改向 `client.send` 投递 pong 消息由 WritePump 写出；`Hub.Run` 的 register 分支遍历 `subscriptions` 时加 `client.mu`。
3. **交易密码**：User 模型无独立交易密码字段 → 下单时 `TradePassword` 必须与账户登录密码（bcrypt）一致；`is_real` 不信任请求体，服务端固定 `false`（仅当环境变量开启真实盘开关且角色为 admin 时透传），保持与 MockBroker 实现一致。
4. **docker 凭据**：统一为 `calliper / 10010hcj / calliper_trading | calliper_tsdb`（与本地开发约定一致），docker-compose 初始化值、healthcheck、`.env.example` 三处对齐；ml-service 注入 `DATABASE_URL`/`TSDB_URL`（主机名 `db`/`tsdb`）。
5. **ML 服务鉴权**：共享密钥 `ML_API_KEY`（服务间认证）：backend `prediction_service` 请求头携带；ML FastAPI 全局依赖校验；8000 端口不再 publish 到宿主机。
6. **前端 WS 心跳**：`onmessage`（含 heartbeat 分支）收到任何数据即 `resetPingTimer()`，40s 定时器仅作死链兜底。
7. **采集驱动**：采集 symbol 集合改为全量 stocks（有界限额 + 分批），循环内应用 `isTradingHours` 交易时段与节假日节流；HK/US 收集器未实现前显式标记不可用并返回明确错误（而非静默 failed）。
8. **资金原子性**：买入/卖出整链路放入单事务（校验→扣款→持仓→流水），`SELECT ... FOR UPDATE` 锁账户行；`FreezeFunds/UnfreezeFunds` 保留为快照展示（入库逻辑不再依赖冻结-解冻往返）。

## ADDED Requirements

### Requirement: 预测标签规范统一
系统 SHALL 以 `period∈{short, medium, long}`、`direction∈{up, down, flat}` 作为预测记录的落库、查询与展示规范。

#### Scenario: 生成预测落库
- **WHEN** ML 返回 `short_term`/`up` 等标签
- **THEN** 落库 `period=short`、`direction=up`，且不违反 DB CHECK 约束

#### Scenario: 存量脏数据
- **WHEN** 数据库存在 `short_term`/`neutral` 历史记录
- **THEN** 新 migration 将存量回填为 `short|medium|long` / `flat` 并更新 CHECK 约束，未来写入违规值被拒绝

#### Scenario: 准确率统计
- **WHEN** 查询近 7/30/累计准确率、周期准确率、排行、趋势
- **THEN** 统计口径与实际落库值一致（不再因标签错位产生恒 0/恒空）

### Requirement: WebSocket 单写者
每个连接 SHALL 至多一个 goroutine 写 conn（WritePump），所有控制/业务帧（Ping、Pong、数据）SHALL 经由该 goroutine 输出。

#### Scenario: 前端 ping
- **WHEN** 客户端发送 ping
- **THEN** 服务端将 pong 投递到该连接的发送队列（不直写 conn），连接维持

#### Scenario: 心跳探测
- **WHEN** 服务端存活探测周期到达
- **THEN** 仅由 WritePump 发出 Ping，无任何并行写者

#### Scenario: 并发订阅/断开
- **WHEN** 多个 goroutine 同时 subscribe/unsubscribe/register/unregister
- **THEN** subscriptions 访问无数据竞争，进程不 panic

### Requirement: 前端 WS 心跳存活判定
前端 SHALL 以"收到任何服务端数据"作为连接存活判定，40s 定时器仅作死链强制回收。

#### Scenario: 健康长连接
- **WHEN** 连接持续接收行情/心跳响应
- **THEN** 定时器持续复位，连接不被误断、不触发无谓重连

### Requirement: 详情页数据真实性
个股详情页 SHALL 在接口失败时展示加载失败/空态并提供重试，SHALL NOT 展示写死的假盘口或假基本面。

#### Scenario: 深度接口失败
- **WHEN** `/market/depth/{symbol}` 失败
- **THEN** 页面展示错误态与重试按钮，不展示 1875.00 等固定档位

### Requirement: 交易下单校验
下真实/模拟订单时，服务端 SHALL 校验 `trade_password` 与用户账户密码一致（bcrypt）；`is_real` SHALL 由服务端决定而非请求体。

#### Scenario: 错误密码
- **WHEN** 用户提交错误交易密码
- **THEN** 下单被拒（401/400），不产生订单记录

#### Scenario: 真实单控制
- **WHEN** 普通请求携带 `is_real=true`
- **THEN** 服务端按模拟盘处理（固定为 false），除非真实盘开关开启且用户为 admin

### Requirement: ML 服务鉴权
ML 服务 SHALL 对 `/train`、`/run`、`/evaluate`、`/api/v1/models/*` 等变更类接口要求 `ml-api-key`（或 `Authorization`）头且值匹配 `ML_API_KEY`，缺失或错误返回 401；服务间调用由 backend 统一携带。

#### Scenario: 无密钥访问
- **WHEN** 直接访问 8000 端口且不带密钥
- **THEN** 返回 401，无法触发训练/预测

## MODIFIED Requirements

### Requirement: 数据源失败降级
原：网络失败回落硬编码假股票列表并 upsert。
改为：实时数据源网络失败返回错误并标记数据不可用（不写假数据）；K 线/初始化场景允许显式 fallback 但必须标注来源。

### Requirement: CORS 与 WS Origin
原：`["*"]` + 回显 origin + credentials。
改为：仅允许配置白名单来源（默认含本地前端地址）；WS 升级校验 Origin 属白名单。

### Requirement: JWT 校验
原：仅验签名不验算法，默认密钥公开。
改为：keyfunc 断言 HS256；`JWT_SECRET` 为空或等于公开默认值时启动打印 ERROR 告警并要求显式配置。

### Requirement: 回填任务生命周期
原：回填透传 HTTP 请求 context，请求取消中断批量任务。
改为：回填派生独立后台 context，仅显式 Stop 时取消。

### Requirement: 前端 401 会话同步
原：401 仅清理 localStorage，用户滞留受保护页。
改为：401 时同步清空 authStore 并跳转登录页。

### Requirement: 前端登出断链
原：登出不关闭 WS 单例，旧 token 连接残留。
改为：logout 时调用 WS disconnect 并清空订阅。

### Requirement: 前端路由守卫
原：任何登录用户可进入 /admin。
改为：`role=admin` 才可进入管理后台，Sidebar 菜单按角色渲染。

### Requirement: ML 预测样本选取
原：medium_term 取 `preds[0]`（历史样本）。
改为：取最新样本 `preds[-1]`（或仅对末根 K 线预测）。

### Requirement: 评估口径
原：`AVG(volume)`+`LIMIT(20)` 无效、行业归因查个股自身、基准股依赖 tsdb 内 stocks。
改为：近 20 日量均值用子查询；行业归因校正为行业对照或改名个股异动；基准股先主库解析 stock_id。

## REMOVED Requirements

### Requirement: Hub 层独立 Ping（重复写者）
**Reason**：`Client.WritePump` 已负责周期 Ping，Hub ticker 构成第二个写者违反单写者约束。
**Migration**：删除 `Hub.Run` ticker 分支，心跳职责归 WritePump。

### Requirement: 数据源假数据兜底（部分接口）
**Reason**：真实网络失败时向 stocks 表写入默认股票列表会掩盖故障、污染数据来源。
**Migration**：失败返回错误 + 状态标注；移除 `defaultStockList` 在实时源的应用。

### Requirement: 无鉴权 ML 接口（BREAKING）
**Reason**：训练/预测接口裸露可被任意触发。
**Migration**：全量接口要求 `ML_API_KEY`；backend internal 调用方同步更新。