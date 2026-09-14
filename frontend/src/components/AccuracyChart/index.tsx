import { useState, useMemo } from 'react';
import ReactECharts from 'echarts-for-react';
import type { EChartsOption } from 'echarts';
import { Spin } from 'antd';
import InfoTip from '@/components/common/InfoTip';
import { PERIOD_INFO } from '@/constants/predictions';
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
    if (!data) return [];
    if (showTimeRange === -1) return data;
    return data.slice(-showTimeRange);
  }, [data, showTimeRange]);

  const avgAccuracy = useMemo(() => {
    // 无数据日（accuracy 为 null）为断点，不参与均值计算
    const valid = filteredData.filter((d) => d.accuracy !== null);
    if (valid.length === 0) return null;
    const sum = valid.reduce((acc, d) => acc + (d.accuracy as number), 0);
    return (sum / valid.length) * 100;
  }, [filteredData]);

  const chartOption: EChartsOption = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(20, 20, 20, 0.9)',
      borderColor: '#333',
      textStyle: { color: '#e0e0e0', fontSize: 12 },
      formatter: (params: unknown) => {
        const p = params as Array<{ axisValue: string; value: number | null; seriesName: string }>;
        if (!p || p.length === 0) return '';
        const value = p[0].value;
        return `
          <div style="font-weight:600;margin-bottom:4px">${p[0].axisValue}</div>
          <div>${p[0].seriesName}: ${value === null || value === undefined ? '无数据' : `${(value * 100).toFixed(1)}%`}</div>
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
        // 无数据日（null）为断点，折线断开不连线
        connectNulls: false,
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
        // 全为断点（无有效均值）时不绘制均值参考线
        markLine:
          avgAccuracy === null
            ? undefined
            : {
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
        <span className="accuracy-chart-title">
          <InfoTip tip="横轴为日期，纵轴为当日到期预测的准确率（%）；无评估的日期显示为断点。数据来自已评估的预测记录。">预测准确率趋势</InfoTip>
        </span>
        <div className="accuracy-chart-controls">
          <div className="accuracy-chart-periods">
            {PERIOD_OPTIONS.map((opt) => (
              <InfoTip key={opt.value} tip={PERIOD_INFO[opt.value].desc} placement="bottom">
                <button
                  className={`accuracy-chart-period-btn ${currentPeriod === opt.value ? 'active' : ''}`}
                  onClick={() => onPeriodChange(opt.value)}
                >
                  {opt.label}
                </button>
              </InfoTip>
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
          opts={{
            renderer: 'canvas',
            // 固定至少 2x 渲染，避免缩放下出现图表模糊
            devicePixelRatio: Math.max(2, Math.round(window.devicePixelRatio || 1)),
          }}
        />
      </div>
      <div className="accuracy-chart-summary">
        <span className="accuracy-chart-summary-label">当前平均准确率</span>
        <span className="accuracy-chart-summary-value">
          {avgAccuracy === null ? '—（无数据）' : `${avgAccuracy.toFixed(1)}%`}
        </span>
      </div>
    </div>
  );
}