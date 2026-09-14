-- 000003_add_stocks_symbol_unique.up.sql
-- stocks.symbol 需要唯一约束以支持批量幂等 upsert（ON CONFLICT (symbol)）。
-- 表内已无重复 symbol（如有重复需先清理再执行）。

CREATE UNIQUE INDEX IF NOT EXISTS idx_stocks_symbol ON stocks (symbol);