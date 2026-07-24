-- Convert tables to TimescaleDB hypertables
-- Note: These tables should be created on the TimescaleDB instance

SELECT create_hypertable('market_quotes', 'time', if_not_exists => TRUE);
SELECT create_hypertable('predictions', 'time', if_not_exists => TRUE);
SELECT create_hypertable('prediction_evaluations', 'time', if_not_exists => TRUE);

-- Create indexes on hypertables
CREATE INDEX IF NOT EXISTS idx_market_quotes_stock_time ON market_quotes (stock_code_id, time DESC);
CREATE INDEX IF NOT EXISTS idx_predictions_stock_horizon ON predictions (stock_code_id, horizon, time DESC);
CREATE INDEX IF NOT EXISTS idx_prediction_evaluations_stock ON prediction_evaluations (stock_code_id, time DESC);