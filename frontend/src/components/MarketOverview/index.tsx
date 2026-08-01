import { useState, useEffect, useCallback } from 'react';
import { Spin } from 'antd';
import { CaretUpOutlined, CaretDownOutlined, MinusOutlined } from '@ant-design/icons';
import api from '@/services/api';
import type { StockQuote } from '@/types';
import './index.css';

interface MarketOverviewProps {
  className?: string;
}

const DEFAULT_INDICES: string[] = [
  '000001.SH',
  '399001.SZ',
  'HSI',
  'SPX',
  'IXIC',
];

const INDEX_NAMES: Record<string, string> = {
  '000001.SH': '上证指数',
  '399001.SZ': '深证成指',
  HSI: '恒生指数',
  SPX: '标普500',
  IXIC: '纳斯达克',
};

export default function MarketOverview({ className = '' }: MarketOverviewProps) {
  const [indices, setIndices] = useState<StockQuote[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchIndices = useCallback(async () => {
    try {
      const data = await api.get<StockQuote[]>('/market/indices', {
        symbols: DEFAULT_INDICES.join(','),
      });
      setIndices(data);
    } catch {
      // 静默处理错误
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchIndices();
    const timer = setInterval(fetchIndices, 30000);
    return () => clearInterval(timer);
  }, [fetchIndices]);

  const formatPrice = (price: number) => {
    if (price >= 1000) return price.toFixed(0);
    if (price >= 10) return price.toFixed(2);
    return price.toFixed(2);
  };

  const renderChangeIcon = (change: number) => {
    if (change > 0) return <CaretUpOutlined />;
    if (change < 0) return <CaretDownOutlined />;
    return <MinusOutlined />;
  };

  if (loading) {
    return (
      <div className="market-overview">
        <div className="market-overview-loading">
          <Spin size="small" />
          <span>加载指数数据...</span>
        </div>
      </div>
    );
  }

  return (
    <div className={`market-overview ${className}`}>
      <div className="market-overview-title">市场概览</div>
      <div className="market-overview-cards">
        {indices.map((idx) => {
          const isUp = idx.change > 0;
          const isDown = idx.change < 0;
          const changeClass = isUp ? 'up' : isDown ? 'down' : 'flat';

          return (
            <div key={idx.symbol} className="market-overview-card">
              <div className="market-overview-card-name">
                {INDEX_NAMES[idx.symbol] || idx.name}
              </div>
              <div className={`market-overview-card-price ${changeClass}`}>
                {formatPrice(idx.price)}
              </div>
              <div className={`market-overview-card-change ${changeClass}`}>
                <span className="market-overview-card-icon">
                  {renderChangeIcon(idx.change)}
                </span>
                <span className="market-overview-card-change-value">
                  {idx.changePercent >= 0 ? '+' : ''}{idx.changePercent.toFixed(2)}%
                </span>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}