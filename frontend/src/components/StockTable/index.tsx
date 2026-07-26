import { useNavigate } from 'react-router-dom';
import type { StockSearchItem } from '@/types';
import styles from './index.module.css';

interface StockTableProps {
  stocks: StockSearchItem[];
  loading: boolean;
  onRowClick?: (symbol: string) => void;
}

function formatMarketCap(marketCap: number | undefined | null): string {
  if (marketCap == null || marketCap <= 0) return '-';
  if (marketCap >= 1e12) {
    return (marketCap / 1e12).toFixed(2) + '万亿';
  }
  if (marketCap >= 1e8) {
    return (marketCap / 1e8).toFixed(2) + '亿';
  }
  if (marketCap >= 1e4) {
    return (marketCap / 1e4).toFixed(2) + '万';
  }
  return marketCap.toFixed(0);
}

function getChangeClass(changePercent: number): string {
  if (changePercent > 0) return styles.up;
  if (changePercent < 0) return styles.down;
  return styles.flat;
}

function formatChangePercent(changePercent: number): string {
  const prefix = changePercent > 0 ? '+' : '';
  return `${prefix}${changePercent.toFixed(2)}%`;
}

function getExchangeLabel(exchange: string): string {
  const map: Record<string, string> = {
    SSE: '上交所',
    SZSE: '深交所',
    BSE: '北交所',
    HKEX: '港交所',
    NYSE: '纽交所',
    NASDAQ: '纳斯达克',
    AMEX: '美交所',
    TSE: '东京',
    LSE: '伦敦',
    Euronext: '泛欧',
    Xetra: '法兰克福',
    ASX: '澳洲',
    TSX: '多伦多',
    KRX: '韩国',
  };
  return map[exchange] || exchange;
}

export default function StockTable({ stocks, loading, onRowClick }: StockTableProps) {
  const navigate = useNavigate();

  const handleRowClick = (symbol: string) => {
    if (onRowClick) {
      onRowClick(symbol);
    } else {
      navigate(`/stocks/${symbol}`);
    }
  };

  if (loading) {
    return (
      <div className={styles.tableWrapper}>
        <table className={styles.table}>
          <thead>
            <tr>
              <th>代码</th>
              <th>名称</th>
              <th>市场</th>
              <th>行业</th>
              <th>市值</th>
              <th>最新价</th>
              <th>涨跌幅</th>
            </tr>
          </thead>
          <tbody>
            {Array.from({ length: 8 }).map((_, i) => (
              <tr key={i} className={styles.skeletonRow}>
                <td><span className={styles.skeleton} style={{ width: 60 }} /></td>
                <td><span className={styles.skeleton} style={{ width: 80 }} /></td>
                <td><span className={styles.skeleton} style={{ width: 50 }} /></td>
                <td><span className={styles.skeleton} style={{ width: 70 }} /></td>
                <td><span className={styles.skeleton} style={{ width: 60 }} /></td>
                <td><span className={styles.skeleton} style={{ width: 50 }} /></td>
                <td><span className={styles.skeleton} style={{ width: 50 }} /></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    );
  }

  if (stocks.length === 0) {
    return (
      <div className={styles.empty}>
        <span className={styles.emptyIcon}>
          <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <circle cx="11" cy="11" r="8" />
            <line x1="21" y1="21" x2="16.65" y2="16.65" />
          </svg>
        </span>
        <p className={styles.emptyText}>暂无搜索结果</p>
        <p className={styles.emptyHint}>请尝试调整搜索关键词或切换市场分类</p>
      </div>
    );
  }

  return (
    <div className={styles.tableWrapper}>
      <table className={styles.table}>
        <thead>
          <tr>
            <th>代码</th>
            <th>名称</th>
            <th>市场</th>
            <th>行业</th>
            <th>市值</th>
            <th>最新价</th>
            <th>涨跌幅</th>
          </tr>
        </thead>
        <tbody>
          {stocks.map((stock) => (
            <tr
              key={stock.symbol}
              className={styles.row}
              onClick={() => handleRowClick(stock.symbol)}
            >
              <td className={styles.symbol}>{stock.symbol}</td>
              <td className={styles.name}>{stock.name}</td>
              <td className={styles.exchange}>{getExchangeLabel(stock.exchange)}</td>
              <td className={styles.industry}>{stock.industry || '-'}</td>
              <td className={styles.marketCap}>{formatMarketCap(stock.marketCap)}</td>
              <td className={styles.price}>{stock.price?.toFixed(2) ?? '-'}</td>
              <td className={`${styles.change} ${getChangeClass(stock.changePercent)}`}>
                {stock.changePercent !== undefined ? formatChangePercent(stock.changePercent) : '-'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}