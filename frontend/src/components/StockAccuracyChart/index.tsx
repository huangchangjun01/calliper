import { useMemo } from 'react';
import ReactECharts from 'echarts-for-react';
import type { EChartsOption } from 'echarts';
import { Spin } from 'antd';
import type { StockAccuracy } from '@/types';
import './index.css';

interface StockAccuracyChartProps {
  data: StockAccuracy[];
  loading: boolean;
}

export default function StockAccuracyChart({ data, loading }: StockAccuracyChartProps) {
  const sortedData = useMemo(() => {
    return [...data].sort((a, b) => b.accuracy - a.accuracy);
  }, [data]);

  const chartOption = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      backgroundColor: 'rgba(20, 20, 20, 0.9)',
      borderColor: '#333',
      textStyle: { color: '#e0e0e0', fontSize: 12 },
      formatter: (params: unknown) => {
        const p = params as Array<{ name: string; value: number }>;
        if (!p || p.length === 0) return '';
        const item = p[0];
        return `
          <div style="font-weight:600;margin-bottom:4px">${item.name}</div>
          <div>准确率: ${(item.value * 100).toFixed(1)}%</div>
        `;
      },
    },
    grid: {
      left: '3%',
      right: '4%',
      top: 10,
      bottom: 30,
      containLabel: true,
    },
    xAxis: {
      type: 'category',
      data: sortedData.map((s) => s.name),
      axisLabel: {
        color: 'var(--text-tertiary)',
        fontSize: 11,
        rotate: 30,
      },
      axisTick: { show: false },
      axisLine: {
        lineStyle: { color: 'var(--border-color)' },
      },
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
        type: 'bar',
        data: sortedData.map((s) => ({
          value: s.accuracy,
          itemStyle: {
            color: s.accuracy >= 0.7
              ? '#52c41a'
              : s.accuracy >= 0.5
              ? '#1677ff'
              : '#ff4d4f',
            borderRadius: [4, 4, 0, 0] as [number, number, number, number],
          },
        })),
        barWidth: '50%',
        emphasis: {
          itemStyle: {
            color: '#4096ff',
          },
        },
        label: {
          show: true,
          position: 'top',
          color: 'var(--text-tertiary)',
          fontSize: 10,
          formatter: (params: { value: number }) => `${(params.value * 100).toFixed(0)}%`,
        },
      },
    ],
  } as EChartsOption;

  if (loading) {
    return (
      <div className="stock-accuracy-chart">
        <div className="stock-accuracy-chart-loading">
          <Spin size="default" />
          <span>加载准确率排行...</span>
        </div>
      </div>
    );
  }

  return (
    <div className="stock-accuracy-chart">
      <div className="stock-accuracy-chart-header">
        <span className="stock-accuracy-chart-title">各股票预测准确率排行</span>
      </div>
      <div className="stock-accuracy-chart-body">
        <ReactECharts
          option={chartOption}
          style={{ height: 300, width: '100%' }}
          notMerge
          lazyUpdate
          opts={{ renderer: 'canvas' }}
        />
      </div>
    </div>
  );
}