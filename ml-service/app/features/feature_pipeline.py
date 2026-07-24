"""
特征工程管道

组合技术指标、市场情绪因子、基本面因子，构建完整特征集。
支持标准化、缺失值处理、异常值处理等预处理步骤。
"""
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


class FeaturePipeline:
    """特征工程管道"""

    def __init__(self):
        self.indicators: list[str] = []       # 已计算的技术指标名称列表
        self.scalers: dict[str, StandardScaler] = {}  # 按特征名存储标准化器
        self._feature_names: list[str] = []   # 缓存的特征名称列表

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