# 量化交易系统 - 技术规格说明书

## 目录
1. [系统总体架构](#1-系统总体架构)
2. [技术栈选型](#2-技术栈选型)
3. [数据层设计](#3-数据层设计)
4. [核心服务模块设计](#4-核心服务模块设计)
5. [ML预测引擎](#5-ml预测引擎)
6. [前端实时展示](#6-前端实时展示)
7. [部署与运维](#7-部署与运维)
8. [附录](#8-附录)

---

## 1. 系统总体架构

### 1.1 架构概览

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           Frontend (React + WebSocket)                    │
│              Dashboard │ Stock Search │ Prediction │ Trading │ Analytics  │
└───────────────────────────────────┬─────────────────────────────────────┘
                                    │ WebSocket / HTTP2 / gRPC-Web
┌───────────────────────────────────┼─────────────────────────────────────┐
│                           API Gateway (Kong/Envoy)                       │
│                  Rate Limit │ Auth │ Route │ Circuit Breaker             │
└───────┬───────────┬──────────┬──────────┬──────────┬──────────┬─────────┘
        │           │          │          │          │          │
┌───────▼──┐ ┌──────▼───┐ ┌───▼────┐ ┌───▼────┐ ┌───▼────┐ ┌───▼────────┐
│ Market   │ │ Stock    │ │ Real-  │ │Trading │ │ML      │ │Performance │
│ Data     │ │ Search   │ │ time   │ │Engine  │ │Predict │ │Analytics   │
│ Ingest   │ │ Service  │ │ Push   │ │        │ │Service │ │Service     │
└───┬──────┘ └───┬──────┘ └───┬────┘ └───┬────┘ └───┬────┘ └───┬────────┘
    │             │            │          │          │           │
┌───▼─────────────▼────────────▼──────────▼──────────▼───────────▼────────┐
│                        Message Queue (Kafka / Redpanda)                  │
│   market.raw │ market.ticks │ trade.orders │ trade.executions │ ml.events │
└────────────────────────────────────┬────────────────────────────────────┘
                                     │
┌────────────────────────────────────┼────────────────────────────────────┐
│                            Data Layer                                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐ │
│  │PostgreSQL│  │  Redis   │  │InfluxDB/ │  │Elastic-  │  │  MinIO   │ │
│  │(业务数据)│  │(缓存/实时)│  │Timescale │  │search    │  │(模型存储)│ │
│  │          │  │          │  │(时序数据)│  │(搜索)    │  │          │ │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘  └──────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
```

### 1.2 核心设计原则

| 原则 | 说明 |
|------|------|
| **事件驱动** | 所有数据流通过 Kafka 异步解耦，保证高吞吐和低延迟 |
| **CQRS** | 读写分离，查询走 Redis/ES 缓存，写入走 PostgreSQL |
| **微服务** | 每个核心能力独立部署，独立扩缩容 |
| **最终一致性** | 实时行情走内存/Redis，历史数据走时序库，允许短暂不一致 |
| **容错与降级** | 每个服务有熔断器，数据源有主备切换 |

### 1.3 数据流设计

```
外部行情源 (Bloomberg/Wind/东方财富/Yahoo Finance/Interactive Brokers)
    │
    ▼
Market Data Ingest Service (多源聚合 + 标准化)
    │
    ├──→ Kafka(market.ticks) ──→ Real-time Push Service ──→ WebSocket ──→ Frontend
    │
    ├──→ InfluxDB (tick级历史数据)
    │
    ├──→ Redis (最新行情快照，毫秒级读取)
    │
    └──→ Kafka(market.minute) ──→ ML Predict Service ──→ PostgreSQL (预测结果)
                                      │
                                      ▼
                               Trading Engine (决策 + 执行)
                                      │
                    ┌─────────────────┼─────────────────┐
                    ▼                 ▼                   ▼
              真实交易账户        模拟交易账户         Performance Analytics
              (IB/券商API)       (内部撮合)           (预测准确率统计)
```

---

## 2. 技术栈选型

### 2.1 后端

| 组件 | 选型 | 理由 |
|------|------|------|
| **语言** | Go + Python | Go: 高并发服务（行情接入、推送、交易引擎）；Python: ML预测服务 |
| **Web框架(Go)** | Fiber (基于Fasthttp) | 极致性能，比Gin快10倍，适合低延迟场景 |
| **Web框架(Python)** | FastAPI | 异步支持好，ML生态完善，与模型推理无缝集成 |
| **RPC框架** | gRPC | 服务间高性能通信，支持流式传输 |
| **API Gateway** | Envoy + 自研控制面 | 高性能代理，支持gRPC-Web转换 |

### 2.2 数据存储

| 存储 | 用途 | 理由 |
|------|------|------|
| **PostgreSQL 16** | 用户、账户、订单、持仓、预测结果 | 成熟可靠，支持复杂查询和事务 |
| **Redis Stack** | 实时行情缓存、会话、分布式锁、排行榜 | 亚毫秒级读写，支持Pub/Sub和Stream |
| **TimescaleDB** | Tick级历史行情、K线数据 | PostgreSQL扩展，时序压缩+自动分区，比InfluxDB更适合金融场景 |
| **Elasticsearch** | 股票代码全文搜索 | 多语言分词、模糊搜索、聚合分析 |
| **MinIO** | ML模型文件、日志归档 | S3兼容，自建低成本 |

### 2.3 消息队列

| 组件 | 选型 | 理由 |
|------|------|------|
| **Redpanda** | 核心消息队列 | Kafka协议兼容，无ZooKeeper依赖，延迟更低，吞吐更高 |

### 2.4 前端

| 组件 | 选型 | 理由 |
|------|------|------|
| **框架** | React 18 + TypeScript | 生态丰富，性能好 |
| **状态管理** | Zustand | 轻量，适合高频更新的行情状态 |
| **实时通信** | WebSocket (原生) | 毫秒级推送，全双工 |
| **图表** | Lightweight Charts (TradingView) + ECharts | 专业金融图表 + 通用图表 |
| **UI组件** | Ant Design | 企业级组件库，表格/表单能力强大 |
| **构建** | Vite | 快速HMR，构建快 |

### 2.5 基础设施

| 组件 | 选型 | 理由 |
|------|------|------|
| **容器编排** | Kubernetes (K8s) | 微服务编排，自动扩缩容 |
| **服务网格** | Istio | 流量管理、可观测性 |
| **监控** | Prometheus + Grafana | 指标采集与可视化 |
| **日志** | Loki + Promtail | 轻量日志聚合 |
| **链路追踪** | Jaeger | 分布式追踪 |
| **CI/CD** | GitLab CI / GitHub Actions | 自动化构建部署 |

---

## 3. 数据层设计

### 3.1 PostgreSQL 核心表结构

```sql
-- ============================================================
-- 1. 市场与股票基础信息
-- ============================================================

-- 市场定义
CREATE TABLE markets (
    id          SERIAL PRIMARY KEY,
    code        VARCHAR(10)  NOT NULL UNIQUE,  -- US, CN, HK, JP, UK, EU...
    name        VARCHAR(100) NOT NULL,          -- 美股, A股, 港股...
    name_en     VARCHAR(100),
    region      VARCHAR(20)  NOT NULL,          -- 区域: ASIA, AMERICA, EUROPE, OCEANIA
    timezone    VARCHAR(50)  NOT NULL,          -- 时区: America/New_York, Asia/Shanghai...
    currency    VARCHAR(10)  NOT NULL,          -- USD, CNY, HKD, JPY...
    open_time   TIME         NOT NULL,
    close_time  TIME         NOT NULL,
    lunch_start TIME,                            -- 午休开始（A股特有）
    lunch_end   TIME,                            -- 午休结束
    is_active   BOOLEAN      DEFAULT true,
    created_at  TIMESTAMPTZ  DEFAULT NOW()
);

-- 股票信息
CREATE TABLE stocks (
    id          BIGSERIAL PRIMARY KEY,
    market_code VARCHAR(10)  NOT NULL REFERENCES markets(code),
    symbol      VARCHAR(50)  NOT NULL,          -- 本地代码: AAPL, 600519
    isin        VARCHAR(20),                     -- 国际证券识别码
    name        VARCHAR(200) NOT NULL,           -- 名称
    name_en     VARCHAR(200),
    sector      VARCHAR(100),                    -- GICS行业分类
    industry    VARCHAR(100),                    -- 细分行业
    market_cap  DECIMAL(20,2),                   -- 市值
    currency    VARCHAR(10),
    lot_size    INTEGER      DEFAULT 1,          -- 每手股数
    is_active   BOOLEAN      DEFAULT true,
    is_etf      BOOLEAN      DEFAULT false,
    is_index    BOOLEAN      DEFAULT false,
    created_at  TIMESTAMPTZ  DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  DEFAULT NOW(),
    UNIQUE(market_code, symbol)
);

-- 股票搜索索引（用于Elasticsearch同步）
CREATE INDEX idx_stocks_name ON stocks USING gin(to_tsvector('simple', name || ' ' || COALESCE(name_en,'')));
CREATE INDEX idx_stocks_symbol ON stocks(symbol);
CREATE INDEX idx_stocks_sector ON stocks(sector);

-- ============================================================
-- 2. 用户与账户
-- ============================================================

CREATE TABLE users (
    id          BIGSERIAL PRIMARY KEY,
    username    VARCHAR(50)  NOT NULL UNIQUE,
    email       VARCHAR(200) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role        VARCHAR(20)  DEFAULT 'user',    -- user, admin, analyst
    risk_level  VARCHAR(20)  DEFAULT 'moderate', -- conservative, moderate, aggressive
    status      VARCHAR(20)  DEFAULT 'active',
    created_at  TIMESTAMPTZ  DEFAULT NOW()
);

CREATE TABLE accounts (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT       NOT NULL REFERENCES users(id),
    account_type    VARCHAR(20)  NOT NULL,       -- real, simulated
    broker          VARCHAR(50),                 -- 券商: IB, 华泰, 中信...
    account_number  VARCHAR(100),
    initial_capital DECIMAL(20,4) NOT NULL,
    current_cash    DECIMAL(20,4) NOT NULL,
    currency        VARCHAR(10)  DEFAULT 'USD',
    status          VARCHAR(20)  DEFAULT 'active',
    created_at      TIMESTAMPTZ  DEFAULT NOW(),
    CONSTRAINT chk_account_type CHECK (account_type IN ('real', 'simulated'))
);

-- ============================================================
-- 3. 持仓与订单
-- ============================================================

CREATE TABLE positions (
    id              BIGSERIAL PRIMARY KEY,
    account_id      BIGINT       NOT NULL REFERENCES accounts(id),
    stock_id        BIGINT       NOT NULL REFERENCES stocks(id),
    quantity        INTEGER      NOT NULL,       -- 正=多仓, 负=空仓
    avg_cost        DECIMAL(20,8) NOT NULL,      -- 平均成本价
    current_price   DECIMAL(20,8),               -- 当前市价（实时更新）
    market_value    DECIMAL(20,4),               -- 市值
    unrealized_pnl  DECIMAL(20,4),               -- 未实现盈亏
    realized_pnl    DECIMAL(20,4) DEFAULT 0,     -- 已实现盈亏
    updated_at      TIMESTAMPTZ  DEFAULT NOW()
);

CREATE TABLE orders (
    id              BIGSERIAL PRIMARY KEY,
    account_id      BIGINT       NOT NULL REFERENCES accounts(id),
    stock_id        BIGINT       NOT NULL REFERENCES stocks(id),
    order_type      VARCHAR(20)  NOT NULL,       -- market, limit, stop_loss, stop_limit
    direction       VARCHAR(10)  NOT NULL,       -- buy, sell, short, cover
    quantity        INTEGER      NOT NULL,
    price           DECIMAL(20,8),               -- 限价（市价单为NULL）
    filled_qty      INTEGER      DEFAULT 0,
    filled_avg_price DECIMAL(20,8),
    status          VARCHAR(20)  DEFAULT 'pending', -- pending, partial, filled, cancelled, rejected
    source          VARCHAR(20)  DEFAULT 'manual',  -- manual, auto_ml, strategy
    strategy_id     VARCHAR(100),                -- 关联策略ID
    created_at      TIMESTAMPTZ  DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  DEFAULT NOW()
);

-- ============================================================
-- 4. ML预测与回测
-- ============================================================

CREATE TABLE predictions (
    id              BIGSERIAL PRIMARY KEY,
    stock_id        BIGINT       NOT NULL REFERENCES stocks(id),
    model_id        VARCHAR(100) NOT NULL,       -- 模型标识
    horizon         VARCHAR(20)  NOT NULL,       -- short_term, medium_term, long_term
    predicted_price DECIMAL(20,8) NOT NULL,
    confidence      DECIMAL(5,4) NOT NULL,       -- 置信度 0-1
    direction       VARCHAR(10)  NOT NULL,       -- bullish, bearish, neutral
    features_hash   VARCHAR(64),                 -- 输入特征哈希，用于去重
    prediction_time TIMESTAMPTZ  NOT NULL,
    target_time     TIMESTAMPTZ  NOT NULL,       -- 预测目标时间
    created_at      TIMESTAMPTZ  DEFAULT NOW()
);

CREATE TABLE prediction_results (
    id              BIGSERIAL PRIMARY KEY,
    prediction_id   BIGINT       NOT NULL REFERENCES predictions(id),
    actual_price    DECIMAL(20,8) NOT NULL,
    error           DECIMAL(20,8) NOT NULL,      -- 绝对误差
    error_pct       DECIMAL(10,6) NOT NULL,      -- 百分比误差
    direction_correct BOOLEAN,                   -- 方向预测是否正确
    profitable      BOOLEAN,                     -- 按此预测交易是否盈利
    evaluated_at    TIMESTAMPTZ  DEFAULT NOW()
);

-- 预测准确率聚合（每日更新）
CREATE TABLE prediction_accuracy (
    id              SERIAL PRIMARY KEY,
    stock_id        BIGINT REFERENCES stocks(id),
    model_id        VARCHAR(100) NOT NULL,
    horizon         VARCHAR(20)  NOT NULL,
    date            DATE         NOT NULL,
    total_predictions   INTEGER,
    direction_accuracy  DECIMAL(5,4),            -- 方向准确率
    profit_ratio        DECIMAL(5,4),            -- 盈利比例
    avg_error_pct       DECIMAL(10,6),
    sharpe_ratio        DECIMAL(10,4),
    created_at      TIMESTAMPTZ  DEFAULT NOW(),
    UNIQUE(stock_id, model_id, horizon, date)
);

-- ============================================================
-- 5. 模拟交易日志
-- ============================================================

CREATE TABLE simulation_log (
    id              BIGSERIAL PRIMARY KEY,
    account_id      BIGINT       NOT NULL REFERENCES accounts(id),
    event_type      VARCHAR(30)  NOT NULL,       -- decision, order, execution, summary
    stock_id        BIGINT REFERENCES stocks(id),
    prediction_id   BIGINT REFERENCES predictions(id),
    action          VARCHAR(20),                 -- buy, sell, hold
    quantity        INTEGER,
    price           DECIMAL(20,8),
    reason          TEXT,                        -- 决策理由
    metadata        JSONB,                       -- 扩展数据
    created_at      TIMESTAMPTZ  DEFAULT NOW()
);
```

### 3.2 Redis 数据结构设计

```
# 1. 实时行情快照 (String + Hash)
market:tick:{market_code}:{symbol}          → Hash {price, volume, bid, ask, bid_size, ask_size, 
                                                   change, change_pct, high, low, open, prev_close, 
                                                   timestamp}
market:tick:all                              → Sorted Set (按涨跌幅排序)

# 2. 实时K线 (Stream)
market:kline:{market_code}:{symbol}:1min     → Stream (最多保留当日数据)
market:kline:{market_code}:{symbol}:5min
market:kline:{market_code}:{symbol}:30min
market:kline:{market_code}:{symbol}:daily

# 3. 市场状态 (String)
market:status:{market_code}                  → String (open, closed, lunch_break, pre_market, after_hours)

# 4. 用户会话 (Hash)
session:{session_id}                         → Hash {user_id, username, expires_at, ...}

# 5. 模拟交易排行榜 (Sorted Set)
sim:leaderboard:daily:{date}                 → ZSet (按日收益率排序)
sim:leaderboard:weekly:{monday_date}
sim:leaderboard:monthly:{year_month}

# 6. 预测缓存 (String)
prediction:latest:{stock_id}:{horizon}       → JSON (最新预测结果)

# 7. 分布式锁
lock:auto_trade:{account_id}                 → String (防止重复下单)
```

### 3.3 TimescaleDB 超表设计

```sql
-- Tick级行情数据
CREATE TABLE market_ticks (
    time            TIMESTAMPTZ  NOT NULL,
    stock_id        BIGINT       NOT NULL,
    price           DECIMAL(20,8) NOT NULL,
    volume          BIGINT,
    bid             DECIMAL(20,8),
    ask             DECIMAL(20,8),
    bid_size        INTEGER,
    ask_size        INTEGER,
    source          VARCHAR(30)               -- 数据源标识
);

SELECT create_hypertable('market_ticks', 'time', chunk_time_interval => INTERVAL '1 day');
CREATE INDEX idx_ticks_stock_time ON market_ticks(stock_id, time DESC);

-- 启用压缩（7天后压缩）
SELECT add_compression_policy('market_ticks', INTERVAL '7 days');

-- 聚合K线物化视图（持续聚合）
CREATE MATERIALIZED VIEW kline_1min
WITH (timescaledb.continuous) AS
SELECT 
    time_bucket('1 minute', time) AS bucket,
    stock_id,
    FIRST(price, time) AS open,
    MAX(price) AS high,
    MIN(price) AS low,
    LAST(price, time) AS close,
    SUM(volume) AS volume
FROM market_ticks
GROUP BY bucket, stock_id;
```

### 3.4 Elasticsearch 搜索索引

```json
// stocks 索引映射
{
  "mappings": {
    "properties": {
      "symbol":          { "type": "keyword" },
      "name":            { "type": "text", "analyzer": "standard", "fields": {
                           "keyword": { "type": "keyword" },
                           "pinyin":  { "type": "text", "analyzer": "pinyin_analyzer" }
                         }},
      "name_en":         { "type": "text", "analyzer": "english" },
      "market_code":     { "type": "keyword" },
      "market_name":     { "type": "keyword" },
      "region":          { "type": "keyword" },
      "sector":          { "type": "keyword" },
      "industry":        { "type": "keyword" },
      "suggest":         { "type": "completion" }
    }
  }
}
```

---

## 4. 核心服务模块设计

### 4.1 Market Data Ingest Service（行情接入服务）

**语言**: Go  
**职责**: 连接多个外部行情源，标准化数据格式，写入Kafka

```
架构:
┌──────────────────────────────────────────────────┐
│              Market Data Ingest Service           │
│                                                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐       │
│  │ Bloomberg│  │   Wind   │  │  Yahoo   │  ...  │
│  │ Adapter  │  │ Adapter  │  │ Adapter  │       │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘       │
│       │              │              │             │
│       └──────────────┼──────────────┘             │
│                      ▼                            │
│           ┌──────────────────┐                   │
│           │ Normalizer       │  ← 统一格式        │
│           └────────┬─────────┘                   │
│                    ▼                              │
│           ┌──────────────────┐                   │
│           │ Deduplicator     │  ← 多源去重        │
│           └────────┬─────────┘                   │
│                    ▼                              │
│           ┌──────────────────┐                   │
│           │ Kafka Producer   │                   │
│           └──────────────────┘                   │
└──────────────────────────────────────────────────┘
```

**多源去重策略**: 为每个数据源配置优先级（Bloomberg > Wind > 东方财富 > Yahoo），同一tick取优先级最高的源。

**数据源覆盖计划**:

| 市场 | 实时源 | 备选源 | 延迟 |
|------|--------|--------|------|
| 美股 | Interactive Brokers / Polygon.io | Yahoo Finance | <100ms |
| A股 | 东方财富/同花顺 Level-2 | 新浪财经 | <500ms |
| 港股 | 港交所 OMDF | 腾讯财经 | <500ms |
| 日股 | JPX | Yahoo Finance JP | <500ms |
| 欧股 | Bloomberg / Refinitiv | Yahoo Finance | <100ms |

### 4.2 Stock Search Service（股票搜索服务）

**语言**: Go  
**API 设计**:

```protobuf
service StockSearch {
  // 全文搜索
  rpc Search(SearchRequest) returns (SearchResponse);
  // 自动补全
  rpc Suggest(SuggestRequest) returns (SuggestResponse);
  // 按市场获取热门股票
  rpc GetHotStocks(GetHotStocksRequest) returns (GetHotStocksResponse);
}

message SearchRequest {
  string keyword = 1;
  repeated string markets = 2;    // 市场过滤
  repeated string regions = 3;    // 区域过滤: ASIA, AMERICA, EUROPE
  string sector = 4;
  int32 page = 5;
  int32 page_size = 6;
}

message SearchResponse {
  repeated StockResult results = 1;
  int64 total = 2;
  map<string, int64> market_aggregations = 3;  // 按市场聚合数量
}
```

**前端分组展示**:
```
搜索 "苹果"
├── 🇺🇸 美股 (2)
│   ├── AAPL - Apple Inc. - 科技
│   └── AAPB - GraniteShares 2x Long AAPL - ETF
├── 🇨🇳 A股 (1)
│   └── 600519 - 贵州茅台 (关联: 苹果概念股)
├── 🇭🇰 港股 (0)
└── 🇯🇵 日股 (0)
```

### 4.3 Real-time Push Service（实时推送服务）

**语言**: Go  
**核心能力**: 毫秒级行情推送，支持订阅/退订

```
┌──────────────────────────────────────────────────┐
│            Real-time Push Service                │
│                                                   │
│  ┌─────────────────────────────────┐             │
│  │ Kafka Consumer Group            │  ← 消费tick │
│  │ (分区对应当前在线用户数)         │             │
│  └────────────┬────────────────────┘             │
│               ▼                                   │
│  ┌─────────────────────────────────┐             │
│  │ Fan-out Router                  │             │
│  │ (根据用户订阅分发给WebSocket)     │             │
│  └────────────┬────────────────────┘             │
│               ▼                                   │
│  ┌─────────────────────────────────┐             │
│  │ Connection Manager              │             │
│  │ - 心跳检测 (15s)                 │             │
│  │ - 连接池管理                     │             │
│  │ - 自动重连                       │             │
│  └────────────┬────────────────────┘             │
│               ▼                                   │
│         WebSocket (每连接一个goroutine)            │
└──────────────────────────────────────────────────┘
```

**WebSocket 消息协议**:

```json
// 客户端→服务端
{
  "type": "subscribe",
  "channels": ["tick:US:AAPL", "tick:CN:600519", "kline:US:AAPL:1min"],
  "request_id": "uuid"
}

// 服务端→客户端 (tick)
{
  "type": "tick",
  "market": "US",
  "symbol": "AAPL",
  "price": 185.32,
  "change": 2.15,
  "change_pct": 1.173,
  "volume": 12456789,
  "bid": 185.31,
  "ask": 185.33,
  "timestamp": 1700000000123,
  "seq": 123456
}
```

**性能优化**:
- 使用 `gnet` 网络库（事件驱动，比标准net/http快10x）
- 消息批量发送（每1ms聚合一次，最多聚合50条）
- 零拷贝序列化（自研二进制协议或Protobuf）

### 4.4 Trading Engine（交易引擎）

**语言**: Go  
**职责**: 订单管理、风控、撮合（模拟盘）、执行（实盘）

```
┌──────────────────────────────────────────────────┐
│                  Trading Engine                    │
│                                                   │
│  ┌──────────────┐  ┌──────────────┐              │
│  │ Order Manager│  │ Risk Control │              │
│  │ - 订单创建    │  │ - 仓位限制    │              │
│  │ - 订单状态    │  │ - 单笔限额    │              │
│  │ - 订单匹配    │  │ - 日亏损限额  │              │
│  └──────┬───────┘  │ - 涨跌停限制  │              │
│         │           └──────┬───────┘              │
│         └──────────┬───────┘                       │
│                    ▼                               │
│  ┌──────────────────────────────────┐             │
│  │       Trade Router               │             │
│  │   ┌────────────┐ ┌────────────┐  │             │
│  │   │ Real Exec  │ │ Sim Match  │  │             │
│  │   │ (IB/券商)  │ │ (内部撮合)  │  │             │
│  │   └────────────┘ └────────────┘  │             │
│  └──────────────────────────────────┘             │
└──────────────────────────────────────────────────┘
```

**模拟盘撮合逻辑**:
- 市价单: 以当前最新tick价格立即成交
- 限价单: 价格达到即成交
- 考虑滑点: 模拟盘成交价 = 市价 × (1 + 随机滑点∈[-0.1%, 0.1%])
- 考虑流动性: 每笔模拟交易量不超过当日成交量×1%

**风控规则**:

| 规则 | 阈值 | 说明 |
|------|------|------|
| 单只股票仓位上限 | 总资产的20% | 防止过度集中 |
| 单笔交易额上限 | 总资产的5% | 分散风险 |
| 日亏损上限 | 总资产的3% | 触发停止交易 |
| 周亏损上限 | 总资产的5% | 触发暂停自动交易 |
| 涨跌停限制 | 市场规则 | 涨停不买，跌停不卖 |

### 4.5 Auto Trading Scheduler（自动交易调度）

**语言**: Go  
**职责**: 在开市期间自动运行ML决策，执行模拟交易

```
┌──────────────────────────────────────────────────┐
│           Auto Trading Scheduler                  │
│                                                   │
│  1. 市场状态检测 (cron: 每分钟)                    │
│     └─→ 查询各市场是否开市                         │
│                                                   │
│  2. 开市时触发交易决策循环 (每5分钟)                │
│     ├─→ 获取最新预测结果 (短/中/长期)               │
│     ├─→ 综合评分 = 预测方向置信度 × 0.4             │
│     │            + 预测收益率 × 0.3                 │
│     │            + 技术指标信号 × 0.2               │
│     │            + 市场情绪 × 0.1                   │
│     ├─→ 排名，选取Top-N                            │
│     ├─→ 风控检查                                    │
│     ├─→ 生成订单                                    │
│     └─→ 记录决策日志                                │
│                                                   │
│  3. 收市后总结 (cron: 收市后10分钟)                 │
│     ├─→ 计算当日盈亏                               │
│     ├─→ 对比预测与实际 → 更新准确率                 │
│     └─→ 生成每日总结报告                           │
└──────────────────────────────────────────────────┘
```

**决策示例** (当日开始时有5只股票在模拟盘):

```
初始资金: $100,000
预测结果:
  AAPL: 短期看涨(置信度0.85), 预期收益+2.3%  → 综合评分0.83 → 买入$4,000
  GOOGL: 短期看涨(置信度0.72), 预期收益+1.8% → 综合评分0.71 → 买入$3,000
  TSLA: 短期看跌(置信度0.68), 预期收益-1.5%  → 综合评分0.30 → 卖出持仓
  600519: 短期看涨(置信度0.91), 预期收益+1.2% → 综合评分0.78 → 买入$3,500
  0700.HK: 中性(置信度0.55), 预期收益+0.3%  → 综合评分0.45 → 持有观望
```

---

## 5. ML预测引擎

### 5.1 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                      ML Prediction Engine                        │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                   Feature Engineering                     │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐ │   │
│  │  │ Technical│  │Fundamental│  │Sentiment │  │  Macro   │ │   │
│  │  │Indicators│  │  Data    │  │(NLP)     │  │  Metrics │ │   │
│  │  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘ │   │
│  │       └──────────────┼──────────────┼──────────────┘      │   │
│  │                      ▼                                     │   │
│  │              Feature Store (Redis + Feast)                 │   │
│  └──────────────────────────────────────────────────────────┘   │
│                              │                                    │
│  ┌───────────────────────────┼──────────────────────────────┐   │
│  │                    Model Ensemble                         │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐               │   │
│  │  │LightGBM  │  │LSTM/     │  │XGBoost   │  ...          │   │
│  │  │(短期)    │  │Transformer│  │(中期)    │               │   │
│  │  │          │  │(短期)    │  │          │               │   │
│  │  └────┬─────┘  └────┬─────┘  └────┬─────┘               │   │
│  │       └──────────────┼──────────────┘                     │   │
│  │                      ▼                                     │   │
│  │              Ensemble Voting                              │   │
│  │       (加权平均 + 置信度校准)                              │   │
│  └──────────────────────────────────────────────────────────┘   │
│                              │                                    │
│  ┌───────────────────────────┼──────────────────────────────┐   │
│  │                  Model Serving (Triton / BentoML)        │   │
│  │          gRPC / HTTP inference with GPU acceleration     │   │
│  └──────────────────────────────────────────────────────────┘   │
│                              │                                    │
│  ┌───────────────────────────┼──────────────────────────────┐   │
│  │                  Result Aggregation & Storage            │   │
│  │         PostgreSQL (predictions) + Redis (cache)         │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### 5.2 特征工程

#### 技术指标 (300+ features per stock per timeframe)

| 类别 | 指标 | 特征数 |
|------|------|--------|
| 趋势 | MA(5,10,20,60,120), EMA, MACD, ADX, Parabolic SAR | ~30 |
| 动量 | RSI, Stochastic, CCI, Williams %R, MFI | ~20 |
| 波动 | Bollinger Bands, ATR, Keltner Channels, Historical Volatility | ~20 |
| 成交量 | OBV, Volume Profile, VWAP, CMF, Force Index | ~20 |
| 形态 | 识别: 头肩顶/底, 双顶/底, 三角形, 旗形 | ~15 |
| 自定义 | 滚动夏普比率, 最大回撤, 偏度, 峰度, Hurst指数 | ~20 |

#### 基本面特征 (季度更新)

| 指标 | 来源 |
|------|------|
| P/E, P/B, P/S, EV/EBITDA | 财报 |
| ROE, ROA, 毛利率, 净利率 | 财报 |
| 营收增长率, 利润增长率 | 财报 |
| 负债率, 流动比率, 速动比率 | 财报 |
| 分红率, 股息率 | 财报 |

#### 情绪特征 (NLP, 每日更新)

| 来源 | 内容 |
|------|------|
| 新闻 | 标题/摘要情感分析 (FinBERT) |
| 社交 | Twitter/Reddit/雪球 讨论情绪 |
| 研报 | 分析师评级变化 |
| 内部 | 高管增持/减持 |

#### 宏观特征

| 指标 | 频率 |
|------|------|
| 利率 (Fed, PBOC, ECB...) | 月度 |
| GDP | 季度 |
| CPI/PPI | 月度 |
| PMI | 月度 |
| VIX 恐慌指数 | 每日 |
| 美元指数, 汇率 | 每日 |
| 板块资金流向 | 每日 |

### 5.3 模型策略

#### 短期预测 (1-5天)

| 模型 | 用途 | 更新频率 |
|------|------|----------|
| **LightGBM** | 次日涨跌方向二分类 | 每日 |
| **LSTM + Attention** | 未来5天价格序列预测 | 每日 |
| **Temporal Fusion Transformer** | 多变量时间序列预测 | 每日 |

输入: 最近60天技术指标 + 最近5天情绪 + 最新基本面
输出: 未来1-5天涨跌概率 + 目标价格区间

#### 中短期预测 (1-4周)

| 模型 | 用途 | 更新频率 |
|------|------|----------|
| **XGBoost** | 未来1-4周收益率回归 | 每周 |
| **CatBoost** | 板块轮动检测 | 每周 |
| **N-BEATS** | 可解释时间序列预测 | 每周 |

输入: 最近1年周线技术指标 + 基本面 + 宏观 + 板块轮动
输出: 未来1-4周预期收益率 + 置信度

#### 长期预测 (1-6月)

| 模型 | 用途 | 更新频率 |
|------|------|----------|
| **多因子模型** | 基本面+宏观驱动的估值 | 月度 |
| **Prophet** | 趋势分解 + 季节效应 | 月度 |
| **图神经网络(GNN)** | 产业链关联分析 | 月度 |

输入: 5年历史数据 + 基本面 + 宏观 + 产业链关系
输出: 未来1-6月价值区间 + 投资建议

### 5.4 模型评估与回测

```
┌──────────────────────────────────────────────────────────┐
│                    Backtesting Framework                  │
│                                                           │
│  1. 训练/验证/测试集划分 (70/15/15, 按时间序列)          │
│  2. 滚动窗口回测 (Walk-Forward Validation)                │
│  3. 评估指标:                                            │
│     - 方向准确率 (Direction Accuracy)                    │
│     - 均方误差 (MSE) / 平均绝对误差 (MAE)                │
│     - 夏普比率 (Sharpe Ratio)                            │
│     - 最大回撤 (Max Drawdown)                            │
│     - 信息系数 (Information Coefficient)                 │
│  4. 过拟合检测: 样本外测试 vs 样本内测试差距 > 15% 则告警│
│  5. 模型版本管理: MLflow 记录每次训练参数和指标           │
└──────────────────────────────────────────────────────────┘
```

### 5.5 ML Pipeline 调度

```
Pipeline (Airflow / Prefect):

每日 06:00 UTC → 特征工程Pipeline
  ├── 技术指标计算
  ├── NLP情绪分析
  └── 宏观数据同步

每日 07:00 UTC → 短期模型训练
  ├── 增量训练 (LightGBM, LSTM)
  ├── 模型评估
  └── 推送到 Model Registry

每日 08:00 UTC → 预测生成
  ├── 短期预测 (所有跟踪股票)
  ├── 写入 PostgreSQL
  └── 更新 Redis 缓存

每周一 06:00 UTC → 中短期模型训练 + 预测

每月1日 06:00 UTC → 长期模型训练 + 预测
```

### 5.6 投资建议生成

根据预测结果生成分级投资建议:

```json
{
  "stock": "AAPL",
  "market": "US",
  "predictions": {
    "short_term": {
      "horizon": "1-5 days",
      "direction": "bullish",
      "confidence": 0.85,
      "target_price": 192.50,
      "expected_return": 0.023
    },
    "medium_term": {
      "horizon": "1-4 weeks",
      "direction": "bullish",
      "confidence": 0.72,
      "target_price": 198.00,
      "expected_return": 0.052
    },
    "long_term": {
      "horizon": "1-6 months",
      "direction": "bullish",
      "confidence": 0.68,
      "target_price": 210.00,
      "expected_return": 0.115
    }
  },
  "recommendation": {
    "level": "STRONG_BUY",
    "summary": "短期技术面强势，中长线基本面支撑，建议积极配置",
    "risk_warning": "短期超买，注意回调风险",
    "suggested_position": "总仓位10-15%"
  }
}
```

**投资建议等级**:
- STRONG_BUY: 三个周期一致看涨，置信度>0.7
- BUY: 两个周期看涨，置信度>0.6
- HOLD: 方向不一致或置信度低
- SELL: 两个周期看跌，置信度>0.6
- STRONG_SELL: 三个周期一致看跌，置信度>0.7

---

## 6. 前端实时展示

### 6.1 页面结构

```
┌──────────────────────────────────────────────────────────────┐
│  Header: Logo | 搜索框 | 用户信息 | 通知                     │
├──────────────────────────────────────────────────────────────┤
│  Sidebar:                                                    │
│  ├── 市场概览 Dashboard                                      │
│  ├── 股票搜索 🔍                                             │
│  ├── 实时行情                                                │
│  ├── 交易面板                                                │
│  │   ├── 实盘交易                                            │
│  │   └── 模拟交易                                            │
│  ├── AI预测                                                  │
│  │   ├── 短期预测                                            │
│  │   ├── 中短期预测                                          │
│  │   └── 长期预测                                            │
│  ├── 投资建议                                                │
│  ├── 业绩分析                                                │
│  │   ├── 预测准确率                                          │
│  │   ├── 模拟交易业绩                                        │
│  │   └── 实盘业绩                                            │
│  └── 系统设置                                                │
├──────────────────────────────────────────────────────────────┤
│                        主内容区                               │
└──────────────────────────────────────────────────────────────┘
```

### 6.2 核心页面设计

#### Dashboard（仪表盘）

```
┌──────────────────────────────────────────────────────────────┐
│  [全球市场概览]                    日期: 2026-07-24           │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐       │
│  │ 标普500   │ │ 上证综指  │ │ 恒生指数  │ │ 日经225  │  ...  │
│  │ 5,432.18  │ │ 3,215.87  │ │ 18,432.5  │ │ 38,921.1 │       │
│  │ ▲ +0.87%  │ │ ▼ -0.32%  │ │ ▲ +1.21%  │ │ ▲ +0.45% │       │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘       │
│                                                               │
│  [市场状态]  🟢 美股: 交易中 | 🟢 A股: 交易中 | 🔴 港股: 午休│
│                                                               │
│  [实时涨跌榜] (毫秒级更新)                                    │
│  ┌──────┬──────────┬────────┬──────────┬──────────┐         │
│  │ 排名 │ 股票      │ 价格   │ 涨跌幅   │ 成交量    │         │
│  │  1   │ AAPL     │ 185.32 │ ▲ +2.15% │ 12.4M    │         │
│  │  2   │ TSLA     │ 248.91 │ ▲ +4.32% │ 89.2M    │         │
│  │  ... │ ...      │ ...    │ ...      │ ...      │         │
│  └──────┴──────────┴────────┴──────────┴──────────┘         │
│                                                               │
│  [AI投资建议] (最新)                                          │
│  ┌──────────────────────────────────────────────────────┐    │
│  │ 🔥 STRONG_BUY: AAPL - 短期技术突破，中期盈利增长...   │    │
│  │ 📈 BUY: GOOGL - AI业务增长预期...                     │    │
│  │ ⚠️ SELL: IBM - 连续两个季度业绩不及预期...            │    │
│  └──────────────────────────────────────────────────────┘    │
│                                                               │
│  [我的模拟交易业绩]                                           │
│  ┌──────────┬──────────┬──────────┬──────────┐             │
│  │ 总资产    │ 今日盈亏  │ 总盈亏    │ 胜率     │             │
│  │ $108,234 │ ▲ +$1,234│ ▲ +$8,234│ 62.3%   │             │
│  └──────────┴──────────┴──────────┴──────────┘             │
└──────────────────────────────────────────────────────────────┘
```

#### 股票搜索页

```
┌──────────────────────────────────────────────────────────────┐
│  🔍 [________________________] 搜索                          │
│                                                               │
│  筛选: [全部市场 ▼] [全部区域 ▼] [全部行业 ▼]                 │
│                                                               │
│  ┌─ 美股 (15) ──────────────────────────────────────┐        │
│  │  AAPL    Apple Inc.          科技     $2.89T      │        │
│  │  MSFT    Microsoft Corp.     科技     $3.01T      │        │
│  │  ...                                              │        │
│  └────────────────────────────────────────────────────┘       │
│  ┌─ A股 (8) ────────────────────────────────────────┐        │
│  │  600519  贵州茅台            白酒     ¥2.15万亿    │        │
│  │  ...                                              │        │
│  └────────────────────────────────────────────────────┘       │
│  ┌─ 港股 (3) ────────────────────────────────────────┐       │
│  │  0700    腾讯控股            科技     HK$3.52万亿  │        │
│  │  ...                                              │        │
│  └────────────────────────────────────────────────────┘       │
└──────────────────────────────────────────────────────────────┘
```

#### 预测分析页

```
┌──────────────────────────────────────────────────────────────┐
│  股票: AAPL - Apple Inc.            [短期] [中短期] [长期]  │
│                                                               │
│  ┌─ 价格预测图 ──────────────────────────────────────┐       │
│  │  [K线图 + 预测区间 (阴影) + 置信度区间]             │       │
│  │                                                      │       │
│  │   $190 ┤     ╭───╮    ╭── 预测上线                  │       │
│  │   $185 ┤─────╯   ╰────╯── 预测中线                  │       │
│  │   $180 ┤────────────── 预测下线                      │       │
│  │        └──┬──┬──┬──┬──┬──┬──                        │       │
│  │         过去             未来                         │       │
│  └──────────────────────────────────────────────────────┘      │
│                                                               │
│  ┌──────────────────┐ ┌──────────────────┐                   │
│  │ 预测方向: 看涨 📈│ │ 置信度: 85%     │                   │
│  │ 目标价: $192.50  │ │ 预期收益: +2.3% │                   │
│  │ 止损: $180.00    │ │ 风险等级: 中等  │                   │
│  └──────────────────┘ └──────────────────┘                   │
│                                                               │
│  ┌─ 特征重要性 ───────────────────────────────────────┐      │
│  │  MACD          ████████████████████  28%            │      │
│  │  RSI           ██████████████        18%            │      │
│  │  Volume        ████████████          14%            │      │
│  │  Sentiment     ██████████            12%            │      │
│  │  ...                                              │      │
│  └────────────────────────────────────────────────────┘      │
│                                                               │
│  ┌─ 历史预测准确率 ───────────────────────────────────┐      │
│  │  (该股票)  短期: 72.3% | 中短期: 68.1% | 长期: 61.5%│      │
│  └────────────────────────────────────────────────────┘      │
└──────────────────────────────────────────────────────────────┘
```

### 6.3 前端性能优化

| 优化策略 | 实现方式 |
|----------|----------|
| **虚拟滚动** | 行情列表使用 `react-window`，只渲染可视区域 |
| **Web Worker** | 将WebSocket数据解析和聚合放在Worker线程 |
| **增量更新** | 只更新变化的数据，使用React.memo + useMemo |
| **Canvas图表** | 高频K线图使用Canvas而非SVG，减少DOM操作 |
| **批量DOM更新** | 使用requestAnimationFrame合并多次渲染 |
| **数据分层** | 自选股(高频) vs 列表(中频) vs 搜索(低频) |

---

## 7. 部署与运维

### 7.1 Kubernetes 部署拓扑

```
┌─────────────────────────────────────────────────────────────────┐
│                      Kubernetes Cluster                         │
│                                                                  │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │   Frontend Pod  │  │   Frontend Pod  │  │   Frontend Pod  │ │
│  │   (nginx+react) │  │   (nginx+react) │  │   (nginx+react) │ │
│  └────────┬────────┘  └────────┬────────┘  └────────┬────────┘ │
│           └────────────────────┼────────────────────┘           │
│                                ▼                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    Envoy Gateway                         │   │
│  │         (gRPC-Web ↔ gRPC 转换, 负载均衡)                 │   │
│  └────────┬──────────┬──────────┬──────────┬───────────────┘   │
│           │          │          │          │                    │
│  ┌────────▼──┐ ┌────▼─────┐ ┌──▼──────┐ ┌─▼──────────┐       │
│  │ API GW    │ │ Push Svc │ │ Trading │ │ ML Predict │       │
│  │ (Go) x3   │ │ (Go) x5  │ │ (Go) x2 │ │ (Py) x2    │       │
│  └───────────┘ └──────────┘ └─────────┘ └────────────┘       │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              Stateful Workloads                          │   │
│  │  PostgreSQL │ Redis │ TimescaleDB │ ES │ Redpanda │ MinIO│   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### 7.2 关键配置

| 配置项 | 值 | 说明 |
|--------|-----|------|
| 前端副本数 | 3 | 水平扩展 |
| Push Service副本数 | 5+ | 按WebSocket连接数自动扩缩(每个Pod 10K连接) |
| Trading Engine副本数 | 2 | 通过分布式锁保证单实例执行 |
| ML Predict副本数 | 2+ | GPU节点，按请求量HPA |
| PostgreSQL | 1主2从 + PgBouncer连接池 | 读写分离 |
| Redis | 3节点Sentinel | 高可用 |
| Redpanda | 3节点 | 高可用 |

### 7.3 监控告警

| 指标 | 告警阈值 | 级别 |
|------|----------|------|
| WebSocket推送延迟 | P99 > 100ms | P1 |
| 行情数据延迟 | 与交易所时间差 > 500ms | P1 |
| 交易引擎响应时间 | P99 > 500ms | P2 |
| ML预测生成延迟 | > 5分钟 | P2 |
| PostgreSQL慢查询 | > 1s | P2 |
| Redis内存使用率 | > 80% | P2 |
| Kafka消费延迟 | > 1000条 | P2 |
| 服务可用性 | < 99.9% | P0 |

---

## 8. 附录

### 8.1 设计优化说明

相对于原始需求，做了以下优化和补充：

1. **多源数据聚合**: 使用多个行情源 + 优先级去重，保证数据可靠性和低延迟
2. **CQRS读写分离**: 实时推送走Redis+WebSocket，历史查询走TimescaleDB，避免互相影响
3. **模拟交易真实性**: 模拟盘加入滑点、流动性限制、交易费用等真实约束
4. **风控体系**: 完整的风险控制（单票上限、日亏损上限、涨跌停处理），防止极端情况
5. **预测准确率统计**: 建立完整的预测→实际→评估闭环，按周期/股票/模型多维度统计
6. **模型版本管理**: MLflow管理模型版本和实验，支持AB测试和回滚
7. **市场时段管理**: 自动识别各市场交易时段，不同时区自动调度
8. **可观测性**: 全链路追踪、指标监控、告警体系
9. **数据源扩展性**: 适配器模式设计，新增行情源只需实现接口
10. **自动交易决策可解释性**: 记录每笔决策理由，便于审计和优化

### 8.2 开发阶段规划

| 阶段 | 内容 | 工期参考 |
|------|------|---------|
| Phase 1 | 基础架构搭建（K8s, 数据库, 消息队列, CI/CD） | 2周 |
| Phase 2 | 行情接入 + 实时推送 + 前端实时行情 | 3周 |
| Phase 3 | 股票搜索服务 + 前端搜索页 | 1周 |
| Phase 4 | 交易引擎（实盘+模拟盘） + 前端交易面板 | 3周 |
| Phase 5 | ML特征工程 + 模型训练 + 预测服务 | 4周 |
| Phase 6 | 自动交易调度 + 预测准确率统计 | 2周 |
| Phase 7 | 投资建议生成 + 前端预测分析页 | 2周 |
| Phase 8 | 性能优化 + 压测 + 安全审计 | 2周 |

### 8.3 关键技术风险与应对

| 风险 | 应对措施 |
|------|----------|
| 行情源不可用 | 多源备选 + 自动切换 + 降级到缓存数据 |
| 网络延迟导致预测滞后 | 预测使用分钟级K线数据，对实时性要求低于tick级 |
| ML模型过拟合 | 严格的滚动窗口回测 + 样本外验证 + 定期重新训练 |
| 模拟交易与现实偏差 | 引入滑点、手续费、流动性约束，定期校准 |
| 高并发WebSocket连接 | 水平扩展Push Service + 连接数HPA + gnet高性能网络库 |
| 跨市场时区调度 | 每个市场独立cron，基于market.timezone字段调度 |

---

*文档版本: v1.0 | 日期: 2026-07-24*