# Tasks — 修复代码审查全量问题

> 阶段 1（Task 1-6）为 blocker 级：业务闭环 + 安全基线，优先完成。
> 阶段 2（Task 7-11）为健壮性/一致性。
> 阶段 3（Task 12）全链路回归，依赖前两阶段。

## 阶段 1：业务闭环与安全基线（P0）

### Task 1: 预测标签规范统一（Go + SQL）
- [x] `prediction_service.go` `PersistPredictions`：落库前把 period 归一化为 `short|medium|long`（空默认 `short`，`short_term→short`、`medium_term→medium`、`long_term→long`）；`normalizeDirection` 维持输出 `up|down|flat` 不变
- [x] `evaluation_service.go` 趋势图查询（~L1259-1266）周期映射改为 `short|medium|long`，与统计/排行口径一致
- [x] 新增 `backend/migrations/000002_unify_prediction_labels.up.sql`：`UPDATE predictions SET period=...` 回填 `_term→short|medium|long`、`direction 的 neutral→flat`（flat 已合法）；`ALTER TABLE predictions DROP CONSTRAINT + ADD CONSTRAINT`，方向 CHECK 改为 `('up','down','flat')`、周期保持 `('short','medium','long')`；对应的 `.down.sql` 回滚
- [x] 全链路 grep 确认：写库处无 `_term` 残留；评估端查询口径一致
- [x] 验证：`go build ./...`；手动触发 generate 落库值符合 CHECK；`/predictions/stats、/accuracy、/accuracy/trend、/accuracy/ranking` 返回非空真实数据（回归阶段复核）

### Task 2: WebSocket 单写者改造（Go）
- [x] `websocket/hub.go`：删除 `Hub.Run` 中 ticker 心跳分支（Ping 职责归 `Client.WritePump`）
- [x] `handlers/ws_handler.go` readPump：收到 `ping` 时改为向 `client.send` 投递 pong 消息（非直写 conn）；确认应用层心跳协议与前端匹配
- [x] `hub.go` register/unregister 分支遍历 `client.subscriptions` 时加 `client.mu.Lock()`（消除与 Subscribe 的 append 数据竞争）
- [x] 确认 `Client.WritePump` 单一写者不变，所有直写 conn 点收敛（grep conn.Write 复核）
- [x] 验证：并发订阅/退订/断开压测无 panic、无数据竞争（`go build -race` 冒烟）；前端连接长跑不被异常断开（回归阶段复核）

### Task 3: 前端 WS 心跳复位 + 详情页假数据移除（前端）
- [x] `services/websocket.ts`：`onmessage`（含 heartbeat 分支）调用 `resetPingTimer()`（收到任何数据即视为存活）；40s 定时器仅兜底死链
- [x] `pages/StockDetail.tsx`：删除盘口/基本面 catch 中的写死假数据，改走空态 + 错误重试（复用 DataState 模式）；恢复数据时正常渲染
- [x] 验证：浏览器长跑无每 40s 断连重连；详情页接口失败显示错误态；`tsc --noEmit` 通过

### Task 4: 交易下单安全（Go）
- [x] `handlers/trade_handler.go`：`TradePassword` 必须与用户账户密码一致（bcrypt 比对，复用现有比对能力）；错误密码返回 401/400
- [x] `is_real` 处理：忽略请求体值，普通用户固定 `false`；仅当 `REAL_TRADING_ENABLED=true` 且用户角色 admin 时才允许真实单（服务端判定，不信任请求）
- [x] `models.go` User 若需记录交易密码则加字段 + migration（先评估：复用登录密码即可，优先不加字段）。若不加字段，删除前端"交易密码"单字段与登录密码混淆之处（检查 OrderForm）
- [x] 验证：错误密码下单被拒；正确密码下单成功且 is_real=false；`go build ./...`（回归阶段复核）

### Task 5: 后端安全基线（Go）
- [x] `middleware/auth.go` + `handlers/ws_handler.go` 两处 `parseToken`：keyfunc 断言 `token.Method` 为 HS256，否则拒绝
- [x] `middleware/cors.go` + `main.go`：CORS 白名单化（env `CORS_ALLOWED_ORIGINS`，默认含本地前端源）；消除 `*`+credentials 组合；WS 握手 `CheckOrigin` 校验 Origin 属白名单
- [x] `config/config.go`：`JWT_SECRET` 为空或等于公开默认值时打印 ERROR 告警（并在文档标注生产必改）；DB/TSDB/MinIO 默认弱密码打 WARN 告警
- [x] 分页钳制：新增统一 `parsePageLimit` 辅助（limit∈[1,100]、offset≥0，非法回落默认值），应用到 stock/admin/prediction/sim_trade/trade/evaluation 各 handler
- [x] `admin_handler.go` 自删保护改为数值比较（转 uint）后再判断
- [x] 敏感错误脱敏：auth.go:129、market_handler 等处不再拼接内部错误进响应（只记日志，对外统一文案）
- [x] admin stub 接口（GetDataSources/GetServiceHealth/GetErrorLogs/GetDataLatency/GetModels）标注"未实现/采样中"，移除"healthy/虚构指标"误报
- [x] 验证：默认 JWT 启动告警；跨域预检仅白名单放行；HS256 伪造 alg 拒绝；limit=-1 被钳制；`go build ./...`

### Task 6: ML 部署与接口修复（Python + infra）
- **ML Python**
- [x] `app/api/predictions.py`：medium_term 预测取 `preds[-1]`（最新样本），单票与批量一致
- [x] `app/features/feature_pipeline.py`：新增 `compute_features(symbol)` 与 `get_feature_history(...)`（组合 build_features/preprocess 与 FeatureStore 读取），供 `app/api/features.py` 调用，消除 AttributeError/空返回
- [x] `app/api/models.py` `/train/{period}`：加载真实数据（复用 train_models DataLoader 逻辑）再调用 `train_single(df=...)`，不再因空数据 500；或改为启动异步训练并返回任务状态
- [x] ML 鉴权：`app/main.py` 增加 `ML_API_KEY` 校验依赖（FastAPI Depends），作用于变更类接口；CORS 收紧为白名单；backend `prediction_service.go` 调用时携带密钥头；docker-compose 8000 不再 publish 宿主机
- **Infra**
- [x] `docker-compose.yml`：postgres/timescaledb 初始化统一为 `calliper/10010hcj/calliper_trading|calliper_tsdb`；tsdb healthcheck 库名改为 `calliper_tsdb`；ml-service 注入 `DATABASE_URL`（postgres://calliper:10010hcj@db:5432/calliper_trading）与 `TSDB_URL`；backend 依赖 ml-service 的健康检查/顺序；`.env.example` 同步
- [x] 验证：`docker-compose config` 校验通过（本机无 docker CLI，改用 YAML 结构解析校验 ✅）；后端容器能以统一凭据连库；ml-service 容器内引擎非 None；无密钥访问 8000 返回 401；`/api/v1/features/{symbol}` 与 `/models/train` 状态正常（回归阶段复核）

## 阶段 2：健壮性与一致性（P1）

### Task 7: 后端服务健壮性（Go）
- [x] `history_backfill.go`：改用独立后台 context（派生自 background，`context.WithCancel`），仅显式 Stop 取消；入口不再透传请求 ctx
- [x] `mock_broker.go`：`rng` 加 `sync.Mutex`（或改 `math/rand/v2` 并发安全）
- [x] `tsdb_persist.go`：symbol→stock_id 改用内存缓存 map（启动/定期刷新 + 失重回查），消除逐条 N+1
- [x] `stock_service.go`：find-or-create 改批量 upsert（`clause.OnConflict` + `CreateInBatches`）；`akshare_client.go` 新浪翻页拉全量（不再固定 page=1 num=200）
- [x] `market_data_service.go`：采集合集改为全量/订阅驱动（上限保护）；循环应用 `isTradingHours` 交易时段节流；HK/US 未注册时显式报错不静默 failed
- [x] `akshare_client.go`/`yahoo_client.go`：实时源网络失败返回错误 + 数据不可用标记，不再 upsert `defaultStockList` 假数据（K 线/初始化 fallback 保留但标注来源）
- [x] 验证：`go build ./...`；回填时断开请求仍继续完成；采集循环盘外不空转；stocks 同步数量覆盖全表

### Task 8: 资金与风控原子性（Go）
- [x] `account_service.go`：`UpdateBalance/FreezeFunds/UnfreezeFunds` 改为事务 + `SELECT ... FOR UPDATE` 行锁（或原子表达式更新）
- [x] `sim_trade_service.go`：下单流程（校验→扣款/释放→持仓 upsert→流水）单事务化，移除"冻结再解冻"冗余往返
- [x] `risk_manager.go`：`CheckDailyLimit`+`RecordTrade` 合并为 Redis Lua 脚本原子执行（含单笔/单日限额、失败订单不计额）
- [x] 验证：并发下单无超卖/资金负值；风控限额并发不突破；`go test`/压测冒烟

### Task 9: 前端交互与状态修复（前端）
- [x] `services/api.ts`：401 分支同步清空 authStore（token/user/isAuthenticated）并跳转登录页（复用 logout 逻辑，避免无限重试 401）
- [x] `stores/authStore.ts` logout：调用 WS 单例 `disconnect()` 并清空订阅
- [x] 路由守卫：`AuthGuard`/`App.tsx` 增加 `role==='admin'` 校验；`Layout/Sidebar.tsx` 管理菜单按角色渲染；AdminPanel 非 admin 重定向
- [x] `hooks/useStockQuote.ts`：暴露稳定 state 快照（批量 flush 后 setState 新 Map）替代每次渲染新建的 getter，修复 Market/Dashboard/StockSearch 的 memo 失效与闪烁 effect
- [x] `components/OrderForm`：选股后按需获取实时价回填（或明确留空强制手输），使限价单可提交；股票搜索加 300ms 防抖
- [x] 轮询优化：TradingPanel/Dashboard 高频 REST 轮询与 WS 重叠部分降频或仅断线兜底（保持后端压力可控）
- [x] 清理 `stores/marketStore.ts` 死代码
- [x] 验证：token 过期自动回登录页；登出后 WS 已断；普通用户进 /admin 被重定向；限价单可提交；`tsc --noEmit` 通过

### Task 10: ML 数据链路一致性（Python）
- [x] 推断与训练口径一致：`prediction_task.py` 按 short=30 / long=52（训练滑窗）切最后滑窗输入，消除 train/serve skew（medium 依据自身模型口径处理）
- [x] `StandardScaler` 持久化：训练拟合后随模型权重落盘，预测链路加载复用（接入 `feature_pipeline._standardize` 或预测管线）
- [x] `train_models.py`：末期无未来样本的行不再标 0（下跌），按 pending/剔除处理（medium concat 段同样处理）
- [x] `app/tasks/scheduler.py` + `app/main.py`：注册周训练与每日评估任务（当前仅日预测）；预测任务加互斥锁（定时与 `/run` 不并发）；交易日/时区统一 Asia/Shanghai
- [x] 验证：train→infer 序列口径一致；重启后 scaler 可复用；调度任务列表包含三任务；并发触发 `/run` 不重复执行

### Task 11: 基础设施清理（infra）
- [x] 各服务新增 `.dockerignore`（排除 `.venv`、`.ml-models`、`mlflow.db`、`*.env*`、`dump.rdb`、`.git`）
- [x] `.gitignore` 补全（`.venv`、`.ml-models/`、`dump.rdb`、`mlflow.db`、`versions.json`、`.env.local` 等）；`git rm --cached` 已追踪的 `dump.rdb`（及模型制品若被追踪）
- [x] `ml-service/requirements.txt`：依赖固定版本（`==`），保留可复现性
- [x] `Makefile`：`db-migrate/db-reset` 指向真实路径（`cmd/migrate` 不存在则修正或删除目标并说明使用 SQL 脚本）
- [x] 验证：`git status` 干净；docker build 镜像体积下降（.venv/.ml-models 不入镜像）

## 阶段 3：全链路回归（P2）

### Task 12: 全链路回归测试（端到端）
- [x] 环境：启动 ML(8000，venv)、后端(8080)；（本机无 docker CLI，compose 以 YAML 结构校验）
- [x] 预测闭环：`POST /predictions/generate` → 落库记录 period/direction 符合规范（无行情时按设计 no_data）→ `/predictions/stats|accuracy|accuracy/trend` 200 → EvaluateDaily 到期预测正确顺延回填
- [x] WS 链路：连接 → 订阅 → 心跳/数据持续（72s 无断开、pong 正常）→ 断线重连逻辑复核
- [x] 交易链路：登录 → 错误密码 401 被拒 → 正确密码（模拟）下单 → 持仓/账户更新数值吻合 → 平仓 → 风控限额拒绝生效
- [x] 前端冒烟：构建通过（tsc），行情数据依赖数据源运行时
- [x] 构建门禁：`go build ./...`、`go vet`、前端 `tsc --noEmit`、ML 编译检查全部通过
- [x] 清理：测试数据（临时用户/订单/持仓/审计/测试预测）清理；ML 与后端测试服务已关闭（8000/8080 释放）

# Task Dependencies
- Task 1-5 仅涉及 backend，多文件修改存在重叠（尤其是 evaluation_service / websocket），**一个实现子代理内串行完成**
- Task 3、9 仅前端，Task 10 仅 ML，Task 11 仅 infra，可与 Task 1-5 并行
- Task 6 跨 ML+infra，可与 Task 1-5 并行（不重叠文件）
- Task 7、8 属于 backend 服务层，在 Task 1-5 完成后执行（或同一子代理内后续批次）
- Task 12 依赖 Task 1-11 全部完成

# 可并行执行的任务组（实现阶段）
- 组 A：Task 1 + 2 + 4 + 5 + 7 + 8（backend，同一子代理串行，避免文件冲突）
- 组 B：Task 3 + 9（frontend，同一子代理）
- 组 C：Task 6(ML 部分) + 10（ml-service，同一子代理）
- 组 D：Task 6(infra 部分) + 11（infra，同一子代理）
- 组 E：Task 12（全链路回归，最后，主线程统筹或独立子代理）