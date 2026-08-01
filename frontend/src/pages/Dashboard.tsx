import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { Spin, Statistic } from 'antd';
import { CaretUpOutlined, CaretDownOutlined, MinusOutlined } from '@ant-design/icons';
import MarketOverview from '@/components/MarketOverview';
import StockChart from '@/components/StockChart';
import api from '@/services/api';
import useStockQuote from '@/hooks/useStockQuote';
import type { StockQuote } from '@/types';
import '@/pages/Dashboard.css';

interface WatchlistItem {
  symbol: string;
  name: string;
  stock?: { name?: string };
}

interface MarketStatistics {
  limitUpCount: number;
  limitDownCount: number;
  upCount: number;
  downCount: number;
  flatCount: number;
  totalAmount: number;
}

export default function Dashboard() {
  const navigate = useNavigate();
  const [watchlist, setWatchlist] = useState<WatchlistItem[]>([]);
  const [watchlistLoading, setWatchlistLoading] = useState(true);
  const [statistics, setStatistics] = useState<MarketStatistics | null>(null);
  const [selectedSymbol, setSelectedSymbol] = useState<string | null>(null);
  const [flashingSymbols, setFlashingSymbols] = useState<Set<string>>(new Set());

  const watchlistSymbols = watchlist.map((w) => w.symbol);
  const { stocks, changedSymbols } = useStockQuote(watchlistSymbols);

  // 获取自选股列表
  useEffect(() => {
    api.get<WatchlistItem[]>('/stocks/watchlist')
      .then((data) => {
        const items = data.map((item) => ({
          symbol: item.symbol,
          name: item.stock?.name || item.symbol,
        }));
        setWatchlist(items);
        if (items.length > 0 && !selectedSymbol) {
          setSelectedSymbol(items[0].symbol);
        }
      })
      .catch(() => {
        // API 请求失败，显示空列表
        setWatchlist([]);
      })
      .finally(() => setWatchlistLoading(false));
  }, []);

  // 获取涨跌统计
  useEffect(() => {
    api.get<MarketStatistics>('/market/statistics')
      .then(setStatistics)
      .catch(() => {
        // API 请求失败，置空统计
        setStatistics(null);
      });
  }, []);

  // 价格变化闪烁动画
  useEffect(() => {
    if (changedSymbols.size === 0) return;
    setFlashingSymbols(new Set(changedSymbols));
    const timer = setTimeout(() => setFlashingSymbols(new Set()), 500);
    return () => clearTimeout(timer);
  }, [changedSymbols]);

  const handleRowClick = useCallback(
    (symbol: string) => {
      setSelectedSymbol(symbol);
    },
    []
  );

  const handleSymbolDoubleClick = useCallback(
    (symbol: string) => {
      navigate(`/stocks/${symbol}`);
    },
    [navigate]
  );

  const formatAmount = (amount: number) => {
    if (amount >= 1e12) return `${(amount / 1e12).toFixed(2)}万亿`;
    if (amount >= 1e8) return `${(amount / 1e8).toFixed(2)}亿`;
    if (amount >= 1e4) return `${(amount / 1e4).toFixed(2)}万`;
    return amount.toFixed(0);
  };

  const getStockQuote = (symbol: string): StockQuote | undefined => {
    return stocks.get(symbol);
  };

  return (
    <div className="dashboard">
      {/* 顶部：市场概览 */}
      <MarketOverview />

      {/* 中部：自选股 + 图表 */}
      <div className="dashboard-middle">
        {/* 左侧：自选股列表 */}
        <div className="dashboard-watchlist">
          <div className="dashboard-watchlist-header">
            <span className="dashboard-watchlist-title">自选股</span>
            <span className="dashboard-watchlist-count">{watchlist.length} 只</span>
          </div>
          <div className="dashboard-watchlist-table">
            {watchlistLoading ? (
              <div className="dashboard-watchlist-loading">
                <Spin size="small" />
              </div>
            ) : (
              <table>
                <thead>
                  <tr>
                    <th>代码</th>
                    <th>名称</th>
                    <th className="col-right">最新价</th>
                    <th className="col-right">涨跌幅</th>
                    <th className="col-right">涨跌额</th>
                  </tr>
                </thead>
                <tbody>
                  {watchlist.map((item) => {
                    const quote = getStockQuote(item.symbol);
                    const isFlashing = flashingSymbols.has(item.symbol);
                    const isSelected = selectedSymbol === item.symbol;
                    const changeClass =
                      quote && quote.change > 0
                        ? 'up'
                        : quote && quote.change < 0
                        ? 'down'
                        : 'flat';

                    return (
                      <tr
                        key={item.symbol}
                        className={`${isSelected ? 'row-selected' : ''} ${isFlashing ? 'row-flash' : ''}`}
                        onClick={() => handleRowClick(item.symbol)}
                        onDoubleClick={() => handleSymbolDoubleClick(item.symbol)}
                      >
                        <td className="col-symbol">{item.symbol}</td>
                        <td className="col-name">{item.name}</td>
                        <td className={`col-right col-price ${changeClass}`}>
                          {quote ? quote.price.toFixed(2) : '--'}
                        </td>
                        <td className={`col-right ${changeClass}`}>
                          {quote ? (
                            <span className="change-cell">
                              {quote.changePercent > 0 ? '+' : ''}
                              {quote.changePercent.toFixed(2)}%
                            </span>
                          ) : (
                            '--'
                          )}
                        </td>
                        <td className={`col-right ${changeClass}`}>
                          {quote ? (
                            <span className="change-cell">
                              {quote.change > 0 ? '+' : ''}
                              {quote.change.toFixed(2)}
                            </span>
                          ) : (
                            '--'
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </div>
        </div>

        {/* 右侧：图表 */}
        <div className="dashboard-chart">
          {selectedSymbol ? (
            <StockChart symbol={selectedSymbol} interval="1d" height={380} />
          ) : (
            <div className="dashboard-chart-empty">
              <span>请选择自选股查看图表</span>
            </div>
          )}
        </div>
      </div>

      {/* 底部：涨跌统计 */}
      <div className="dashboard-statistics">
        <div className="dashboard-statistics-title">市场统计</div>
        <div className="dashboard-statistics-grid">
          <div className="dashboard-stat-item">
            <Statistic
              title="涨停"
              value={statistics?.limitUpCount ?? '--'}
              valueStyle={{ color: 'var(--color-error)', fontSize: 24, fontWeight: 700 }}
              prefix={<CaretUpOutlined />}
            />
          </div>
          <div className="dashboard-stat-item">
            <Statistic
              title="跌停"
              value={statistics?.limitDownCount ?? '--'}
              valueStyle={{ color: 'var(--color-success)', fontSize: 24, fontWeight: 700 }}
              prefix={<CaretDownOutlined />}
            />
          </div>
          <div className="dashboard-stat-item">
            <Statistic
              title="上涨"
              value={statistics?.upCount ?? '--'}
              valueStyle={{ color: 'var(--color-error)', fontSize: 24, fontWeight: 700 }}
              prefix={<CaretUpOutlined />}
            />
          </div>
          <div className="dashboard-stat-item">
            <Statistic
              title="下跌"
              value={statistics?.downCount ?? '--'}
              valueStyle={{ color: 'var(--color-success)', fontSize: 24, fontWeight: 700 }}
              prefix={<CaretDownOutlined />}
            />
          </div>
          <div className="dashboard-stat-item">
            <Statistic
              title="平盘"
              value={statistics?.flatCount ?? '--'}
              valueStyle={{ fontSize: 24, fontWeight: 700 }}
              prefix={<MinusOutlined />}
            />
          </div>
          <div className="dashboard-stat-item">
            <Statistic
              title="成交额"
              value={statistics ? formatAmount(statistics.totalAmount) : '--'}
              valueStyle={{ fontSize: 24, fontWeight: 700 }}
            />
          </div>
        </div>
      </div>
    </div>
  );
}