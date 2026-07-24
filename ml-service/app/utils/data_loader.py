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
        从外部 API 加载数据（yfinance / akshare）

        当前为 mock 实现，返回符合规格的随机数据
        """
        start_dt = datetime.strptime(start, '%Y-%m-%d')
        end_dt = datetime.strptime(end, '%Y-%m-%d')
        dates = pd.date_range(start=start_dt, end=end_dt, freq='B')

        n = len(dates)
        base_price = 100.0
        rng = np.random.default_rng(hash(symbol) % (2**32))

        # 生成带趋势的随机价格
        returns = rng.normal(0.0005, 0.015, n)
        prices = base_price * np.cumprod(1 + returns)

        open_prices = prices * (1 + rng.normal(0, 0.003, n))
        high_prices = np.maximum(open_prices, prices) * (1 + np.abs(rng.normal(0, 0.005, n)))
        low_prices = np.minimum(open_prices, prices) * (1 - np.abs(rng.normal(0, 0.005, n)))
        volumes = rng.integers(1_000_000, 50_000_000, n)

        df = pd.DataFrame({
            'open': open_prices,
            'high': high_prices,
            'low': low_prices,
            'close': prices,
            'volume': volumes,
        }, index=dates)

        df.index.name = 'date'
        return df

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