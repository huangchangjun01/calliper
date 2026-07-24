"""
特征工程模块

提供技术指标计算、市场情绪因子、基本面因子采集和特征工程管道。
"""
from .technical_indicators import (
    calc_ma,
    calc_macd,
    calc_rsi,
    calc_kdj,
    calc_bollinger,
    calc_atr,
    calc_obv,
)
from .market_sentiment import (
    calc_money_flow,
    calc_money_flow_ratio,
    calc_turnover_signal,
    calc_volume_ratio,
    calc_advance_decline_ratio,
    calc_vwap,
)
from .fundamental import (
    fetch_fundamentals,
    fetch_fundamentals_batch,
    fundamentals_to_series,
)
from .feature_pipeline import FeaturePipeline
from .feature_store import FeatureStore

__all__ = [
    # 技术指标
    'calc_ma',
    'calc_macd',
    'calc_rsi',
    'calc_kdj',
    'calc_bollinger',
    'calc_atr',
    'calc_obv',
    # 市场情绪
    'calc_money_flow',
    'calc_money_flow_ratio',
    'calc_turnover_signal',
    'calc_volume_ratio',
    'calc_advance_decline_ratio',
    'calc_vwap',
    # 基本面
    'fetch_fundamentals',
    'fetch_fundamentals_batch',
    'fundamentals_to_series',
    # 管道与存储
    'FeaturePipeline',
    'FeatureStore',
]