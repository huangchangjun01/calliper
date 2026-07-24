"""
技术指标计算模块
纯 Python/Pandas 实现，不依赖 TA-Lib
"""
import pandas as pd
import numpy as np
from typing import Optional


def calc_ma(df: pd.DataFrame, col: str = 'close', periods: list[int] = [5, 10, 20, 60]) -> pd.DataFrame:
    """
    计算移动平均线

    Args:
        df: 包含价格数据的 DataFrame
        col: 用于计算均线的列名
        periods: 均线周期列表

    Returns:
        包含各周期 MA 列的 DataFrame
    """
    result = pd.DataFrame(index=df.index)
    for p in periods:
        result[f'ma_{p}'] = df[col].rolling(window=p).mean()
    return result


def calc_macd(
    df: pd.DataFrame,
    col: str = 'close',
    fast: int = 12,
    slow: int = 26,
    signal: int = 9
) -> pd.DataFrame:
    """
    计算 MACD 指标

    Args:
        df: 包含价格数据的 DataFrame
        col: 用于计算的价格列名
        fast: 快线 EMA 周期
        slow: 慢线 EMA 周期
        signal: 信号线 EMA 周期

    Returns:
        包含 DIF、DEA、MACD 的 DataFrame
    """
    ema_fast = df[col].ewm(span=fast, adjust=False).mean()
    ema_slow = df[col].ewm(span=slow, adjust=False).mean()
    dif = ema_fast - ema_slow
    dea = dif.ewm(span=signal, adjust=False).mean()
    macd_bar = 2 * (dif - dea)

    result = pd.DataFrame({
        'dif': dif,
        'dea': dea,
        'macd': macd_bar,
    }, index=df.index)
    return result


def calc_rsi(df: pd.DataFrame, col: str = 'close', period: int = 14) -> pd.Series:
    """
    计算 RSI 相对强弱指标

    Args:
        df: 包含价格数据的 DataFrame
        col: 用于计算的价格列名
        period: RSI 周期

    Returns:
        RSI 值 Series
    """
    delta = df[col].diff()
    gain = delta.clip(lower=0)
    loss = (-delta).clip(lower=0)

    avg_gain = gain.ewm(alpha=1 / period, adjust=False).mean()
    avg_loss = loss.ewm(alpha=1 / period, adjust=False).mean()

    rs = avg_gain / avg_loss.replace(0, np.nan)
    rsi = 100 - (100 / (1 + rs))
    rsi.name = 'rsi'
    return rsi


def calc_kdj(
    df: pd.DataFrame,
    high: str = 'high',
    low: str = 'low',
    close: str = 'close',
    n: int = 9
) -> pd.DataFrame:
    """
    计算 KDJ 指标

    Args:
        df: 包含 OHLC 数据的 DataFrame
        high: 最高价列名
        low: 最低价列名
        close: 收盘价列名
        n: RSV 周期

    Returns:
        包含 K、D、J 的 DataFrame
    """
    lowest_low = df[low].rolling(window=n).min()
    highest_high = df[high].rolling(window=n).max()

    rsv = ((df[close] - lowest_low) / (highest_high - lowest_low).replace(0, np.nan)) * 100

    K = rsv.copy()
    D = rsv.copy()
    for i in range(1, len(rsv)):
        if pd.isna(K.iloc[i]):
            K.iloc[i] = 50.0
        else:
            K.iloc[i] = 2 / 3 * K.iloc[i - 1] + 1 / 3 * K.iloc[i]
        if pd.isna(D.iloc[i]):
            D.iloc[i] = 50.0
        else:
            D.iloc[i] = 2 / 3 * D.iloc[i - 1] + 1 / 3 * D.iloc[i]

    # 过滤掉初始未收敛的值
    mask = rsv.isna()
    K = K.where(~mask, np.nan)
    D = D.where(~mask, np.nan)

    J = 3 * K - 2 * D

    result = pd.DataFrame({
        'K': K,
        'D': D,
        'J': J,
    }, index=df.index)
    return result


def calc_bollinger(
    df: pd.DataFrame,
    col: str = 'close',
    period: int = 20,
    std: int = 2
) -> pd.DataFrame:
    """
    计算布林带

    Args:
        df: 包含价格数据的 DataFrame
        col: 用于计算的价格列名
        period: 中轨 MA 周期
        std: 标准差倍数

    Returns:
        包含 upper、middle、lower 的 DataFrame
    """
    middle = df[col].rolling(window=period).mean()
    rolling_std = df[col].rolling(window=period).std()
    upper = middle + std * rolling_std
    lower = middle - std * rolling_std

    result = pd.DataFrame({
        'upper': upper,
        'middle': middle,
        'lower': lower,
    }, index=df.index)
    return result


def calc_atr(
    df: pd.DataFrame,
    high: str = 'high',
    low: str = 'low',
    close: str = 'close',
    period: int = 14
) -> pd.Series:
    """
    计算平均真实波幅 (ATR)

    Args:
        df: 包含 OHLC 数据的 DataFrame
        high: 最高价列名
        low: 最低价列名
        close: 收盘价列名
        period: ATR 周期

    Returns:
        ATR 值 Series
    """
    prev_close = df[close].shift(1)
    tr1 = df[high] - df[low]
    tr2 = (df[high] - prev_close).abs()
    tr3 = (df[low] - prev_close).abs()
    true_range = pd.concat([tr1, tr2, tr3], axis=1).max(axis=1)

    atr = true_range.ewm(alpha=1 / period, adjust=False).mean()
    atr.name = 'atr'
    return atr


def calc_obv(
    df: pd.DataFrame,
    close: str = 'close',
    volume: str = 'volume'
) -> pd.Series:
    """
    计算能量潮指标 (OBV)

    Args:
        df: 包含价格和成交量数据的 DataFrame
        close: 收盘价列名
        volume: 成交量列名

    Returns:
        OBV 值 Series
    """
    price_diff = df[close].diff()
    direction = np.where(price_diff > 0, 1, np.where(price_diff < 0, -1, 0))
    obv = (direction * df[volume]).cumsum()
    obv.name = 'obv'
    return obv