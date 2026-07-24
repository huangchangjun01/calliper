import { useState, useMemo } from 'react';
import ReactECharts from 'echarts-for-react';
import type { EChartsOption } from 'echarts';
import { Spin } from 'antd';
import type { AccuracyTrend, PredictionPeriod } from '@/types';
import './index.css';

interface AccuracyChartProps {
  data: AccuracyTrend | undefined;
  loading: boolean;
  onPeriodChange: (period: PredictionPeriod) => void;
  currentPeriod: PredictionPeriod;
}

const PERIOD_OPTIONS: { value: PredictionPeriod; label: string }[] = [
  { value: 'short', label: '短期' },
  { value: 'medium', label: '中短期' },
  { value: 'long', label: '长期' },
];

export default function AccuracyChart({
  data,
  loading,
  onPeriodChange,
  currentPeriod,
}: AccuracyChartProps) {
  const [showTimeRange, setShowTimeRange] = useState<7 | 30 | -1>(30);

  const filteredData = useMemo(() => {
    if (!data?.data) return [];
    if (showTimeRange === -1) return data.data;
    return data.data.slice(-showTimeRange);
  }, [data, showTimeRange]);

  const avgAccuracy = useMemo(() => {
    if (filteredData.length === 0) return 0;
    const sum = filteredData.reduce((acc, d) => acc + d.accuracy, 0);
    return (sum / filteredData.length) * 100;
  }, [filteredData]);

  const chartOption: EChartsOption = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(20, 20, 20, 0.9)',
      borderColor: '#333',
      textStyle: { color: '#e0e0e0', fontSize: 12 },
      formatter: (params: unknown) => {
        const p = params as Array<{ axisValue: string; value: number; seriesName: string }>;
        if (!p || p.length === 0) return '';
        return `
          <div style="font-weight:600;margin-bottom:4px">${p[0].axisValue}</div>
          <div>${p[0].seriesName}: ${(p[0].value * 100).toFixed(1)}%</div>
        `;
      },
    },
    grid: {
      left: '3%',
      right: '4%',
      top: 20,
      bottom: 30,
      containLabel: true,
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: filteredData.map((d) => d.date.slice(5)),
      axisLine: {
        lineStyle: { color: 'var(--border-color)' },
      },
      axisLabel: {
        color: 'var(--text-tertiary)',
        fontSize: 11,
      },
      axisTick: { show: false },
    },
    yAxis: {
      type: 'value',
      min: 0,
      max: 1,
      axisLabel: {
        color: 'var(--text-tertiary)',
        fontSize: 11,
        formatter: (value: number) => `${(value * 100).toFixed(0)}%`,
      },
      splitLine: {
        lineStyle: { color: 'var(--border-color)', type: 'dashed' },
      },
    },
    series: [
      {
        name: '准确率',
        type: 'line',
        data: filteredData.map((d) => d.accuracy),
        smooth: true,
        symbol: 'circle',
        symbolSize: 4,
        lineStyle: {
          color: '#1677ff',
          width: 2,
        },
        itemStyle: {
          color: '#1677ff',
        },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(22, 119, 255, 0.25)' },
              { offset: 1, color: 'rgba(22, 119, 255, 0.02)' },
            ],
          },
        },
        markLine: {
          silent: true,
          symbol: 'none',
          lineStyle: {
            color: 'var(--text-tertiary)',
            type: 'dashed',
          },
          label: {
            color: 'var(--text-tertiary)',
            fontSize: 11,
            formatter: `均值: ${avgAccuracy.toFixed(1)}%`,
          },
          data: [
            { yAxis: avgAccuracy / 100 },
          ],
        },
      },
    ],
  };

  if (loading) {
    return (
      <div className="accuracy-chart">
        <div className="accuracy-chart-loading">
          <Spin size="default" />
          <span>加载准确率数据...</span>
        </div>
      </div>
    );
  }

  return (
    <div className="accuracy-chart">
      <div className="accuracy-chart-header">
        <span className="accuracy-chart-title">预测准确率趋势</span>
        <div className="accuracy-chart-controls">
          <div className="accuracy-chart-periods">
            {PERIOD_OPTIONS.map((opt) => (
              <button
                key={opt.value}
                className={`accuracy-chart-period-btn ${currentPeriod === opt.value ? 'active' : ''}`}
                onClick={() => onPeriodChange(opt.value)}
              >
                {opt.label}
              </button>
            ))}
          </div>
          <div className="accuracy-chart-range">
            <button
              className={`accuracy-chart-range-btn ${showTimeRange === 7 ? 'active' : ''}`}
              onClick={() => setShowTimeRange(7)}
            >
              7日
            </button>
            <button
              className={`accuracy-chart-range-btn ${showTimeRange === 30 ? 'active' : ''}`}
              onClick={() => setShowTimeRange(30)}
            >
              30日
            </button>
            <button
              className={`accuracy-chart-range-btn ${showTimeRange === -1 ? 'active' : ''}`}
              onClick={() => setShowTimeRange(-1)}
            >
              累计
            </button>
          </div>
        </div>
      </div>
      <div className="accuracy-chart-body">
        <ReactECharts
          option={chartOption}
          style={{ height: 320, width: '100%' }}
          notMerge
          lazyUpdate
          opts={{ renderer: 'canvas' }}
        />
      </div>
      <div className="accuracy-chart-summary">
        <span className="accuracy-chart-summary-label">当前平均准确率</span>
        <span className="accuracy-chart-summary-value">
          {avgAccuracy.toFixed(1)}%
        </span>
      </div>
    </div>
  );
}