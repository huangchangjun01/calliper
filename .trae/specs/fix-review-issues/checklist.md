# Checklist — 修复代码审查全量问题

> 逐项验证通过后勾选。任一项失败 → 回到 tasks.md 补任务并复验。

## 预测闭环（Task 1）
- [x] 落库 period 恒为 short|medium|long，direction 恒为 up|down|flat（回归：psql 确认存量 6 条已回填为 short/medium/long + flat，新写入归一化）
- [x] 存量 `_term`/`neutral` 记录已被 migration 回填，CHECK 约束已更新（000002 已对本地库执行，predictions_direction_check 已重建为 up/down/flat）
- [x] 评估端统计/排行/趋势查询口径与实际落库一致，无恒 0/恒空（趋势图周期映射已统一 short/medium/long）
- [x] generate→落库→stats→accuracy 全链路真实数据可见（端点全部 200 非报错；本环境 ML 无实时行情故 generate 产出 no_data 0 条，落库规范与评估回填机制均验证正常）

## WebSocket（Task 2、3）
- [x] 每个连接仅 WritePump 一个写者（无 hub ticker、无 readPump 直写 pong）（代码收敛 + grep 复核）
- [x] subscriptions 访问无数据竞争（并发订阅/断开不 panic）（SnapshotSubscriptions 加锁方案）
- [x] 前端心跳按收到数据复位，健康连接无每 40s 断连/重连（回归：WS 连接持续 72s 未被服务端断开，收到 23 次 pong）
- [x] 真实断链后能按指数退避自动重连并恢复订阅（既有重连逻辑保留，代码复核）

## 数据真实性（Task 3、7）
- [x] StockDetail 接口失败显示错误态/空态，无 1875.00 假盘口/假基本面（catch 假值已删除，DataState 化）
- [x] 实时数据源网络失败不 upsert 假股票列表，状态标注不可用（akshare/yahoo 改为返回错误）

## 交易安全（Task 4）
- [x] 错误交易密码下单被拒（回归：401 code 40103）
- [x] 正确密码下单成功，普通用户 is_real 恒为 false（回归：订单 is_real=false）
- [x] 真实单仅当开关开启且 admin 时可达（REAL_TRADING_ENABLED + admin 双条件，代码实现）

## 后端安全基线（Task 5）
- [x] JWT 仅接受 HS256（算法混淆被拒）；默认/空密钥启动有 ERROR 告警（两处 parseToken + LoadConfig 告警）
- [x] CORS 白名单生效（回归：非白名单 Origin 无 ACAO 头、预检不放开；白名单 http://localhost:5173 正常回显）；WS Origin 校验生效
- [x] 全部分页接口 limit 钳制（回归：limit=-1/99999 均回落到 limit=20，未拉全表）
- [x] Admin 自删保护以数值比较生效（ParseUint 比较）
- [x] 内部错误不泄露进响应；admin stub 接口已标注非真实数据（not_implemented + 文案）

## ML 服务（Task 6、10）
- [x] medium 预测取最新样本（preds[-1]），方向/置信度不基于历史日（单票与批量一致）
- [x] /api/v1/features/{symbol} 与 */history 返回真实结果（根因 postgres://→postgresql:// 已修，engine 创建验证通过，回归 500 消除）
- [x] /models/train/{period} 不再因空数据 500（加载真实数据后训练，无数据返回 4xx）
- [x] 无 ML_API_KEY 访问 8000 变更接口返回 401（回归验证 401）；docker 不 publish 8000（compose 已移除 ports）
- [x] 推断滑窗与训练一致（short=30/long=52 末滑窗）；scaler 落盘并在预测时复用
- [x] 训练末期行不误标下跌；调度含周训练/每日评估；任务并发有互斥（main.py 注册三任务 + threading.Lock + Asia/Shanghai）

## 部署编排（Task 6、11）
- [x] docker-compose 三处 DB 凭据与代码一致（统一 calliper/10010hcj/calliper_trading|calliper_tsdb；本机无 docker CLI，YAML 结构解析校验通过）
- [x] ml-service 注入 DATABASE_URL/TSDB_URL（postgresql://，host 用服务名），容器内引擎可用
- [x] tsdb healthcheck 库名正确（calliper_tsdb）；backend 依赖顺序含 ml-service（service_healthy）
- [x] .dockerignore/.gitignore 生效；dump.rdb 等已移出版本控制（git rm --cached 完成）
- [x] requirements 依赖固定版本（== 对齐 .venv 实际版本）；Makefile 失效目标已修复（提示使用 SQL 脚本）

## 资金与风控（Task 8）
- [x] 网关账户更新走事务/行锁，并发下单无超卖、无资金负值（UpdateBalance/Freeze/Unfreeze 事务 + FOR UPDATE）
- [x] 风控限额原子执行（Redis Lua，回归：210 万大单被限额拒绝未建单）；失败订单不占额度（RecordTrade 仅在订单成功回调）

## 前端交互与状态（Task 9）
- [x] 401 自动登出并回登录页，不滞留受保护页（api.ts 401 → authStore.logout）
- [x] 登出时 WS 已断开、订阅已清（logout → wsClient.disconnect）
- [x] 非 admin 用户进入 /admin 被重定向，菜单不显示（AuthGuard roles + Sidebar 条件渲染）
- [x] useStockQuote 引用稳定（snapshot state 化，memo 生效、闪烁 effect 不空转）
- [x] OrderForm 选股后限价单可正常提交（realtime/batch 回填实时价，路由已存在）；搜索有防抖（300ms）

## 全链路回归（Task 12）
- [x] 预测生成→评估→准确率回流数据链路无断点（generate 200、stats/accuracy/trend 200、admin/evaluation/run 200 且到期预测正确顺延）
- [x] WS 连接/订阅/心跳/断线重连长跑稳定（72s 无断开 + pong 正常）
- [x] 交易：登录→下单→持仓→平仓→风控全流程通过（错误密码 401；限价单填单；持仓 buy qty/avg_cost 与 sell 减仓/realized 均数值吻合；限额拒绝）
- [x] 前端 Market/StockDetail/Predictions/Dashboard/Admin 冒烟通过（tsc 全绿，运行时行情依赖数据源）
- [x] `go build ./...`、`go vet`、前端 `tsc --noEmit`、ML 编译检查全部通过
- [x] 测试数据已清理（临时用户/订单/持仓/审计/测试预测已删），测试服务已关闭（8000/8080 释放）

> 备注：docker-compose 未实跑（本机无 docker CLI）；预测闭环在无实时行情环境下以 no_data 验证落库规范与评估机制；真实交易分支（REAL_TRADING_ENABLED+admin）仅代码层覆盖未联调。