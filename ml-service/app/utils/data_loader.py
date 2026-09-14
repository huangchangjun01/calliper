"""
数据加载工具

真实数据优先：TSDB（calliper_tsdb 行情库） → 真实外部 API（yfinance / sina）。
任何情况下都不生成合成数据；无真实数据时返回空 DataFrame。
"""
import pandas as pd
import numpy as np
from typing import Optional
from datetime import datetime, timedelta
from sqlalchemy import create_engine, text
import os
import re


class DataLoader:
    """股票和市场数据加载器"""

    def __init__(self, db_url: Optional[str] = None, tsdb_url: Optional[str] = None):
        """
        初始化数据加载器

        Args:
            db_url: 主数据库（calliper_trading，含 stocks 表 symbol→id）连接 URL，
                默认从环境变量 DATABASE_URL 读取
            tsdb_url: 时序数据库（calliper_tsdb，含 stock_prices_daily/stock_prices_1min）
                连接 URL，默认从环境变量 TSDB_URL 读取
        """
        self.db_url = self._normalize_url(db_url or os.getenv('DATABASE_URL', ''))
        self.tsdb_url = self._normalize_url(tsdb_url or os.getenv('TSDB_URL', ''))
        self._main_engine = None
        self._tsdb_engine = None

    @staticmethod
    def _normalize_url(url: str) -> str:
        """SQLAlchemy 2.x 不识别 postgres://，统一归一化为 postgresql://。"""
        return re.sub(r'^postgres://', 'postgresql://', url)

    @property
    def engine(self):
        """主数据库引擎（stocks 等业务表）"""
        return self.main_engine

    @property
    def main_engine(self):
        if self._main_engine is None and self.db_url:
            self._main_engine = create_engine(self.db_url)
        return self._main_engine

    @property
    def tsdb_engine(self):
        if self._tsdb_engine is None and self.tsdb_url:
            self._tsdb_engine = create_engine(self.tsdb_url)
        return self._tsdb_engine

    def load_stock_data(
        self,
        symbol: str,
        start: str,
        end: str,
        interval: str = '1d'
    ) -> pd.DataFrame:
        """
        加载股票数据：TSDB 优先 → 真实 API → 空 DataFrame（绝不合成数据）

        Args:
            symbol: 股票代码
            start: 起始日期 'YYYY-MM-DD'
            end: 结束日期 'YYYY-MM-DD'
            interval: 数据周期 '1d', '1h', '30m' 等

        Returns:
            包含 OHLCV 数据的 DataFrame，列: open, high, low, close, volume
        """
        # 1) 优先从 TSDB 读取真实行情
        df = self._load_from_tsdb(symbol, start, end, interval)
        if df is not None and not df.empty:
            return df

        # 2) 回退到真实外部 API（yfinance / sina），失败返回空 DataFrame
        return self._load_from_api(symbol, start, end, interval)

    def _resolve_stock_id(self, symbol: str) -> Optional[int]:
        """从主数据库 stocks 表解析 symbol → stock_id，找不到返回 None"""
        if self.main_engine is None:
            return None
        query = text("SELECT id FROM stocks WHERE symbol = :symbol LIMIT 1")
        with self.main_engine.connect() as conn:
            row = conn.execute(query, {"symbol": symbol}).fetchone()
        if row is None:
            return None
        return int(row[0])

    def _load_from_tsdb(
        self,
        symbol: str,
        start: str,
        end: str,
        interval: str = '1d'
    ) -> pd.DataFrame:
        """
        从 TSDB (calliper_tsdb) 加载真实行情数据。

        interval '1d' → stock_prices_daily；其他周期 → stock_prices_1min。
        无法解析 stock_id 或无数据时返回空 DataFrame。
        """
        if self.tsdb_engine is None:
            return pd.DataFrame()

        try:
            stock_id = self._resolve_stock_id(symbol)
            if stock_id is None:
                print(f"[DataLoader] No stock_id found for {symbol} in main DB, TSDB skip")
                return pd.DataFrame()

            table = 'stock_prices_daily' if interval == '1d' else 'stock_prices_1min'
            query = text(f"""
                SELECT time, open, high, low, close, volume
                FROM {table}
                WHERE stock_id = :stock_id
                  AND time BETWEEN :start AND :end
                ORDER BY time
            """)
            with self.tsdb_engine.connect() as conn:
                df = pd.read_sql_query(
                    query,
                    conn,
                    params={'stock_id': stock_id, 'start': start, 'end': end},
                    parse_dates=['time']
                )

            if df.empty:
                return pd.DataFrame()

            for col in ['open', 'high', 'low', 'close']:
                df[col] = df[col].astype(float)
            df['volume'] = df['volume'].astype(float)
            df = df.set_index('time')
            df.index.name = 'date'
            return df[['open', 'high', 'low', 'close', 'volume']]
        except Exception as e:
            print(f"[DataLoader] TSDB load failed for {symbol}: {e}")
            return pd.DataFrame()

    def _load_from_api(
        self,
        symbol: str,
        start: str,
        end: str,
        interval: str = '1d'
    ) -> pd.DataFrame:
        """
        从外部 API 加载数据（yfinance / sina）

        A 股 6 位数字代码直接走新浪财经（yfinance 无法解析纯数字 A 股代码，
        避免无效请求）；其余代码（美股/指数）走 yfinance。
        失败时返回空 DataFrame。
        """
        # A 股数字代码：跳过 yfinance，直接走新浪（失败自动返回空）
        if interval == '1d' and symbol.isdigit() and len(symbol) == 6:
            return self._load_from_sina(symbol, start, end)

        # Try Yahoo Finance API first
        try:
            import yfinance as yf
            ticker = yf.Ticker(symbol)
            df = ticker.history(start=start, end=end, interval=interval)
            if not df.empty:
                # Rename columns to standard format
                df = df.rename(columns={
                    'Open': 'open', 'High': 'high', 'Low': 'low',
                    'Close': 'close', 'Volume': 'volume'
                })
                df.index.name = 'date'
                return df
        except Exception as e:
            print(f"[DataLoader] Yahoo Finance failed for {symbol}: {e}")

        # Try Sina Finance API for A-shares
        try:
            if symbol.isdigit() and len(symbol) == 6:
                return self._load_from_sina(symbol, start, end)
        except Exception as e:
            print(f"[DataLoader] Sina Finance failed for {symbol}: {e}")

        # Return empty DataFrame if all APIs fail
        return pd.DataFrame()

    def _load_from_sina(self, symbol: str, start: str, end: str) -> pd.DataFrame:
        """从新浪财经 API 加载 A 股数据"""
        import requests

        # Determine exchange prefix
        if symbol.startswith(('6', '5', '9')):
            sina_code = f"sh{symbol}"
        else:
            sina_code = f"sz{symbol}"

        # 近 2500 根日线（约 10 年），保证兜底数据能超过训练脚本 MIN_ROWS(120) 阈值，
        # 避免因默认 100 根被判定数据不足而跳过该股票。
        url = f"https://money.finance.sina.com.cn/quotes_service/api/json_v2.php/CN_MarketData.getKLineData?symbol={sina_code}&scale=240&ma=no&datalen=2500"
        try:
            resp = requests.get(url, timeout=10, headers={
                "Referer": "https://finance.sina.com.cn",
                "User-Agent": "Mozilla/5.0"
            })
            if resp.status_code != 200:
                return pd.DataFrame()

            data = resp.json()
            if not data:
                return pd.DataFrame()

            records = []
            for item in data:
                records.append({
                    'date': item.get('day', ''),
                    'open': float(item.get('open', 0)),
                    'high': float(item.get('high', 0)),
                    'low': float(item.get('low', 0)),
                    'close': float(item.get('close', 0)),
                    'volume': float(item.get('volume', 0)),
                })

            df = pd.DataFrame(records)
            if not df.empty:
                df['date'] = pd.to_datetime(df['date'])
                df.set_index('date', inplace=True)
                df.index.name = 'date'
                # Filter by date range
                df = df[(df.index >= start) & (df.index <= end)]
            return df
        except Exception:
            return pd.DataFrame()

    def load_market_data(
        self,
        market_code: str,
        start: str,
        end: str
    ) -> pd.DataFrame:
        """
        加载市场指数数据

        Args:
            market_code: 市场指数代码，如 '000001.SH'（上证指数）、'^GSPC'（标普500）
            start: 起始日期
            end: 结束日期

        Returns:
            包含指数 OHLCV 数据的 DataFrame
        """
        if self.main_engine is not None:
            return self._load_market_from_db(market_code, start, end)
        return self._load_from_api(market_code, start, end)

    def _load_market_from_db(
        self,
        market_code: str,
        start: str,
        end: str
    ) -> pd.DataFrame:
        """从数据库加载市场指数数据"""
        query = text("""
            SELECT trade_date, open, high, low, close, volume, amount
            FROM market_index
            WHERE index_code = :code
              AND trade_date BETWEEN :start AND :end
            ORDER BY trade_date
        """)
        with self.main_engine.connect() as conn:
            df = pd.read_sql_query(
                query,
                conn,
                params={'code': market_code, 'start': start, 'end': end},
                parse_dates=['trade_date']
            )
        if not df.empty:
            df.set_index('trade_date', inplace=True)
            df.index.name = 'date'
        return df

    def load_batch(
        self,
        symbols: list[str],
        start: str,
        end: str,
        interval: str = '1d'
    ) -> dict[str, pd.DataFrame]:
        """
        批量加载多只股票数据

        Args:
            symbols: 股票代码列表
            start: 起始日期
            end: 结束日期
            interval: 数据周期

        Returns:
            {symbol: DataFrame} 的字典
        """
        return {
            symbol: self.load_stock_data(symbol, start, end, interval)
            for symbol in symbols
        }
