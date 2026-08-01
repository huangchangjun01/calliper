import { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import { Spin, Descriptions, Tag } from 'antd';
import { CaretUpOutlined, CaretDownOutlined, MinusOutlined } from '@ant-design/icons';
import StockChart from '@/components/StockChart';
import useStockQuote from '@/hooks/useStockQuote';
import api from '@/services/api';
import type { Stock, StockQuote } from '@/types';
import '@/pages/StockDetail.css';

interface DepthLevel {
  price: number;
  volume: number;
}

interface DepthData {
  bids: DepthLevel[];
  asks: DepthLevel[];
}

interface RawDepthData {
  symbol: string;
  bid_prices: number[];
  bid_volumes: number[];
  ask_prices: number[];
  ask_volumes: number[];
  timestamp: number;
}

interface Fundamentals {
  marketCap?: number;
  pe?: number;
  pb?: number;
  eps?: number;
  roe?: number;
  debtRatio?: number;
  currentRatio?: number;
  dividendYield?: number;
}

export default function StockDetail() {
  const { symbol } = useParams<{ symbol: string }>();
  const [stock, setStock] = useState<Stock | null>(null);
  const [loading, setLoading] = useState(true);
  const [depth, setDepth] = useState<DepthData | null>(null);
  const [fundamentals, setFundamentals] = useState<Fundamentals | null>(null);

  const { stocks } = useStockQuote(symbol ? [symbol] : []);
  const quote: StockQuote | undefined = symbol ? stocks.get(symbol) : undefined;

  // 获取股票基本信息
  useEffect(() => {
    if (!symbol) return;
    setLoading(true);

    api.get<Stock>(`/stocks/${symbol}`)
      .then(setStock)
      .catch(() => {
        setStock({
          symbol,
          name: symbol,
          exchange: '--',
          industry: '--',
          marketCap: 0,
          listingDate: '--',
          description: '--',
        });
      })
      .finally(() => setLoading(false));
  }, [symbol]);

  // 获取盘口深度
  useEffect(() => {
    if (!symbol) return;
    api.get<RawDepthData>(`/market/depth/${symbol}`)
      .then((data) => {
        const mappedDepth: DepthData = {
          bids: (data.bid_prices || []).map((price: number, i: number) => ({
            price,
            volume: data.bid_volumes?.[i] ?? 0,
          })),
          asks: (data.ask_prices || []).map((price: number, i: number) => ({
            price,
            volume: data.ask_volumes?.[i] ?? 0,
          })),
        };
        setDepth(mappedDepth);
      })
      .catch(() => {
        setDepth({
          bids: [
            { price: 1875.00, volume: 1200 },
            { price: 1874.50, volume: 3500 },
            { price: 1874.00, volume: 2100 },
            { price: 1873.50, volume: 5800 },
            { price: 1873.00, volume: 4200 },
          ],
          asks: [
            { price: 1875.50, volume: 800 },
            { price: 1876.00, volume: 2600 },
            { price: 1876.50, volume: 1900 },
            { price: 1877.00, volume: 4100 },
            { price: 1877.50, volume: 3200 },
          ],
        });
      });
  }, [symbol]);

  // 获取基本面信息
  useEffect(() => {
    if (!symbol) return;
    api.get<Fundamentals>(`/market/fundamentals/${symbol}`)
      .then(setFundamentals)
      .catch(() => {
        setFundamentals({
          marketCap: 2.35e12,
          pe: 28.5,
          pb: 6.2,
          eps: 65.8,
          roe: 25.3,
          dividendYield: 1.2,
        });
      });
  }, [symbol]);

  const formatNumber = (num: number, decimals = 2) => {
    if (num >= 1e12) return `${(num / 1e12).toFixed(decimals)}万亿`;
    if (num >= 1e8) return `${(num / 1e8).toFixed(decimals)}亿`;
    if (num >= 1e4) return `${(num / 1e4).toFixed(decimals)}万`;
    return num.toFixed(decimals);
  };

  const getMaxDepthVolume = () => {
    if (!depth) return 1;
    const allVolumes = [...depth.bids.map((b) => b.volume), ...depth.asks.map((a) => a.volume)];
    return Math.max(...allVolumes, 1);
  };

  if (loading) {
    return (
      <div className="stock-detail">
        <div className="stock-detail-loading">
          <Spin size="large" />
          <span>加载中...</span>
        </div>
      </div>
    );
  }

  const isUp = quote && quote.change > 0;
  const isDown = quote && quote.change < 0;
  const changeClass = isUp ? 'up' : isDown ? 'down' : 'flat';

  return (
    <div className="stock-detail">
      {/* 顶部：股票基本信息 */}
      <div className="stock-detail-header">
        <div className="stock-detail-header-left">
          <h1 className="stock-detail-name">{stock?.name || symbol}</h1>
          <span className="stock-detail-symbol">{symbol}</span>
          {stock?.exchange && (
            <Tag color="blue" className="stock-detail-tag">
              {stock.exchange}
            </Tag>
          )}
          {stock?.industry && (
            <Tag className="stock-detail-tag">{stock.industry}</Tag>
          )}
        </div>
        {quote && (
          <div className={`stock-detail-header-right ${changeClass}`}>
            <div className="stock-detail-price">
              <span className="stock-detail-price-value">{quote.price.toFixed(2)}</span>
            </div>
            <div className="stock-detail-change">
              <span className="stock-detail-change-icon">
                {isUp ? <CaretUpOutlined /> : isDown ? <CaretDownOutlined /> : <MinusOutlined />}
              </span>
              <span className="stock-detail-change-value">
                {quote.change > 0 ? '+' : ''}{quote.change.toFixed(2)}
              </span>
              <span className="stock-detail-change-percent">
                ({quote.changePercent > 0 ? '+' : ''}{quote.changePercent.toFixed(2)}%)
              </span>
            </div>
          </div>
        )}
      </div>

      {/* 内容区 */}
      <div className="stock-detail-body">
        {/* 左侧：实时行情数据 */}
        <div className="stock-detail-quote">
          <div className="stock-detail-section-title">实时行情</div>
          {quote ? (
            <Descriptions column={1} size="small" colon={false}>
              <Descriptions.Item label="开盘价">
                <span className="detail-value">{quote.open.toFixed(2)}</span>
              </Descriptions.Item>
              <Descriptions.Item label="最高价">
                <span className="detail-value up">{quote.high.toFixed(2)}</span>
              </Descriptions.Item>
              <Descriptions.Item label="最低价">
                <span className="detail-value down">{quote.low.toFixed(2)}</span>
              </Descriptions.Item>
              <Descriptions.Item label="昨收价">
                <span className="detail-value">{quote.preClose.toFixed(2)}</span>
              </Descriptions.Item>
              <Descriptions.Item label="成交量">
                <span className="detail-value">{formatNumber(quote.volume)}</span>
              </Descriptions.Item>
              <Descriptions.Item label="成交额">
                <span className="detail-value">{formatNumber(quote.amount)}</span>
              </Descriptions.Item>
              <Descriptions.Item label="换手率">
                <span className="detail-value">--</span>
              </Descriptions.Item>
            </Descriptions>
          ) : (
            <div className="stock-detail-no-data">等待行情数据...</div>
          )}
        </div>

        {/* 中间：K线图 */}
        <div className="stock-detail-chart">
          {symbol && (
            <StockChart
              symbol={symbol}
              interval="1d"
              height={420}
            />
          )}
        </div>

        {/* 右侧：盘口深度 */}
        <div className="stock-detail-depth">
          <div className="stock-detail-section-title">盘口深度</div>
          {depth ? (
            <div className="depth-panel">
              <div className="depth-asks">
                {[...depth.asks].reverse().map((level, i) => {
                  const widthPercent = (level.volume / getMaxDepthVolume()) * 100;
                  return (
                    <div key={`ask-${i}`} className="depth-row">
                      <span className="depth-price down">{level.price.toFixed(2)}</span>
                      <span className="depth-volume">{level.volume}</span>
                      <div
                        className="depth-bar depth-bar-sell"
                        style={{ width: `${widthPercent}%` }}
                      />
                    </div>
                  );
                })}
              </div>
              <div className="depth-spread">
                {quote && (
                  <span className={`depth-spread-value ${changeClass}`}>
                    {quote.price.toFixed(2)}
                  </span>
                )}
              </div>
              <div className="depth-bids">
                {depth.bids.map((level, i) => {
                  const widthPercent = (level.volume / getMaxDepthVolume()) * 100;
                  return (
                    <div key={`bid-${i}`} className="depth-row">
                      <span className="depth-price up">{level.price.toFixed(2)}</span>
                      <span className="depth-volume">{level.volume}</span>
                      <div
                        className="depth-bar depth-bar-buy"
                        style={{ width: `${widthPercent}%` }}
                      />
                    </div>
                  );
                })}
              </div>
            </div>
          ) : (
            <div className="stock-detail-no-data">暂无盘口数据</div>
          )}
        </div>
      </div>

      {/* 底部：基本面信息 */}
      <div className="stock-detail-fundamentals">
        <div className="stock-detail-section-title">基本面信息</div>
        {fundamentals ? (
          <div className="fundamentals-grid">
            <div className="fundamental-item">
              <span className="fundamental-label">总市值</span>
              <span className="fundamental-value">{fundamentals.marketCap != null ? formatNumber(fundamentals.marketCap) : '--'}</span>
            </div>
            <div className="fundamental-item">
              <span className="fundamental-label">市盈率(PE)</span>
              <span className="fundamental-value">{fundamentals.pe?.toFixed(2) ?? '--'}</span>
            </div>
            <div className="fundamental-item">
              <span className="fundamental-label">市净率(PB)</span>
              <span className="fundamental-value">{fundamentals.pb?.toFixed(2) ?? '--'}</span>
            </div>
            <div className="fundamental-item">
              <span className="fundamental-label">每股收益(EPS)</span>
              <span className="fundamental-value">{fundamentals.eps?.toFixed(2) ?? '--'}</span>
            </div>
            <div className="fundamental-item">
              <span className="fundamental-label">净资产收益率(ROE)</span>
              <span className="fundamental-value">{fundamentals.roe != null ? `${fundamentals.roe.toFixed(2)}%` : '--'}</span>
            </div>
            <div className="fundamental-item">
              <span className="fundamental-label">股息率</span>
              <span className="fundamental-value">{fundamentals.dividendYield != null ? `${fundamentals.dividendYield.toFixed(2)}%` : '--'}</span>
            </div>
            <div className="fundamental-item">
              <span className="fundamental-label">资产负债率</span>
              <span className="fundamental-value">{fundamentals.debtRatio != null ? `${fundamentals.debtRatio.toFixed(2)}%` : '--'}</span>
            </div>
            <div className="fundamental-item">
              <span className="fundamental-label">流动比率</span>
              <span className="fundamental-value">{fundamentals.currentRatio?.toFixed(2) ?? '--'}</span>
            </div>
          </div>
        ) : (
          <div className="stock-detail-no-data">暂无基本面数据</div>
        )}
      </div>
    </div>
  );
}