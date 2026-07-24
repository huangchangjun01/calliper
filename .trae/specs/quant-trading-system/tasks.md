# Tasks

## 阶段一：项目基础设施搭建

- [x] Task 1: 项目初始化与 Docker 开发环境搭建
  - [x] 创建 monorepo 项目结构（frontend/、backend/、ml-service/）
  - [x] 编写 Docker Compose 配置（PostgreSQL+TimescaleDB、Redis、Kafka、MinIO）
  - [x] 创建各服务的 Dockerfile 和基础配置
  - [x] 编写 Makefile 统一管理开发命令

- [x] Task 2: 数据库 Schema 设计与初始化
  - [x] 设计并创建 PostgreSQL 表结构（stocks、markets、users、orders、positions、simulated_trades、predictions、prediction_accuracy）
  - [x] 创建 TimescaleDB 超表（stock_prices_1min、stock_prices_daily、stock_prices_tick）
  - [x] 编写数据库迁移脚本（golang-migrate）
  - [x] 创建种子数据脚本（市场定义、交易所信息）

- [x] Task 3: API Gateway 搭建
  - [x] 初始化 Go 项目，集成 Gin 框架
  - [x] 实现 JWT 鉴权中间件
  - [x] 实现请求限流中间件
  - [x] 实现 WebSocket 连接管理（心跳、断线重连、频道订阅）
  - [x] 实现 CORS 和静态资源代理

## 阶段二：数据采集与股票检索

- [x] Task 4: 全球股票代码检索服务
  - [x] 实现多市场股票代码同步模块（AKShare + Yahoo Finance）
  - [x] 实现模糊搜索和精确匹配 API
  - [x] 实现按市场分类查询 API
  - [x] 实现股票代码缓存策略（Redis 24h TTL）
  - [x] 实现数据源健康检查与降级逻辑

- [x] Task 5: 实时行情数据采集服务
  - [x] 实现行情数据采集器（AKShare A股、Yahoo Finance 海外）
  - [x] 实现数据清洗管道（去重、缺失值填充、异常值检测、复权处理）
  - [x] 实现 Kafka Producer 写入行情数据流
  - [x] 实现 Kafka Consumer 写入 TimescaleDB
  - [x] 实现历史数据回填任务（5年日线 + 1年分钟线）

- [x] Task 6: 实时行情推送服务
  - [x] 实现 Redis 实时行情缓存更新
  - [x] 实现 WebSocket 行情推送（按股票代码订阅频道）
  - [x] 实现行情数据序列化优化（JSON）
  - [x] 实现多股票并发推送管理

## 阶段三：前端 Dashboard

- [x] Task 7: 前端项目初始化与布局
  - [x] 初始化 React 18 + TypeScript + Vite 项目
  - [x] 实现全局布局组件（Header、Sidebar、Main Content）
  - [x] 实现主题切换（明/暗）
  - [x] 实现路由配置（Dashboard、Stock Search、Trading、Predictions、Admin）

- [x] Task 8: 股票检索页面
  - [x] 实现市场分类筛选组件（Tab 切换 A股/港股/美股/日股/欧股/其他）
  - [x] 实现股票搜索框（模糊搜索 + 搜索结果下拉）
  - [x] 实现股票列表展示（代码、名称、行业、市值、最新价、涨跌幅）
  - [x] 实现虚拟滚动优化大数据量渲染

- [x] Task 9: 实时行情 Dashboard
  - [x] 实现 WebSocket 客户端连接管理（自动重连、心跳）
  - [x] 实现自选股实时行情面板（价格跳动高亮、涨跌红绿动画）
  - [x] 实现个股详情弹窗（分时图、K线图、盘口深度、基本信息）
  - [x] 实现 ECharts 分时图和 K 线图组件
  - [x] 实现市场概览面板（指数、涨跌统计、成交额）

- [x] Task 10: 预测结果展示页面
  - [x] 实现预测概览面板（短期/中短期/长期预测方向分布）
  - [x] 实现个股预测详情（预测方向、置信度、目标价位、关键因子）
  - [x] 实现预测成功率统计图表（时间序列、按股票排行）
  - [x] 实现预测失败归因分析展示

- [x] Task 11: 交易面板
  - [x] 实现真实交易下单界面（买入/卖出、限价/市价、数量、确认）
  - [x] 实现模拟交易面板（自动交易状态、今日成交记录、持仓）
  - [x] 实现持仓管理页面（真实持仓 + 模拟持仓、盈亏展示）
  - [x] 实现交易历史查询（日期筛选、交易类型筛选）
  - [x] 实现风险控制状态展示

## 阶段四：ML 预测引擎

- [x] Task 12: 特征工程管道
  - [x] 实现技术指标计算模块（MA、MACD、RSI、KDJ、BOLL、ATR、OBV）
  - [x] 实现基本面数据采集模块（PE、PB、ROE、营收增长率等）
  - [x] 实现市场情绪因子计算（换手率、资金流向、融资融券）
  - [x] 实现特征标准化和缺失值处理管道
  - [x] 实现特征存储（Feature Store on PostgreSQL）

- [x] Task 13: ML 模型训练与预测
  - [x] 实现短期 LSTM+Attention 模型（训练 + 推理）
  - [x] 实现中短期 XGBoost+LightGBM 集成模型（训练 + 推理）
  - [x] 实现长期 Transformer 模型（训练 + 推理）
  - [x] 实现模型版本管理（MLflow 集成）
  - [x] 实现每日定时预测任务（APScheduler）
  - [x] 实现模型自动重训练触发逻辑

- [x] Task 14: 预测服务 API
  - [x] 实现 FastAPI 预测服务框架
  - [x] 实现单股票预测查询 API
  - [x] 实现批量预测查询 API
  - [x] 实现预测结果存储到数据库
  - [x] 实现模型健康检查端点

## 阶段五：交易引擎

- [x] Task 15: 模拟交易引擎
  - [x] 实现模拟交易决策引擎（读取预测 → 过滤 → 排序 → 仓位管理 → 生成指令）
  - [x] 实现模拟交易执行器（模拟成交、滑点模拟）
  - [x] 实现模拟账户管理（资金、持仓、交易记录）
  - [x] 实现定时触发（开市期间每 30 分钟执行一次决策）
  - [x] 实现风险控制（单日亏损 5% 熔断、单票 20% 仓位上限、行业 40% 上限）
  - [x] 实现盘后自动结算

- [x] Task 16: 真实交易引擎
  - [x] 实现券商 API 抽象接口（统一买入/卖出/查询接口）
  - [x] 实现模拟券商适配器（用于开发测试）
  - [x] 实现交易安全校验（二次验证、限额检查、异常检测）
  - [x] 实现交易审计日志（完整记录每次操作）
  - [x] 实现交易状态查询和撤单

- [x] Task 17: 预测成功率自评估
  - [x] 实现日度成功率计算任务（收盘后自动触发）
  - [x] 实现成功率统计聚合（7日、30日、累计）
  - [x] 实现预测失败归因分析（突发事件检测、财报发布检测、行业异动检测）
  - [x] 实现评估报告生成和存储

- [x] Task 18: 管理后台
  - [x] 实现数据源配置页面（API Key 管理、采集频率设置）
  - [x] 实现系统健康监控面板（服务状态、数据延迟、错误日志）
  - [x] 实现用户管理（角色、权限）
  - [x] 实现模型参数管理页面

- [x] Task 19: 系统集成与联调
  - [x] 端到端流程测试（数据采集 → 存储 → 预测 → 交易 → 评估）
  - [x] WebSocket 实时推送全链路测试
  - [x] 多市场数据源切换和降级测试
  - [x] 性能压测（100+ 股票同时订阅）
  - [x] 编写 docker-compose 一键启动脚本

# Task Dependencies
- Task 2 依赖 Task 1（需要 Docker 环境）
- Task 3 依赖 Task 2（需要数据库 Schema）
- Task 4、Task 5 依赖 Task 3（需要 API Gateway）
- Task 6 依赖 Task 5（需要数据采集完成）
- Task 7 依赖 Task 3（需要 API 端点定义）
- Task 8、Task 9 依赖 Task 4、Task 6（需要数据和接口）
- Task 10 依赖 Task 14（需要预测 API）
- Task 11 依赖 Task 15、Task 16（需要交易引擎）
- Task 12 依赖 Task 5（需要行情数据）
- Task 13 依赖 Task 12（需要特征工程）
- Task 14 依赖 Task 13（需要模型训练完成）
- Task 15 依赖 Task 14（需要预测结果）
- Task 16 依赖 Task 3（需要 API Gateway）
- Task 17 依赖 Task 15（需要模拟交易数据）
- Task 18 依赖 Task 3（需要 API Gateway）
- Task 19 依赖 Task 1~18（全系统集成）

# 可并行执行的任务组
- 组 A（数据层）：Task 4、Task 5 可并行
- 组 B（前端）：Task 7、Task 8 可并行，Task 9、Task 10 可并行（在 Task 7 完成后）
- 组 C（ML）：Task 12 完成后，Task 13 的三个模型可并行开发
- 组 D（交易）：Task 15、Task 16 可并行
- 组 E（管理）：Task 17、Task 18 可并行