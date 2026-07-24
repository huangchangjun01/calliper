-- 000001_init_schema.down.sql
-- Quantitative Trading System - Rollback Initial Schema

-- Drop hypertables first (TimescaleDB requirement)
DROP TABLE IF EXISTS stock_prices_tick CASCADE;
DROP TABLE IF EXISTS stock_prices_daily CASCADE;
DROP TABLE IF EXISTS stock_prices_1min CASCADE;

-- Drop standard tables in dependency order
DROP TABLE IF EXISTS audit_logs CASCADE;
DROP TABLE IF EXISTS system_configs CASCADE;
DROP TABLE IF EXISTS prediction_accuracies CASCADE;
DROP TABLE IF EXISTS predictions CASCADE;
DROP TABLE IF EXISTS simulated_trades CASCADE;
DROP TABLE IF EXISTS positions CASCADE;
DROP TABLE IF EXISTS orders CASCADE;
DROP TABLE IF EXISTS watchlists CASCADE;
DROP TABLE IF EXISTS stocks CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS markets CASCADE;

-- Drop extensions (optional - may be used by other databases)
-- DROP EXTENSION IF EXISTS timescaledb CASCADE;
-- DROP EXTENSION IF EXISTS "uuid-ossp" CASCADE;