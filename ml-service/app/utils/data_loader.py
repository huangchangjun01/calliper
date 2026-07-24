"""
数据加载工具
"""
import pandas as pd
import numpy as np
from typing import Optional
from datetime import datetime, timedelta
from sqlalchemy import create_engine, text
import os


class DataLoader:
    """股票和市场数据加载器"""

    def __init__(self, db_url: Optional[str] = None):
        """
        初始化数据加载器

        Args:
            db_url: 数据库连接 URL，默认从环境变量 DATABASE_URL 读取
        """
        self.db_url = db_url or os.getenv('DATABASE_URL', '')
        self._engine = None

    @property
    def engine(self):
        if self._engine is None and self.db_url:
            self._engine = create_engine(self.db_url)
        return self._engine

    def load_stock_data(
        self,
        symbol: str,
        start: str,
        end: str,
        interval: str = '1d'
    ) -> pd.DataFrame:
        """
        从数据库或 API 加载股票数据

        Args:
            symbol: 股票代码
            start: 起始日期 'YYYY-MM-DD'
            end: 结束日期 'YYYY-MM-DD'
            interval: 数据周期 '1d', '1h', '30m' 等

        Returns:
            包含 OHLCV 数据的 DataFrame，列: open, high, low, close, volume
        """
        if self.engine is not None:
            return self._load_from_db(symbol, start, end, interval)
        return self._load_from_api(symbol, start, end, interval)

    def _load_from_db(
        self,
        symbol: str,
        start: str,
        end: str,
        interval: str = '1d'
    ) -> pd.DataFrame:
        """从 PostgreSQL 数据库加载数据"""
        table = 'stock_daily' if interval == '1d' else 'stock_minute'
        query = text(f"""
            SELECT trade_date, open, high, low, close, volume, amount
            FROM {table}
            WHERE symbol = :symbol
              AND trade_date BETWEEN :start AND :end
            ORDER BY trade_date
        """)
        with self.engine.connect() as conn:
            df = pd.read_sql_query(
                query,
                conn,
                params={'symbol': symbol, 'start': start, 'end': end},
                parse_dates=['trade_date']
            )
        if df.empty:
            return df
        df.set_index('trade_date', inplace=True)
        df.index.name = 'date'
        return df

    def _load_from_api(
        self,
        symbol: str,
        start: str,
        end: str,
        interval: str = '1d'
    ) -> pd.DataFrame:
        """
        从外部 API 加载数据（yfinance / akshare / sina）

        尝试从真实 API 获取数据，失败时返回空 DataFrame
        """
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

        url = f"https://money.finance.sina.com.cn/quotes_service/api/json_v2.php/CN_MarketData.getKLineData?symbol={sina_code}&scale=240&ma=no&datalen=100"
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
        if self.engine is not None:
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
        with self.engine.connect() as conn:
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