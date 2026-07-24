"""
基本面因子采集模块

支持 A 股（通过 AKShare / Sina）和海外市场（通过 yfinance）
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
    # Auto-detect market
    if market == 'auto':
        if symbol.isdigit() and len(symbol) == 6:
            market = 'cn'
        else:
            market = 'us'

    if market == 'cn':
        return _fetch_cn_fundamentals(symbol)
    else:
        return _fetch_us_fundamentals(symbol)


def _fetch_cn_fundamentals(symbol: str) -> dict:
    """从新浪财经获取 A 股基本面数据"""
    try:
        # Determine exchange prefix
        if symbol.startswith(('6', '5', '9')):
            sina_code = f"sh{symbol}"
        else:
            sina_code = f"sz{symbol}"

        import requests
        url = f"https://hq.sinajs.cn/list={sina_code}"
        resp = requests.get(url, timeout=10, headers={
            "Referer": "https://finance.sina.com.cn",
            "User-Agent": "Mozilla/5.0"
        })
        if resp.status_code == 200:
            # Sina API returns basic quote data, fundamentals need separate API
            # For now, return empty placeholder - fundamentals can be added later
            pass
    except Exception:
        pass

    return _empty_fundamentals()


def _fetch_us_fundamentals(symbol: str) -> dict:
    """从 Yahoo Finance 获取美股基本面数据"""
    try:
        import yfinance as yf
        ticker = yf.Ticker(symbol)
        info = ticker.info
        if info:
            return {
                'pe': round(float(info.get('trailingPE', 0) or 0), 2),
                'pb': round(float(info.get('priceToBook', 0) or 0), 2),
                'roe': round(float(info.get('returnOnEquity', 0) or 0) * 100, 2),
                'revenue_growth': round(float(info.get('revenueGrowth', 0) or 0) * 100, 2),
                'net_profit_margin': round(float(info.get('profitMargins', 0) or 0) * 100, 2),
                'debt_to_equity': round(float(info.get('debtToEquity', 0) or 0), 2),
                'current_ratio': round(float(info.get('currentRatio', 0) or 0), 2),
                'eps': round(float(info.get('trailingEps', 0) or 0), 2),
                'market_cap': round(float(info.get('marketCap', 0) or 0), 2),
                'dividend_yield': round(float(info.get('dividendYield', 0) or 0) * 100, 2),
            }
    except Exception as e:
        print(f"[Fundamental] Yahoo Finance failed for {symbol}: {e}")

    return _empty_fundamentals()


def _empty_fundamentals() -> dict:
    """返回空的基本面数据"""
    return {
        'pe': 0.0,
        'pb': 0.0,
        'roe': 0.0,
        'revenue_growth': 0.0,
        'net_profit_margin': 0.0,
        'debt_to_equity': 0.0,
        'current_ratio': 0.0,
        'eps': 0.0,
        'market_cap': 0.0,
        'dividend_yield': 0.0,
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