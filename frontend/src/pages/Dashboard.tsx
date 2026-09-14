import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { Statistic, Card, Badge, Spin } from 'antd';
import { CaretUpOutlined, CaretDownOutlined, MinusOutlined } from '@ant-design/icons';
import MarketOverview from '@/components/MarketOverview';
import StockChart from '@/components/StockChart';
import DataState from '@/components/common/DataState';
import InfoTip from '@/components/common/InfoTip';
import { useDashboard } from '@/services/predictions';
import api from '@/services/api';
import useStockQuote from '@/hooks/useStockQuote';
import type { StockQuote, UnavailableSection } from '@/types';
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

function isUnavailable(section: unknown): section is UnavailableSection {
  return (
    !!section &&
    typeof section === 'object' &&
    (section as UnavailableSection).status === 'unavailable'
  );
}

const UNAVAILABLE_MSG = '该数据源暂不可用，已降级处理';

const DIRECTION_LABEL: Record<string, string> = { up: '看涨', down: '看跌', flat: '震荡' };

const PERIOD_LABEL: Record<string, string> = { short: '短期', medium: '中短期', long: '长期' };

function formatPercent(value: number | null | undefined) {
  return value === null || value === undefined ? '--' : `${(value * 100).toFixed(1)}%`;
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

  const { data: dashData, isLoading: dashLoading, refetch } = useDashboard();

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

  const highConfidence = dashData?.high_confidence;
  const accuracySummary = dashData?.accuracy_summary;
  const riskAlerts = dashData?.risk_alerts;
  const systemStatus = dashData?.system_status;

  const renderSystemStatus = () => {
    const isUnavail = isUnavailable(systemStatus);
    const status = systemStatus as Exclude<typeof systemStatus, UnavailableSection>;
    const body = () => {
      if (!status) return null;
      const health = status.model_health;
      return (
        <div className="dash-card-body">
          <div className="dash-sys-row">
            <span className="dash-sys-label"><InfoTip tip="股票列表最近一次成功同步的时间；超过 24 小时未同步会标记为数据陈旧并降级">最近同步</InfoTip></span>
            <span className="dash-sys-value">
              {status.last_sync ? new Date(status.last_sync).toLocaleString('zh-CN') : '--'}
            </span>
          </div>
          <div className="dash-sys-row">
            <span className="dash-sys-label"><InfoTip tip="ML 模型健康度；连续多日准确率低于阈值时自动暂停展示预测">模型状态</InfoTip></span>
            <span className="dash-sys-value">
              {health.suspend ? (
                <Badge status="error" text={`已暂停展示${health.reason ? `：${health.reason}` : ''}`} />
              ) : (
                <Badge status="success" text="续用中" />
              )}
            </span>
          </div>
          {health.suspend && (
            <div className="dash-sys-row">
              <span className="dash-sys-label">连续低于阈值</span>
              <span className="dash-sys-value">{health.consecutive_below_threshold ?? 0} 天</span>
            </div>
          )}
          <div className="dash-sys-row">
            <span className="dash-sys-label"><InfoTip tip="整体运行标记；任一数据源或同步异常时显示「降级运行」">系统状态</InfoTip></span>
            <span className="dash-sys-value">
              {status.degraded ? <Badge status="warning" text="降级运行" /> : <Badge status="success" text="正常" />}
            </span>
          </div>
        </div>
      );
    };

    return (
      <DataState
        loading={dashLoading}
        error={isUnavail ? UNAVAILABLE_MSG : null}
        onRetry={refetch}
        isEmpty={!isUnavail && !status}
        emptyText="暂无系统状态"
      >
        {body()}
      </DataState>
    );
  };

  return (
    <div className="dashboard">
      {/* 顶部：市场概览 */}
      <MarketOverview />

      {/* 决策支持卡片区 */}
      <section className="dashboard-support">
        <div className="dashboard-support-title">决策支持</div>
        <div className="dashboard-support-grid">
          <InfoTip title="今日高置信度标的" tip="按置信度降序展示当前待验证、且置信度 ≥ 阈值（默认 60%）的预测标的，帮助快速定位模型最看好的股票。点击条目可查看个股详情。">
          <Card title="今日高置信度标的" className="dash-card" bordered={false}>
            <DataState
              loading={dashLoading}
              error={isUnavailable(highConfidence) ? UNAVAILABLE_MSG : null}
              onRetry={refetch}
              isEmpty={!isUnavailable(highConfidence) && (!highConfidence || highConfidence.length === 0)}
              emptyText="暂无高置信度标的"
              emptyDescription="可切换预测周期或提交流行标的后再查看"
            >
              <ul className="dash-highconf-list">
                {(Array.isArray(highConfidence) ? highConfidence : []).map((it) => (
                  <li
                    key={it.symbol}
                    className="dash-highconf-item"
                    onClick={() => navigate(`/stocks/${it.symbol}`)}
                  >
                    <div className="dash-highconf-main">
                      <span className="dash-highconf-symbol">{it.symbol}</span>
                      <span className="dash-highconf-name">{it.name}</span>
                    </div>
                    <div className="dash-highconf-sub">
                      <span>{DIRECTION_LABEL[it.direction] ?? it.direction}</span>
                      <span className="dash-highconf-period">{PERIOD_LABEL[it.period] ?? it.period}</span>
                      <span className="dash-highconf-conf">{(it.confidence * 100).toFixed(0)}%</span>
                      {it.target_price !== null && it.target_price !== undefined && (
                        <span className="dash-highconf-price">¥{it.target_price.toFixed(2)}</span>
                      )}
                    </div>
                  </li>
                ))}
              </ul>
            </DataState>
          </Card>
          </InfoTip>

          <Card title={<InfoTip tip="展示预测模型的准确率统计：近 7 日、近 30 日、累计准确率与已评估样本数量。"><span>预测准确率摘要</span></InfoTip>} className="dash-card" bordered={false}>
            <DataState
              loading={dashLoading}
              error={isUnavailable(accuracySummary) ? UNAVAILABLE_MSG : null}
              onRetry={refetch}
              isEmpty={!isUnavailable(accuracySummary) && !accuracySummary}
              emptyText="暂无准确率数据"
              emptyDescription="历史样本积累到一定数量后自动展示"
            >
              {accuracySummary && !isUnavailable(accuracySummary) && (
                <div className="dash-acc-grid">
                  <InfoTip tip="近 7 天已到期预测中判定正确的比例 = 正确数 ÷ 已评估数 × 100%">
                  <div className="dash-acc-item">
                    <span className="dash-acc-label">近7日</span>
                    <span className="dash-acc-value">{formatPercent(accuracySummary.accuracy_7d)}</span>
                  </div>
                  </InfoTip>
                  <InfoTip tip="近 30 天已到期预测中判定正确的比例">
                  <div className="dash-acc-item">
                    <span className="dash-acc-label">近30日</span>
                    <span className="dash-acc-value">{formatPercent(accuracySummary.accuracy_30d)}</span>
                  </div>
                  </InfoTip>
                  <InfoTip tip="全部已评估预测的累计正确率">
                  <div className="dash-acc-item">
                    <span className="dash-acc-label">累计</span>
                    <span className="dash-acc-value">{formatPercent(accuracySummary.accuracy_total)}</span>
                  </div>
                  </InfoTip>
                  <InfoTip tip="已产生判定结果（正确/错误）的预测总数；样本过少时准确率参考意义有限">
                  <div className="dash-acc-item">
                    <span className="dash-acc-label">已评估样本</span>
                    <span className="dash-acc-value">
                      {accuracySummary.total_evaluated === null || accuracySummary.total_evaluated === undefined
                        ? '--'
                        : accuracySummary.total_evaluated}
                    </span>
                  </div>
                  </InfoTip>
                </div>
              )}
            </DataState>
          </Card>

          <Card title={<InfoTip tip="连续 3 次及以上判定错误的预测标的，以及命中异常波动的事件，用于提示模型近期失准风险。"><span>风险提示</span></InfoTip>} className="dash-card" bordered={false}>
            <DataState
              loading={dashLoading}
              error={isUnavailable(riskAlerts) ? UNAVAILABLE_MSG : null}
              onRetry={refetch}
              isEmpty={!isUnavailable(riskAlerts) && (!riskAlerts || riskAlerts.length === 0)}
              emptyText="暂无风险提示"
              emptyDescription="系统将就触发风控信号的标的给出风险提示"
            >
              <ul className="dash-risk-list">
                {(Array.isArray(riskAlerts) ? riskAlerts : []).map((alert, idx) => (
                  <li key={idx} className="dash-risk-item">
                    <span className="dash-risk-title">
                      {[alert.name, alert.symbol, alert.type].filter(Boolean).join(' · ') || '风险'}
                    </span>
                    <span className="dash-risk-msg">{alert.message || alert.content || '--'}</span>
                    {alert.level && <Badge status="warning" text={alert.level} />}
                  </li>
                ))}
              </ul>
            </DataState>
          </Card>

          <Card title="系统状态" className="dash-card" bordered={false}>
            {renderSystemStatus()}
          </Card>
        </div>
      </section>

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
                    <th className="col-right"><InfoTip tip="最新成交价">最新价</InfoTip></th>
                    <th className="col-right"><InfoTip tip="涨跌额 ÷ 昨收 × 100%">涨跌幅</InfoTip></th>
                    <th className="col-right"><InfoTip tip="现价 − 昨收">涨跌额</InfoTip></th>
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