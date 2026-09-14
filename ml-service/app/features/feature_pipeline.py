"""
特征工程管道

组合技术指标、市场情绪因子、基本面因子，构建完整特征集。
支持标准化、缺失值处理、异常值处理等预处理步骤。
"""
import json
import os
from datetime import datetime, timedelta

import pandas as pd
import numpy as np
from typing import Optional
from sklearn.preprocessing import StandardScaler

from .technical_indicators import (
    calc_ma, calc_macd, calc_rsi, calc_kdj,
    calc_bollinger, calc_atr, calc_obv
)
from .market_sentiment import (
    calc_money_flow, calc_money_flow_ratio,
    calc_turnover_signal, calc_volume_ratio, calc_vwap
)
from .fundamental import fetch_fundamentals, fundamentals_to_series
from .feature_store import FeatureStore
from ..utils.data_loader import DataLoader


class FeaturePipeline:
    """特征工程管道"""

    def __init__(self):
        self.indicators: list[str] = []       # 已计算的技术指标名称列表
        self.scalers: dict[str, StandardScaler] = {}  # 按特征名存储标准化器
        self._feature_names: list[str] = []   # 缓存的特征名称列表
        self._data_loader: Optional[DataLoader] = None
        self._feature_store: Optional['FeatureStore'] = None

    def _get_data_loader(self) -> DataLoader:
        """惰性创建真实数据加载器（绝不合成数据）。"""
        if self._data_loader is None:
            self._data_loader = DataLoader()
        return self._data_loader

    def _get_feature_store(self) -> Optional['FeatureStore']:
        """惰性创建特征存储（无 DATABASE_URL 时为 None）。"""
        db_url = os.getenv("DATABASE_URL", "")
        if self._feature_store is None and db_url:
            try:
                self._feature_store = FeatureStore(db_url)
            except Exception as e:
                print(f"[FeaturePipeline] feature store init skipped: {e}")
                self._feature_store = None
        return self._feature_store

    def compute_features(self, symbol: str, history_days: int = 120) -> dict:
        """
        基于真实行情为单只股票计算最新特征快照，返回 {feature_name: float}。

        组合 build_features / add_fundamental_features 并从真实数据源加载 OHLCV，
        取最新一日的特征值。无真实数据时返回空 dict（绝不合成）。
        """
        loader = self._get_data_loader()
        end = datetime.now()
        start = end - timedelta(days=history_days)
        df = loader.load_stock_data(
            symbol, start.strftime("%Y-%m-%d"), end.strftime("%Y-%m-%d"), interval="1d"
        )
        if df is None or df.empty:
            print(f"[FeaturePipeline] No real data for {symbol}, returning empty features")
            return {}

        features = self.build_features(df.copy())
        features = self.add_fundamental_features(features, symbol)

        last = features.iloc[-1]
        result: dict = {}
        for name, value in last.items():
            try:
                if pd.isna(value):
                    result[name] = 0.0
                else:
                    result[name] = float(value)
            except (TypeError, ValueError):
                result[name] = 0.0

        # 若特征存储可用，则持久化当次快照
        store = self._get_feature_store()
        if store is not None:
            try:
                idx = last.name
                if isinstance(idx, (pd.Timestamp, np.datetime64)):
                    trade_date = pd.Timestamp(idx).strftime("%Y-%m-%d")
                else:
                    trade_date = datetime.now().strftime("%Y-%m-%d")
                store.save_features(symbol, trade_date, result)
            except Exception as e:
                print(f"[FeaturePipeline] feature store save skipped for {symbol}: {e}")

        return result

    def get_feature_history(self, symbol: str, limit: int = 10) -> list:
        """
        从特征存储读取历史特征记录。

        返回 [{symbol, computed_at, feature_count, missing_count}]，
        存储不可用或无记录时返回空列表。
        """
        store = self._get_feature_store()
        if store is None:
            return []
        try:
            from sqlalchemy import text
            query = text("""
                SELECT trade_date, features
                FROM ml_features
                WHERE symbol = :symbol
                ORDER BY trade_date DESC
                LIMIT :limit
            """)
            with store.engine.connect() as conn:
                rows = conn.execute(query, {"symbol": symbol, "limit": limit}).fetchall()

            history = []
            for r in reversed(rows):
                feat_data = r[1]
                if isinstance(feat_data, str):
                    feat_data = json.loads(feat_data)
                missing = 0
                for v in feat_data.values():
                    if v is None:
                        missing += 1
                    elif isinstance(v, float) and v != v:
                        missing += 1
                date_str = pd.Timestamp(r[0]).strftime("%Y-%m-%d") if r[0] else ""
                history.append({
                    "symbol": symbol,
                    "computed_at": date_str,
                    "feature_count": len(feat_data),
                    "missing_count": missing,
                })
            return history[:limit]
        except Exception as e:
            print(f"[FeaturePipeline] get_feature_history failed for {symbol}: {e}")
            return []

    def build_features(self, df: pd.DataFrame) -> pd.DataFrame:
        """
        构建完整特征集

        在原始 DataFrame 上计算所有技术指标、市场情绪因子和基本面因子

        Args:
            df: 包含 OHLCV 数据的 DataFrame，必须包含 open, high, low, close, volume 列

        Returns:
            包含所有特征的 DataFrame
        """
        features = pd.DataFrame(index=df.index)

        # 1. 计算所有技术指标
        features = self._add_technical_indicators(df, features)

        # 2. 添加市场情绪因子
        features = self._add_market_sentiment(df, features)

        # 3. 添加价格相关特征
        features = self._add_price_features(df, features)

        self._feature_names = list(features.columns)
        self.indicators = self._feature_names.copy()
        return features

    def _add_technical_indicators(self, df: pd.DataFrame, features: pd.DataFrame) -> pd.DataFrame:
        """添加技术指标"""
        # 移动平均线
        ma_df = calc_ma(df, col='close', periods=[5, 10, 20, 60])
        for col in ma_df.columns:
            features[col] = ma_df[col]

        # MACD
        macd_df = calc_macd(df, col='close')
        for col in macd_df.columns:
            features[col] = macd_df[col]

        # RSI
        features['rsi'] = calc_rsi(df, col='close', period=14)

        # KDJ
        if 'high' in df.columns and 'low' in df.columns:
            kdj_df = calc_kdj(df, high='high', low='low', close='close', n=9)
            for col in kdj_df.columns:
                features[col] = kdj_df[col]

        # 布林带
        boll_df = calc_bollinger(df, col='close', period=20, std=2)
        for col in boll_df.columns:
            features[col] = boll_df[col]

        # ATR
        if 'high' in df.columns and 'low' in df.columns:
            features['atr'] = calc_atr(df, high='high', low='low', close='close', period=14)

        # OBV
        if 'volume' in df.columns:
            features['obv'] = calc_obv(df, close='close', volume='volume')

        return features

    def _add_market_sentiment(self, df: pd.DataFrame, features: pd.DataFrame) -> pd.DataFrame:
        """添加市场情绪因子"""
        if 'volume' in df.columns:
            # 资金流向
            if 'high' in df.columns and 'low' in df.columns:
                features['money_flow'] = calc_money_flow(
                    df, close='close', volume='volume', high='high', low='low'
                )
                features['mfi'] = calc_money_flow_ratio(
                    df, close='close', volume='volume', high='high', low='low', period=14
                )

            # 量比
            features['volume_ratio'] = calc_volume_ratio(df, volume='volume', period=5)

            # 换手率信号
            features['turnover_signal'] = calc_turnover_signal(df, volume='volume')

            # VWAP
            if 'high' in df.columns and 'low' in df.columns:
                features['vwap'] = calc_vwap(
                    df, high='high', low='low', close='close', volume='volume'
                )

        return features

    def _add_price_features(self, df: pd.DataFrame, features: pd.DataFrame) -> pd.DataFrame:
        """添加价格相关衍生特征"""
        # 收益率
        features['returns'] = df['close'].pct_change()

        # 对数收益率
        features['log_returns'] = np.log(df['close'] / df['close'].shift(1))

        # 价格波动率（滚动20日）
        features['volatility_20'] = features['returns'].rolling(window=20).std()

        # 价格与各均线的偏离度
        for p in [5, 10, 20, 60]:
            ma_col = f'ma_{p}'
            if ma_col in features.columns:
                features[f'price_deviation_{p}'] = (df['close'] - features[ma_col]) / features[ma_col]

        # 多空排列特征
        if 'ma_5' in features.columns and 'ma_20' in features.columns:
            features['ma_alignment'] = (features['ma_5'] - features['ma_20']).apply(
                lambda x: 1 if x > 0 else -1
            )

        return features

    def preprocess(self, df: pd.DataFrame, fit: bool = False) -> pd.DataFrame:
        """
        预处理：标准化、缺失值处理、异常值处理

        Args:
            df: 特征 DataFrame
            fit: 是否拟合标准化器（训练时设为 True，预测时设为 False）

        Returns:
            预处理后的 DataFrame
        """
        result = df.copy()

        # 1. 缺失值填充
        result = self._fill_missing(result)

        # 2. 异常值处理（winsorize 99%）
        result = self._winsorize(result)

        # 3. 标准化
        result = self._standardize(result, fit=fit)

        return result

    def _fill_missing(self, df: pd.DataFrame) -> pd.DataFrame:
        """缺失值填充：前向填充 + 均值填充"""
        result = df.copy()

        # 先向前填充（处理连续缺失）
        result = result.ffill()

        # 剩余缺失值用均值填充
        for col in result.columns:
            if result[col].isna().any():
                col_mean = result[col].mean()
                if pd.isna(col_mean):
                    col_mean = 0.0
                result[col] = result[col].fillna(col_mean)

        return result

    def _winsorize(self, df: pd.DataFrame, lower: float = 0.01, upper: float = 0.99) -> pd.DataFrame:
        """
        异常值处理：Winsorize 截尾

        将超出分位数范围的值截断到边界值

        Args:
            df: 特征 DataFrame
            lower: 下分位数
            upper: 上分位数

        Returns:
            Winsorize 后的 DataFrame
        """
        result = df.copy()
        for col in result.columns:
            lo = result[col].quantile(lower)
            hi = result[col].quantile(upper)
            result[col] = result[col].clip(lower=lo, upper=hi)
        return result

    def _standardize(self, df: pd.DataFrame, fit: bool = False) -> pd.DataFrame:
        """
        标准化处理

        Args:
            df: 特征 DataFrame
            fit: 是否拟合标准化器

        Returns:
            标准化后的 DataFrame
        """
        result = df.copy()
        for col in result.columns:
            if fit:
                scaler = StandardScaler()
                values = result[col].values.reshape(-1, 1)
                result[col] = scaler.fit_transform(values).flatten()
                self.scalers[col] = scaler
            else:
                if col in self.scalers:
                    values = result[col].values.reshape(-1, 1)
                    result[col] = self.scalers[col].transform(values).flatten()
        return result

    def get_feature_names(self) -> list[str]:
        """返回特征名称列表"""
        if not self._feature_names:
            return self.indicators
        return self._feature_names

    def add_fundamental_features(self, df: pd.DataFrame, symbol: str) -> pd.DataFrame:
        """
        添加基本面因子到特征 DataFrame

        Args:
            df: 特征 DataFrame
            symbol: 股票代码

        Returns:
            添加了基本面因子的特征 DataFrame
        """
        fundamentals = fetch_fundamentals(symbol)
        flat = fundamentals_to_series(fundamentals)

        for key, value in flat.items():
            df[key] = value

        self._feature_names = list(df.columns)
        return df