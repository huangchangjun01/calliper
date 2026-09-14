import { useState, useEffect, useCallback } from 'react';
import { useParams } from 'react-router-dom';
import { Spin, Descriptions, Tag } from 'antd';
import { CaretUpOutlined, CaretDownOutlined, MinusOutlined } from '@ant-design/icons';
import StockChart from '@/components/StockChart';
import DataState from '@/components/common/DataState';
import InfoTip from '@/components/common/InfoTip';
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
  const [depthError, setDepthError] = useState<string | null>(null);
  const [fundamentals, setFundamentals] = useState<Fundamentals | null>(null);
  const [fundamentalsError, setFundamentalsError] = useState<string | null>(null);

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
  const loadDepth = useCallback(() => {
    if (!symbol) return;
    setDepthError(null);
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
        setDepth(null);
        setDepthError('盘口数据加载失败');
      });
  }, [symbol]);

  useEffect(() => {
    loadDepth();
  }, [loadDepth]);

  // 获取基本面信息
  const loadFundamentals = useCallback(() => {
    if (!symbol) return;
    setFundamentalsError(null);
    api.get<Fundamentals>(`/market/fundamentals/${symbol}`)
      .then(setFundamentals)
      .catch(() => {
        setFundamentals(null);
        setFundamentalsError('基本面数据加载失败');
      });
  }, [symbol]);

  useEffect(() => {
    loadFundamentals();
  }, [loadFundamentals]);

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
              <InfoTip tip="最新成交价"><span className="stock-detail-price-value">{quote.price.toFixed(2)}</span></InfoTip>
            </div>
            <div className="stock-detail-change">
              <span className="stock-detail-change-icon">
                {isUp ? <CaretUpOutlined /> : isDown ? <CaretDownOutlined /> : <MinusOutlined />}
              </span>
              <InfoTip tip="现价 − 昨收">
              <span className="stock-detail-change-value">
                {quote.change > 0 ? '+' : ''}{quote.change.toFixed(2)}
              </span>
              </InfoTip>
              <InfoTip tip="涨跌额 ÷ 昨收 × 100%">
              <span className="stock-detail-change-percent">
                ({quote.changePercent > 0 ? '+' : ''}{quote.changePercent.toFixed(2)}%)
              </span>
              </InfoTip>
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
              <Descriptions.Item label={<InfoTip tip="当日开盘价"><span>开盘价</span></InfoTip>}>
                <span className="detail-value">{quote.open.toFixed(2)}</span>
              </Descriptions.Item>
              <Descriptions.Item label={<InfoTip tip="当日最高 / 最低成交价"><span>最高价</span></InfoTip>}>
                <span className="detail-value up">{quote.high.toFixed(2)}</span>
              </Descriptions.Item>
              <Descriptions.Item label={<InfoTip tip="当日最高 / 最低成交价"><span>最低价</span></InfoTip>}>
                <span className="detail-value down">{quote.low.toFixed(2)}</span>
              </Descriptions.Item>
              <Descriptions.Item label={<InfoTip tip="上一交易日收盘价"><span>昨收价</span></InfoTip>}>
                <span className="detail-value">{quote.preClose.toFixed(2)}</span>
              </Descriptions.Item>
              <Descriptions.Item label={<InfoTip tip="当日累计成交量（手）"><span>成交量</span></InfoTip>}>
                <span className="detail-value">{formatNumber(quote.volume)}</span>
              </Descriptions.Item>
              <Descriptions.Item label={<InfoTip tip="当日累计成交金额（元）"><span>成交额</span></InfoTip>}>
                <span className="detail-value">{formatNumber(quote.amount)}</span>
              </Descriptions.Item>
              <Descriptions.Item label={<span>换手率</span>}>
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
          <div className="stock-detail-section-title"><InfoTip tip="当前委托盘实时挂单（买/卖五档）。买一为当前最高买入价，卖一为当前最低卖出价；红色为买盘、绿色为卖盘。">盘口深度</InfoTip></div>
          <DataState
            error={depthError}
            onRetry={loadDepth}
            isEmpty={!depthError && !depth}
            emptyText="暂无盘口数据"
          >
            {depth && (
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
            )}
          </DataState>
        </div>
      </div>

      {/* 底部：基本面信息 */}
      <div className="stock-detail-fundamentals">
        <div className="stock-detail-section-title">基本面信息</div>
        <DataState
          error={fundamentalsError}
          onRetry={loadFundamentals}
          isEmpty={!fundamentalsError && !fundamentals}
          emptyText="暂无基本面数据"
        >
          {fundamentals && (
            <div className="fundamentals-grid">
            <div className="fundamental-item">
              <span className="fundamental-label">总市值</span>
              <InfoTip tip="总市值 = 总股本 × 最新股价"><span className="fundamental-value">{fundamentals.marketCap != null ? formatNumber(fundamentals.marketCap) : '--'}</span></InfoTip>
            </div>
            <div className="fundamental-item">
              <span className="fundamental-label">市盈率(PE)</span>
              <InfoTip tip="市盈率 = 股价 ÷ 每股收益；衡量估值高低，不同行业差异较大"><span className="fundamental-value">{fundamentals.pe?.toFixed(2) ?? '--'}</span></InfoTip>
            </div>
            <div className="fundamental-item">
              <span className="fundamental-label">市净率(PB)</span>
              <InfoTip tip="市净率 = 股价 ÷ 每股净资产；衡量估值，<1 常被视为破净"><span className="fundamental-value">{fundamentals.pb?.toFixed(2) ?? '--'}</span></InfoTip>
            </div>
            <div className="fundamental-item">
              <span className="fundamental-label">每股收益(EPS)</span>
              <InfoTip tip="每股收益 = 净利润 ÷ 总股本"><span className="fundamental-value">{fundamentals.eps?.toFixed(2) ?? '--'}</span></InfoTip>
            </div>
            <div className="fundamental-item">
              <span className="fundamental-label">净资产收益率(ROE)</span>
              <InfoTip tip="净资产收益率 = 净利润 ÷ 净资产 × 100%；衡量公司盈利能力"><span className="fundamental-value">{fundamentals.roe != null ? `${fundamentals.roe.toFixed(2)}%` : '--'}</span></InfoTip>
            </div>
            <div className="fundamental-item">
              <span className="fundamental-label">股息率</span>
              <InfoTip tip="每股分红 ÷ 股价 × 100%；衡量现金回报"><span className="fundamental-value">{fundamentals.dividendYield != null ? `${fundamentals.dividendYield.toFixed(2)}%` : '--'}</span></InfoTip>
            </div>
            <div className="fundamental-item">
              <span className="fundamental-label">资产负债率</span>
              <InfoTip tip="总负债 ÷ 总资产 × 100%"><span className="fundamental-value">{fundamentals.debtRatio != null ? `${fundamentals.debtRatio.toFixed(2)}%` : '--'}</span></InfoTip>
            </div>
            <div className="fundamental-item">
              <span className="fundamental-label">流动比率</span>
              <InfoTip tip="流动资产 ÷ 流动负债，衡量短期偿债能力"><span className="fundamental-value">{fundamentals.currentRatio?.toFixed(2) ?? '--'}</span></InfoTip>
            </div>
          </div>
          )}
        </DataState>
      </div>
    </div>
  );
}