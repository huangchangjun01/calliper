"""
特征存储

将构建好的特征持久化到 PostgreSQL，支持按股票代码和日期范围查询。
"""
import json
import pandas as pd
import numpy as np
from typing import Optional
from sqlalchemy import create_engine, text, Table, Column, MetaData, JSON, String, Date, Float
from datetime import datetime


class FeatureStore:
    """特征存储管理器"""

    def __init__(self, db_url: str):
        """
        初始化特征存储

        Args:
            db_url: PostgreSQL 数据库连接 URL
        """
        self.db_url = db_url
        self.engine = create_engine(db_url)
        self.metadata = MetaData()

        # 定义特征表
        self.features_table = Table(
            'ml_features',
            self.metadata,
            Column('id', String(64), primary_key=True),
            Column('symbol', String(32), nullable=False, index=True),
            Column('trade_date', Date, nullable=False, index=True),
            Column('features', JSON, nullable=False),
            Column('created_at', String(32), nullable=False),
        )

        self._ensure_tables()

    def _ensure_tables(self):
        """确保特征表存在"""
        self.metadata.create_all(self.engine, checkfirst=True)

    def save_features(self, symbol: str, date: str, features: dict) -> None:
        """
        保存特征到 PostgreSQL

        Args:
            symbol: 股票代码
            date: 交易日期 'YYYY-MM-DD'
            features: 特征字典
        """
        record_id = f"{symbol}_{date}"

        # 将 numpy 类型转换为 Python 原生类型
        serializable = {}
        for k, v in features.items():
            if isinstance(v, (np.integer,)):
                serializable[k] = int(v)
            elif isinstance(v, (np.floating,)):
                serializable[k] = float(v)
            elif isinstance(v, np.ndarray):
                serializable[k] = v.tolist()
            elif pd.isna(v):
                serializable[k] = None
            else:
                serializable[k] = v

        now = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
        features_json = json.dumps(serializable, ensure_ascii=False)

        with self.engine.begin() as conn:
            conn.execute(
                text("DELETE FROM ml_features WHERE id = :id"),
                {'id': record_id}
            )
            conn.execute(
                text("""
                    INSERT INTO ml_features (id, symbol, trade_date, features, created_at)
                    VALUES (:id, :symbol, :trade_date, :features, :created_at)
                """),
                {
                    'id': record_id,
                    'symbol': symbol,
                    'trade_date': date,
                    'features': features_json,
                    'created_at': now,
                }
            )

    def save_features_batch(self, records: list[dict]) -> None:
        """
        批量保存特征

        Args:
            records: 记录列表，每条包含 symbol, date, features
        """
        for record in records:
            self.save_features(
                symbol=record['symbol'],
                date=record['date'],
                features=record['features'],
            )

    def load_features(
        self,
        symbol: str,
        start_date: str,
        end_date: str
    ) -> pd.DataFrame:
        """
        从数据库加载特征

        Args:
            symbol: 股票代码
            start_date: 起始日期 'YYYY-MM-DD'
            end_date: 结束日期 'YYYY-MM-DD'

        Returns:
            特征 DataFrame，index 为 trade_date，列为各特征
        """
        query = text("""
            SELECT trade_date, features
            FROM ml_features
            WHERE symbol = :symbol
              AND trade_date BETWEEN :start_date AND :end_date
            ORDER BY trade_date
        """)

        with self.engine.connect() as conn:
            rows = conn.execute(query, {
                'symbol': symbol,
                'start_date': start_date,
                'end_date': end_date,
            }).fetchall()

        if not rows:
            return pd.DataFrame()

        records = []
        for row in rows:
            record = {'trade_date': row[0]}
            features_data = row[1]
            if isinstance(features_data, str):
                features_data = json.loads(features_data)
            record.update(features_data)
            records.append(record)

        df = pd.DataFrame(records)
        df['trade_date'] = pd.to_datetime(df['trade_date'])
        df.set_index('trade_date', inplace=True)
        df.index.name = 'date'
        return df

    def get_latest_features(self, symbol: str) -> Optional[dict]:
        """
        获取最新特征

        Args:
            symbol: 股票代码

        Returns:
            最新特征字典，若无数据返回 None
        """
        query = text("""
            SELECT features
            FROM ml_features
            WHERE symbol = :symbol
            ORDER BY trade_date DESC
            LIMIT 1
        """)

        with self.engine.connect() as conn:
            row = conn.execute(query, {'symbol': symbol}).fetchone()

        if row is None:
            return None
        features_data = row[0]
        if isinstance(features_data, str):
            features_data = json.loads(features_data)
        return dict(features_data)

    def get_feature_history(
        self,
        symbol: str,
        feature_names: list[str],
        start_date: str,
        end_date: str
    ) -> pd.DataFrame:
        """
        获取指定特征的历史数据

        Args:
            symbol: 股票代码
            feature_names: 特征名称列表
            start_date: 起始日期
            end_date: 结束日期

        Returns:
            仅包含指定特征的 DataFrame
        """
        df = self.load_features(symbol, start_date, end_date)
        if df.empty:
            return df
        available = [f for f in feature_names if f in df.columns]
        return df[available]

    def delete_features(self, symbol: str, before_date: Optional[str] = None) -> int:
        """
        删除特征数据

        Args:
            symbol: 股票代码
            before_date: 删除该日期之前的数据，若为 None 则删除全部

        Returns:
            删除的记录数
        """
        if before_date:
            query = text("""
                DELETE FROM ml_features
                WHERE symbol = :symbol AND trade_date < :before_date
            """)
        else:
            query = text("""
                DELETE FROM ml_features
                WHERE symbol = :symbol
            """)

        with self.engine.begin() as conn:
            result = conn.execute(query, {
                'symbol': symbol,
                'before_date': before_date,
            })
            return result.rowcount

    def list_symbols(self) -> list[str]:
        """列出所有已存储特征的股票代码"""
        query = text("SELECT DISTINCT symbol FROM ml_features ORDER BY symbol")
        with self.engine.connect() as conn:
            rows = conn.execute(query).fetchall()
        return [row[0] for row in rows]