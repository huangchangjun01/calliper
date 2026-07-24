"""
基本面因子采集模块

支持 A 股（通过 AKShare）和海外市场（通过 yfinance）
当前实现为 mock 返回，后续可接入真实数据源
"""
from typing import Optional


def fetch_fundamentals(symbol: str, market: str = 'auto') -> dict:
    """
    采集基本面因子

    Args:
        symbol: 股票代码，如 '600519'（A股）或 'AAPL'（美股）
        market: 市场类型，'auto' 自动识别，'cn' A股，'us' 美股

    Returns:
        包含基本面因子的字典：
        - pe: 市盈率
        - pb: 市净率
        - roe: 净资产收益率
        - revenue_growth: 营收增长率
        - net_profit_margin: 净利润率
        - debt_to_equity: 资产负债率
        - current_ratio: 流动比率
        - eps: 每股收益
        - market_cap: 总市值
        - dividend_yield: 股息率
    """
    # TODO: 接入真实数据源
    # - A 股: 通过 akshare 获取
    # - 海外: 通过 yfinance 获取
    return _mock_fundamentals(symbol, market)


def _mock_fundamentals(symbol: str, market: str = 'auto') -> dict:
    """Mock 基本面数据"""
    # 根据 symbol 特征生成一致的 mock 数据
    seed = sum(ord(c) for c in symbol)
    import numpy as np
    rng = np.random.default_rng(seed)

    return {
        'pe': round(float(rng.uniform(8, 60)), 2),
        'pb': round(float(rng.uniform(0.5, 10)), 2),
        'roe': round(float(rng.uniform(2, 35)), 2),
        'revenue_growth': round(float(rng.uniform(-20, 50)), 2),
        'net_profit_margin': round(float(rng.uniform(-5, 40)), 2),
        'debt_to_equity': round(float(rng.uniform(10, 200)), 2),
        'current_ratio': round(float(rng.uniform(0.5, 5)), 2),
        'eps': round(float(rng.uniform(-2, 20)), 2),
        'market_cap': round(float(rng.uniform(1e9, 1e12)), 2),
        'dividend_yield': round(float(rng.uniform(0, 6)), 2),
    }


def fetch_fundamentals_batch(symbols: list[str], market: str = 'auto') -> dict[str, dict]:
    """
    批量采集基本面因子

    Args:
        symbols: 股票代码列表
        market: 市场类型

    Returns:
        {symbol: {fundamentals_dict}} 的字典
    """
    return {symbol: fetch_fundamentals(symbol, market) for symbol in symbols}


def fundamentals_to_series(fundamentals: dict) -> dict:
    """
    将基本面字典转换为可附加到 DataFrame 的 Series 格式

    Args:
        fundamentals: fetch_fundamentals 返回的字典

    Returns:
        扁平化特征字典，key 为特征名，value 为标量值
    """
    return {
        f'fund_{k}': v
        for k, v in fundamentals.items()
    }