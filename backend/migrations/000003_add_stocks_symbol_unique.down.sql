-- 000003_add_stocks_symbol_unique.down.sql
-- 回滚：移除 stocks.symbol 的唯一约束（同时移除同名非唯一索引重建）。

DROP INDEX IF EXISTS idx_stocks_symbol;
CREATE INDEX IF NOT EXISTS idx_stocks_symbol ON stocks (symbol);