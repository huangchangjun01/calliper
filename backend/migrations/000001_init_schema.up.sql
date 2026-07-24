-- 000001_init_schema.up.sql
-- Quantitative Trading System - Initial Schema Migration
-- Requires: TimescaleDB extension, PostgreSQL 14+

-- ============================================================
-- Extensions
-- ============================================================
CREATE EXTENSION IF NOT EXISTS timescaledb;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================
-- Reference Tables
-- ============================================================

-- Markets: stock exchange definitions
CREATE TABLE IF NOT EXISTS markets (
    id              BIGSERIAL PRIMARY KEY,
    code            VARCHAR(20)   NOT NULL UNIQUE,
    name            VARCHAR(100)  NOT NULL,
    name_cn         VARCHAR(100),
    country         VARCHAR(50)   NOT NULL,
    currency        VARCHAR(10)   NOT NULL,
    timezone        VARCHAR(50)   NOT NULL,
    trading_hours   TEXT,
    status          VARCHAR(20)   NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'inactive')),
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- Stocks: tradable instrument definitions
CREATE TABLE IF NOT EXISTS stocks (
    id              BIGSERIAL PRIMARY KEY,
    symbol          VARCHAR(20)   NOT NULL,
    name            VARCHAR(200)  NOT NULL,
    name_cn         VARCHAR(200),
    market_id       BIGINT        NOT NULL REFERENCES markets(id) ON DELETE RESTRICT,
    exchange        VARCHAR(50),
    industry        VARCHAR(100),
    sector          VARCHAR(100),
    market_cap      DECIMAL(20,2),
    currency        VARCHAR(10)   DEFAULT 'CNY',
    lot_size        INTEGER       DEFAULT 100,
    is_active       BOOLEAN       DEFAULT TRUE,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_stocks_symbol ON stocks(symbol);
CREATE INDEX IF NOT EXISTS idx_stocks_market_id ON stocks(market_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_stocks_symbol_market ON stocks(symbol, market_id);

-- ============================================================
-- User & Permissions
-- ============================================================

CREATE TABLE IF NOT EXISTS users (
    id              BIGSERIAL PRIMARY KEY,
    username        VARCHAR(50)   NOT NULL UNIQUE,
    email           VARCHAR(100)  NOT NULL UNIQUE,
    password_hash   VARCHAR(255)  NOT NULL,
    role            VARCHAR(20)   NOT NULL DEFAULT 'user'
        CHECK (role IN ('admin', 'user')),
    is_active       BOOLEAN       DEFAULT TRUE,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- ============================================================
-- Trading
-- ============================================================

-- Watchlists: user stock watchlists
CREATE TABLE IF NOT EXISTS watchlists (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    stock_id        BIGINT        NOT NULL REFERENCES stocks(id) ON DELETE CASCADE,
    added_at        TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, stock_id)
);

-- Orders: trading orders (both real and simulated)
CREATE TABLE IF NOT EXISTS orders (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    stock_id        BIGINT        NOT NULL REFERENCES stocks(id) ON DELETE RESTRICT,
    order_type      VARCHAR(10)   NOT NULL
        CHECK (order_type IN ('buy', 'sell')),
    order_kind      VARCHAR(10)   NOT NULL
        CHECK (order_kind IN ('market', 'limit')),
    price           DECIMAL(18,4),
    quantity        INTEGER       NOT NULL CHECK (quantity > 0),
    filled_quantity INTEGER       DEFAULT 0 CHECK (filled_quantity >= 0),
    status          VARCHAR(20)   NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'partial', 'filled', 'cancelled', 'rejected')),
    is_real         BOOLEAN       DEFAULT FALSE,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);
CREATE INDEX IF NOT EXISTS idx_orders_stock_id ON orders(stock_id);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at);

-- Positions: user holdings
CREATE TABLE IF NOT EXISTS positions (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    stock_id        BIGINT        NOT NULL REFERENCES stocks(id) ON DELETE RESTRICT,
    quantity        INTEGER       NOT NULL DEFAULT 0,
    avg_cost        DECIMAL(18,4),
    current_value   DECIMAL(20,2),
    unrealized_pnl  DECIMAL(20,2),
    realized_pnl    DECIMAL(20,2),
    is_real         BOOLEAN       DEFAULT FALSE,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, stock_id)
);

-- ============================================================
-- Simulated Trading
-- ============================================================

CREATE TABLE IF NOT EXISTS simulated_trades (
    id              BIGSERIAL PRIMARY KEY,
    stock_id        BIGINT        NOT NULL REFERENCES stocks(id) ON DELETE RESTRICT,
    trade_type      VARCHAR(10)   NOT NULL
        CHECK (trade_type IN ('buy', 'sell')),
    price           DECIMAL(18,4) NOT NULL,
    quantity        INTEGER       NOT NULL CHECK (quantity > 0),
    confidence      DECIMAL(5,2),
    prediction_id   BIGINT,
    reason          TEXT,
    executed_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_simulated_trades_stock_id ON simulated_trades(stock_id);
CREATE INDEX IF NOT EXISTS idx_simulated_trades_executed_at ON simulated_trades(executed_at);

-- ============================================================
-- AI/ML Predictions
-- ============================================================

CREATE TABLE IF NOT EXISTS predictions (
    id              BIGSERIAL PRIMARY KEY,
    stock_id        BIGINT        NOT NULL REFERENCES stocks(id) ON DELETE CASCADE,
    period          VARCHAR(20)   NOT NULL
        CHECK (period IN ('short', 'medium', 'long')),
    direction       VARCHAR(10)   NOT NULL
        CHECK (direction IN ('up', 'down', 'neutral')),
    confidence      DECIMAL(5,2),
    target_price    DECIMAL(18,4),
    factors         JSONB,
    model_version   VARCHAR(50),
    predicted_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    valid_until     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_predictions_stock_id ON predictions(stock_id);
CREATE INDEX IF NOT EXISTS idx_predictions_predicted_at ON predictions(predicted_at);
CREATE INDEX IF NOT EXISTS idx_predictions_period ON predictions(period);

-- Prediction accuracy tracking
CREATE TABLE IF NOT EXISTS prediction_accuracies (
    id                  BIGSERIAL PRIMARY KEY,
    stock_id            BIGINT        NOT NULL REFERENCES stocks(id) ON DELETE CASCADE,
    prediction_id       BIGINT        NOT NULL REFERENCES predictions(id) ON DELETE CASCADE,
    predicted_direction VARCHAR(10)   NOT NULL,
    actual_direction    VARCHAR(10)   NOT NULL,
    is_correct          BOOLEAN       NOT NULL,
    period              VARCHAR(20)   NOT NULL,
    evaluated_at        TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_prediction_accuracies_prediction_id ON prediction_accuracies(prediction_id);
CREATE INDEX IF NOT EXISTS idx_prediction_accuracies_evaluated_at ON prediction_accuracies(evaluated_at);

-- ============================================================
-- System Configuration
-- ============================================================

CREATE TABLE IF NOT EXISTS system_configs (
    id              BIGSERIAL PRIMARY KEY,
    key             VARCHAR(100)  NOT NULL UNIQUE,
    value           TEXT          NOT NULL,
    description     TEXT,
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- ============================================================
-- Audit Logging
-- ============================================================

CREATE TABLE IF NOT EXISTS audit_logs (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT        REFERENCES users(id) ON DELETE SET NULL,
    action          VARCHAR(50)   NOT NULL,
    resource        VARCHAR(50)   NOT NULL,
    resource_id     BIGINT,
    details         JSONB,
    ip_address      VARCHAR(45),
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);

-- ============================================================
-- TimescaleDB Hypertables (Time-Series Price Data)
-- ============================================================

-- 1-minute OHLCV bars
CREATE TABLE IF NOT EXISTS stock_prices_1min (
    time        TIMESTAMPTZ    NOT NULL,
    stock_id    BIGINT         NOT NULL,
    open        DECIMAL(18,4),
    high        DECIMAL(18,4),
    low         DECIMAL(18,4),
    close       DECIMAL(18,4),
    volume      BIGINT,
    amount      DECIMAL(20,2),
    UNIQUE (time, stock_id)
);

SELECT create_hypertable('stock_prices_1min', 'time', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_1min_stock_id ON stock_prices_1min(stock_id, time DESC);

-- Daily OHLCV bars with fundamental data
CREATE TABLE IF NOT EXISTS stock_prices_daily (
    time              TIMESTAMPTZ    NOT NULL,
    stock_id          BIGINT         NOT NULL,
    open              DECIMAL(18,4),
    high              DECIMAL(18,4),
    low               DECIMAL(18,4),
    close             DECIMAL(18,4),
    volume            BIGINT,
    amount            DECIMAL(20,2),
    turnover_rate     DECIMAL(10,4),
    pe_ratio          DECIMAL(10,2),
    pb_ratio          DECIMAL(10,2),
    total_market_cap  DECIMAL(20,2),
    float_market_cap  DECIMAL(20,2),
    UNIQUE (time, stock_id)
);

SELECT create_hypertable('stock_prices_daily', 'time', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_daily_stock_id ON stock_prices_daily(stock_id, time DESC);

-- Tick-level trade data
CREATE TABLE IF NOT EXISTS stock_prices_tick (
    time        TIMESTAMPTZ    NOT NULL,
    stock_id    BIGINT         NOT NULL,
    price       DECIMAL(18,4)  NOT NULL,
    volume      BIGINT,
    direction   VARCHAR(10)
        CHECK (direction IN ('buy', 'sell', 'neutral'))
);

SELECT create_hypertable('stock_prices_tick', 'time', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_tick_stock_id ON stock_prices_tick(stock_id, time DESC);

-- ============================================================
-- Seed Data: Markets
-- ============================================================

INSERT INTO markets (code, name, name_cn, country, currency, timezone, trading_hours) VALUES
('SSE',      'Shanghai Stock Exchange',      '上海证券交易所', 'China',         'CNY', 'Asia/Shanghai',   '09:30-15:00'),
('SZSE',     'Shenzhen Stock Exchange',      '深圳证券交易所', 'China',         'CNY', 'Asia/Shanghai',   '09:30-15:00'),
('BSE',      'Beijing Stock Exchange',       '北京证券交易所', 'China',         'CNY', 'Asia/Shanghai',   '09:30-15:00'),
('HKEX',     'Hong Kong Stock Exchange',     '香港交易所',     'Hong Kong',     'HKD', 'Asia/Hong_Kong',  '09:30-16:00'),
('NYSE',     'New York Stock Exchange',      '纽约证券交易所', 'United States',  'USD', 'America/New_York','09:30-16:00'),
('NASDAQ',   'NASDAQ Stock Market',          '纳斯达克',       'United States',  'USD', 'America/New_York','09:30-16:00'),
('AMEX',     'American Stock Exchange',      '美国证券交易所', 'United States',  'USD', 'America/New_York','09:30-16:00'),
('TSE',      'Tokyo Stock Exchange',         '东京证券交易所', 'Japan',          'JPY', 'Asia/Tokyo',      '09:00-15:00'),
('LSE',      'London Stock Exchange',        '伦敦证券交易所', 'United Kingdom', 'GBP', 'Europe/London',   '08:00-16:30'),
('Euronext', 'Euronext Stock Exchange',      '泛欧交易所',     'European Union', 'EUR', 'Europe/Paris',    '09:00-17:30'),
('Xetra',    'Xetra (Deutsche Börse)',       'Xetra交易所',    'Germany',        'EUR', 'Europe/Berlin',   '09:00-17:30'),
('ASX',      'Australian Securities Exchange','澳大利亚证券交易所','Australia',   'AUD', 'Australia/Sydney','10:00-16:00'),
('TSX',      'Toronto Stock Exchange',       '多伦多证券交易所','Canada',        'CAD', 'America/Toronto', '09:30-16:00'),
('KRX',      'Korea Exchange',               '韩国交易所',     'South Korea',    'KRW', 'Asia/Seoul',      '09:00-15:30')
ON CONFLICT (code) DO NOTHING;

-- ============================================================
-- Seed Data: Default Admin User
-- Password: admin123 (bcrypt placeholder - replace in production)
-- ============================================================

INSERT INTO users (username, email, password_hash, role, is_active) VALUES
('admin', 'admin@quant-trading.local',
 '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
 'admin', TRUE)
ON CONFLICT (username) DO NOTHING;

-- ============================================================
-- Seed Data: Default System Configurations
-- ============================================================

INSERT INTO system_configs (key, value, description) VALUES
('trading.default_commission_rate', '0.0003', 'Default commission rate (0.03%)'),
('trading.default_stamp_tax', '0.001', 'Default stamp tax rate (0.1%, A-shares only)'),
('trading.max_position_per_stock', '0.2', 'Maximum position ratio per single stock (20%)'),
('risk.max_daily_loss', '0.05', 'Maximum daily loss ratio before trading halt (5%)'),
('risk.max_drawdown', '0.15', 'Maximum drawdown before forced stop (15%)'),
('system.version', '1.0.0', 'Current system version'),
('system.maintenance_mode', 'false', 'Whether system is in maintenance mode')
ON CONFLICT (key) DO NOTHING;