import { useState, useEffect, useCallback } from 'react';
import ReactECharts from 'echarts-for-react';
import type { EChartsOption } from 'echarts';
import { Spin } from 'antd';
import api from '@/services/api';
import type { KlineItem } from '@/types';
import dayjs from 'dayjs';
import './index.css';

interface StockChartProps {
  symbol: string;
  interval?: '1m' | '5m' | '15m' | '30m' | '60m' | '1d' | '1w';
  height?: number;
  className?: string;
}

const INTERVAL_LABELS: Record<string, string> = {
  '1m': '分时',
  '5m': '5分',
  '15m': '15分',
  '30m': '30分',
  '60m': '60分',
  '1d': '日K',
  '1w': '周K',
};

export default function StockChart({
  symbol,
  interval = '1d',
  height = 400,
  className = '',
}: StockChartProps) {
  const [data, setData] = useState<KlineItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [currentInterval, setCurrentInterval] = useState(interval);

  const fetchKline = useCallback(async (sym: string, intv: string) => {
    setLoading(true);
    try {
      const result = await api.get<{ symbol: string; interval: string; data: Record<string, unknown>[] }>(`/market/kline/${sym}`, {
        interval: intv,
      });
      // 后端 MarketData 用 price 表示收盘价，前端 KlineItem 用 close；timestamp 为 ISO 字符串需转 number
      const mapped: KlineItem[] = (result.data || []).map((item) => ({
        timestamp: typeof item.timestamp === 'string'
          ? new Date(item.timestamp as string).getTime()
          : (item.timestamp as number) || 0,
        open: (item.open as number) ?? 0,
        high: (item.high as number) ?? 0,
        low: (item.low as number) ?? 0,
        close: (item.price as number) ?? (item.close as number) ?? 0,
        volume: (item.volume as number) ?? 0,
        amount: (item.amount as number) ?? 0,
      }));
      setData(mapped);
    } catch {
      setData([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (symbol) {
      fetchKline(symbol, currentInterval);
    }
  }, [symbol, currentInterval, fetchKline]);

  const isIntraday = ['1m', '5m', '15m', '30m', '60m'].includes(currentInterval);

  const intervals = ['1m', '5m', '15m', '30m', '60m', '1d', '1w'] as const;

  const handleIntervalChange = (newInterval: typeof intervals[number]) => {
    setCurrentInterval(newInterval);
  };

  const option: EChartsOption = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'cross',
        crossStyle: {
          color: '#999',
        },
      },
      backgroundColor: 'rgba(20, 20, 20, 0.92)',
      borderColor: '#444',
      textStyle: {
        color: '#e8e8e8',
        fontSize: 12,
      },
    },
    grid: [
      {
        left: '3%',
        right: '3%',
        top: 10,
        height: '60%',
        containLabel: true,
      },
      {
        left: '3%',
        right: '3%',
        top: '75%',
        height: '20%',
        containLabel: true,
      },
    ],
    xAxis: [
      {
        type: 'category' as const,
        gridIndex: 0,
        data: data.map((item) =>
          isIntraday
            ? dayjs(item.timestamp).format('HH:mm')
            : dayjs(item.timestamp).format('MM-DD')
        ),
        axisLine: { lineStyle: { color: '#bbb' } },
        axisLabel: { color: '#666', fontSize: 11 },
        axisTick: { show: false },
      },
      {
        type: 'category' as const,
        gridIndex: 1,
        data: data.map((item) =>
          isIntraday
            ? dayjs(item.timestamp).format('HH:mm')
            : dayjs(item.timestamp).format('MM-DD')
        ),
        axisLine: { lineStyle: { color: '#bbb' } },
        axisLabel: { show: false },
        axisTick: { show: false },
      },
    ],
    yAxis: [
      {
        type: 'value' as const,
        scale: true,
        gridIndex: 0,
        axisLine: { show: false },
        axisLabel: { color: '#666', fontSize: 11 },
        splitLine: { lineStyle: { color: '#e8e8e8', type: 'dashed' as const } },
      },
      {
        type: 'value' as const,
        gridIndex: 1,
        axisLine: { show: false },
        axisLabel: { color: '#666', fontSize: 10 },
        splitLine: { show: false },
      },
    ],
    dataZoom: [
      {
        type: 'inside' as const,
        start: 50,
        end: 100,
      },
      ...(isIntraday
        ? []
        : [
            {
              type: 'slider' as const,
              start: 50,
              end: 100,
              height: 20,
              bottom: 0,
              borderColor: '#d9d9d9',
              backgroundColor: '#f5f5f5',
              fillerColor: 'rgba(22, 119, 255, 0.1)',
              handleStyle: {
                color: '#1677ff',
              },
              textStyle: {
                color: '#666',
              },
            },
          ]),
    ],
    series: (isIntraday
      ? [
          {
            name: '价格',
            type: 'line' as const,
            xAxisIndex: 0,
            yAxisIndex: 0,
            data: data.map((item) => item.close),
            smooth: false,
            symbol: 'none' as const,
            lineStyle: {
              color: '#1677ff',
              width: 1.5,
            },
            areaStyle: {
              color: {
                type: 'linear' as const,
                x: 0,
                y: 0,
                x2: 0,
                y2: 1,
                colorStops: [
                  { offset: 0, color: 'rgba(22, 119, 255, 0.3)' },
                  { offset: 1, color: 'rgba(22, 119, 255, 0.02)' },
                ],
              },
            },
            markLine: {
              silent: true,
              symbol: 'none' as const,
              lineStyle: {
                color: '#bbb',
                type: 'dashed' as const,
              },
              data: [
                {
                  yAxis: data.length > 0 ? data[0]?.close : undefined,
                  label: {
                    formatter: '昨收: {c}',
                    color: '#888',
                  },
                },
              ],
            },
          },
          {
            name: '成交量',
            type: 'bar' as const,
            xAxisIndex: 1,
            yAxisIndex: 1,
            data: data.map((item) => ({
              value: item.volume,
              itemStyle: { color: item.close >= item.open ? '#ef5350' : '#26a69a' },
            })),
          },
        ]
      : [
          {
            name: 'K线',
            type: 'candlestick' as const,
            xAxisIndex: 0,
            yAxisIndex: 0,
            data: data.map((item) => [item.open, item.close, item.low, item.high]),
            itemStyle: {
              color: '#ef5350',
              color0: '#26a69a',
              borderColor: '#ef5350',
              borderColor0: '#26a69a',
            },
          },
          {
            name: 'MA5',
            type: 'line' as const,
            xAxisIndex: 0,
            yAxisIndex: 0,
            data: calcMA(data, 5),
            smooth: true,
            symbol: 'none' as const,
            lineStyle: { width: 1, color: '#f5c542' },
          },
          {
            name: 'MA10',
            type: 'line' as const,
            xAxisIndex: 0,
            yAxisIndex: 0,
            data: calcMA(data, 10),
            smooth: true,
            symbol: 'none' as const,
            lineStyle: { width: 1, color: '#e91e63' },
          },
          {
            name: 'MA20',
            type: 'line' as const,
            xAxisIndex: 0,
            yAxisIndex: 0,
            data: calcMA(data, 20),
            smooth: true,
            symbol: 'none' as const,
            lineStyle: { width: 1, color: '#7b1fa2' },
          },
          {
            name: '成交量',
            type: 'bar' as const,
            xAxisIndex: 1,
            yAxisIndex: 1,
            data: data.map((item) => ({
              value: item.volume,
              itemStyle: { color: item.close >= item.open ? '#ef5350' : '#26a69a' },
            })),
          },
        ]) as EChartsOption['series'],
  };

  return (
    <div className={`stock-chart ${className}`}>
      <div className="stock-chart-header">
        <span className="stock-chart-symbol">{symbol}</span>
        <div className="stock-chart-intervals">
          {intervals.map((intv) => (
            <button
              key={intv}
              className={`stock-chart-interval-btn ${currentInterval === intv ? 'active' : ''}`}
              onClick={() => handleIntervalChange(intv)}
            >
              {INTERVAL_LABELS[intv]}
            </button>
          ))}
        </div>
      </div>
      <div className="stock-chart-body">
        {loading ? (
          <div className="stock-chart-loading">
            <Spin size="default" />
          </div>
        ) : data.length === 0 ? (
          <div className="stock-chart-empty">
            <span>暂无数据</span>
          </div>
        ) : (
          <ReactECharts
            option={option}
            style={{ height, width: '100%' }}
            notMerge
            lazyUpdate
            opts={{ renderer: 'canvas' }}
          />
        )}
      </div>
    </div>
  );
}

function calcMA(data: KlineItem[], period: number): (number | null)[] {
  const result: (number | null)[] = [];
  for (let i = 0; i < data.length; i++) {
    if (i < period - 1) {
      result.push(null);
    } else {
      let sum = 0;
      for (let j = i - period + 1; j <= i; j++) {
        sum += data[j].close;
      }
      result.push(+(sum / period).toFixed(2));
    }
  }
  return result;
}