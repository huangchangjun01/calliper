"""
市场情绪因子计算模块
"""
import pandas as pd
import numpy as np
from typing import Optional


def calc_money_flow(df: pd.DataFrame, close: str = 'close', volume: str = 'volume',
                    high: str = 'high', low: str = 'low') -> pd.Series:
    """
    计算资金流向 (Money Flow)

    基于典型价格和成交量计算资金流向指标。
    正值表示资金流入，负值表示资金流出。

    Args:
        df: 包含 OHLCV 数据的 DataFrame
        close: 收盘价列名
        volume: 成交量列名
        high: 最高价列名
        low: 最低价列名

    Returns:
        资金流向 Series
    """
    typical_price = (df[high] + df[low] + df[close]) / 3
    raw_money_flow = typical_price * df[volume]

    price_diff = typical_price.diff()
    money_flow = np.where(price_diff > 0, raw_money_flow,
                          np.where(price_diff < 0, -raw_money_flow, 0))
    result = pd.Series(money_flow, index=df.index, name='money_flow')
    return result


def calc_money_flow_ratio(df: pd.DataFrame, period: int = 14,
                          close: str = 'close', volume: str = 'volume',
                          high: str = 'high', low: str = 'low') -> pd.Series:
    """
    计算资金流量比率 (MFI - Money Flow Index)

    Args:
        df: 包含 OHLCV 数据的 DataFrame
        period: 计算周期
        close: 收盘价列名
        volume: 成交量列名
        high: 最高价列名
        low: 最低价列名

    Returns:
        MFI 值 Series
    """
    typical_price = (df[high] + df[low] + df[close]) / 3
    raw_money_flow = typical_price * df[volume]

    price_diff = typical_price.diff()
    positive_flow = np.where(price_diff > 0, raw_money_flow, 0)
    negative_flow = np.where(price_diff < 0, raw_money_flow, 0)

    positive_flow = pd.Series(positive_flow, index=df.index)
    negative_flow = pd.Series(negative_flow, index=df.index)

    pos_sum = positive_flow.rolling(window=period).sum()
    neg_sum = negative_flow.rolling(window=period).sum()

    mfi = 100 - (100 / (1 + pos_sum / neg_sum.replace(0, np.nan)))
    mfi.name = 'mfi'
    return mfi


def calc_turnover_signal(df: pd.DataFrame, volume: str = 'volume',
                         float_shares: Optional[pd.Series] = None) -> pd.Series:
    """
    计算换手率信号

    换手率 = 成交量 / 流通股本

    Args:
        df: 包含成交量数据的 DataFrame
        volume: 成交量列名
        float_shares: 流通股本 Series（与 df 同索引），若为 None 则用成交量变化率替代

    Returns:
        换手率信号 Series
    """
    if float_shares is not None:
        turnover = df[volume] / float_shares
    else:
        # 无流通股本数据时，使用成交量变化率作为替代
        turnover = df[volume].pct_change()

    # 标准化为信号：使用过去20日均值进行归一化
    turnover_ma = turnover.rolling(window=20).mean()
    turnover_std = turnover.rolling(window=20).std().replace(0, np.nan)
    signal = (turnover - turnover_ma) / turnover_std
    signal.name = 'turnover_signal'
    return signal


def calc_volume_ratio(df: pd.DataFrame, volume: str = 'volume',
                      period: int = 5) -> pd.Series:
    """
    计算量比

    量比 = 当前成交量 / 过去 N 日平均成交量

    Args:
        df: 包含成交量数据的 DataFrame
        volume: 成交量列名
        period: 对比周期

    Returns:
        量比 Series
    """
    ma_volume = df[volume].shift(1).rolling(window=period).mean()
    volume_ratio = df[volume] / ma_volume.replace(0, np.nan)
    volume_ratio.name = 'volume_ratio'
    return volume_ratio


def calc_advance_decline_ratio(df: pd.DataFrame, adv: str = 'advancing',
                               dec: str = 'declining') -> pd.Series:
    """
    计算涨跌比 (ADR)

    Args:
        df: 包含涨跌家数数据的 DataFrame
        adv: 上涨家数列名
        dec: 下跌家数列名

    Returns:
        涨跌比 Series
    """
    adr = df[adv] / df[dec].replace(0, np.nan)
    adr.name = 'adr'
    return adr


def calc_vwap(df: pd.DataFrame, high: str = 'high', low: str = 'low',
              close: str = 'close', volume: str = 'volume') -> pd.Series:
    """
    计算成交量加权平均价格 (VWAP)

    Args:
        df: 包含 OHLCV 数据的 DataFrame
        high: 最高价列名
        low: 最低价列名
        close: 收盘价列名
        volume: 成交量列名

    Returns:
        VWAP Series
    """
    typical_price = (df[high] + df[low] + df[close]) / 3
    cum_vp = (typical_price * df[volume]).cumsum()
    cum_volume = df[volume].cumsum()
    vwap = cum_vp / cum_volume.replace(0, np.nan)
    vwap.name = 'vwap'
    return vwap