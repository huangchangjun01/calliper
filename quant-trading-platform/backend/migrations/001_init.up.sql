-- ============================================
-- Business Tables (PostgreSQL)
-- ============================================

-- Users table
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Stock codes table (global market stocks)
CREATE TABLE stock_codes (
    id SERIAL PRIMARY KEY,
    code VARCHAR(20) NOT NULL,
    name VARCHAR(200) NOT NULL,
    exchange VARCHAR(20) NOT NULL,       -- SSE, SZSE, NYSE, NASDAQ, etc.
    market VARCHAR(20) NOT NULL,         -- CN, HK, US, EU, JP, etc.
    sector VARCHAR(100),                 -- Industry sector
    currency VARCHAR(10) DEFAULT 'USD',
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(code, exchange)
);

CREATE INDEX idx_stock_codes_market ON stock_codes(market);
CREATE INDEX idx_stock_codes_exchange ON stock_codes(exchange);
CREATE INDEX idx_stock_codes_name_trgm ON stock_codes USING gin (name gin_trgm_ops);
CREATE INDEX idx_stock_codes_code_trgm ON stock_codes USING gin (code gin_trgm_ops);

-- Watchlists
CREATE TABLE watchlists (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    stock_code_id INTEGER NOT NULL REFERENCES stock_codes(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, stock_code_id)
);

-- Trading accounts
CREATE TABLE trading_accounts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_type VARCHAR(10) NOT NULL CHECK (account_type IN ('real', 'simulated')),
    initial_balance DECIMAL(18, 4) NOT NULL DEFAULT 100000.0000,
    current_balance DECIMAL(18, 4) NOT NULL DEFAULT 100000.0000,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, account_type)
);

-- Positions
CREATE TABLE positions (
    id SERIAL PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES trading_accounts(id) ON DELETE CASCADE,
    stock_code_id INTEGER NOT NULL REFERENCES stock_codes(id),
    quantity DECIMAL(18, 4) NOT NULL,
    avg_cost DECIMAL(18, 4) NOT NULL,
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(account_id, stock_code_id)
);

-- Orders
CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES trading_accounts(id) ON DELETE CASCADE,
    stock_code_id INTEGER NOT NULL REFERENCES stock_codes(id),
    order_type VARCHAR(10) NOT NULL CHECK (order_type IN ('buy', 'sell')),
    order_kind VARCHAR(10) NOT NULL CHECK (order_kind IN ('market', 'limit')),
    quantity DECIMAL(18, 4) NOT NULL,
    price DECIMAL(18, 4),
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'filled', 'partial', 'cancelled', 'rejected')),
    filled_quantity DECIMAL(18, 4) DEFAULT 0,
    filled_price DECIMAL(18, 4),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_orders_account ON orders(account_id);
CREATE INDEX idx_orders_status ON orders(status);

-- Risk rules
CREATE TABLE risk_rules (
    id SERIAL PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES trading_accounts(id) ON DELETE CASCADE,
    rule_type VARCHAR(30) NOT NULL,
    rule_value VARCHAR(100) NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Blacklist
CREATE TABLE blacklist_stocks (
    id SERIAL PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES trading_accounts(id) ON DELETE CASCADE,
    stock_code_id INTEGER NOT NULL REFERENCES stock_codes(id),
    reason VARCHAR(200),
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(account_id, stock_code_id)
);

-- Trading session (market hours)
CREATE TABLE trading_sessions (
    id SERIAL PRIMARY KEY,
    exchange VARCHAR(20) NOT NULL,
    market VARCHAR(20) NOT NULL,
    open_time TIME NOT NULL,
    close_time TIME NOT NULL,
    timezone VARCHAR(50) NOT NULL DEFAULT 'UTC',
    lunch_start TIME,
    lunch_end TIME,
    trading_days VARCHAR(20) DEFAULT '1,2,3,4,5'  -- Mon-Fri by default
);

-- ============================================
-- Time-Series Tables (will be converted to hypertables on TimescaleDB)
-- These are created in the same PostgreSQL but TimescaleDB extension handles them
-- ============================================

-- Market quotes hypertable
CREATE TABLE market_quotes (
    time TIMESTAMPTZ NOT NULL,
    stock_code_id INTEGER NOT NULL,
    open DECIMAL(18, 4),
    high DECIMAL(18, 4),
    low DECIMAL(18, 4),
    close DECIMAL(18, 4),
    volume BIGINT,
    amount DECIMAL(24, 4),
    change_pct DECIMAL(10, 4)
);

-- Predictions hypertable
CREATE TABLE predictions (
    time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    stock_code_id INTEGER NOT NULL REFERENCES stock_codes(id),
    model_version VARCHAR(50) NOT NULL,
    horizon VARCHAR(10) NOT NULL CHECK (horizon IN ('short', 'medium', 'long')),
    direction VARCHAR(10) NOT NULL CHECK (direction IN ('up', 'down', 'flat')),
    confidence DECIMAL(5, 2) NOT NULL,
    action VARCHAR(10) NOT NULL CHECK (action IN ('buy', 'sell', 'hold')),
    key_factors TEXT,
    valid_until TIMESTAMPTZ
);

-- Prediction evaluations hypertable
CREATE TABLE prediction_evaluations (
    time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    prediction_id INTEGER NOT NULL,
    stock_code_id INTEGER NOT NULL,
    horizon VARCHAR(10) NOT NULL,
    predicted_direction VARCHAR(10) NOT NULL,
    actual_direction VARCHAR(10),
    was_correct BOOLEAN,
    confidence DECIMAL(5, 2),
    evaluated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Enable extensions
CREATE EXTENSION IF NOT EXISTS pg_trgm;