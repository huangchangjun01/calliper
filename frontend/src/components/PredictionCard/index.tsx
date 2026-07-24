import ReactECharts from 'echarts-for-react';
import type { EChartsOption } from 'echarts';
import type { PredictionSummary, PredictionPeriod } from '@/types';
import './index.css';

interface PredictionCardProps {
  data: PredictionSummary;
  period: PredictionPeriod;
}

const PERIOD_CONFIG: Record<PredictionPeriod, { label: string; icon: string; description: string }> = {
  short: { label: '短期预测', icon: '📊', description: '1-3 个交易日' },
  medium: { label: '中短期预测', icon: '📈', description: '1-2 周' },
  long: { label: '长期预测', icon: '🎯', description: '1-3 个月' },
};

export default function PredictionCard({ data, period }: PredictionCardProps) {
  const config = PERIOD_CONFIG[period];
  const upPercent = data.total > 0 ? ((data.upCount / data.total) * 100).toFixed(0) : '0';
  const downPercent = data.total > 0 ? ((data.downCount / data.total) * 100).toFixed(0) : '0';
  const flatPercent = data.total > 0 ? ((data.flatCount / data.total) * 100).toFixed(0) : '0';

  const pieOption: EChartsOption = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      backgroundColor: 'rgba(20, 20, 20, 0.9)',
      borderColor: '#333',
      textStyle: { color: '#e0e0e0', fontSize: 12 },
      formatter: '{b}: {c} 只 ({d}%)',
    },
    series: [
      {
        type: 'pie',
        radius: ['55%', '80%'],
        center: ['50%', '50%'],
        avoidLabelOverlap: false,
        label: { show: false },
        emphasis: {
          label: { show: true, fontSize: 14, fontWeight: 'bold' },
        },
        itemStyle: {
          borderColor: 'var(--bg-base)',
          borderWidth: 2,
        },
        data: [
          { value: data.upCount, name: '看涨', itemStyle: { color: '#ef5350' } },
          { value: data.downCount, name: '看跌', itemStyle: { color: '#26a69a' } },
          { value: data.flatCount, name: '震荡', itemStyle: { color: '#9e9e9e' } },
        ],
      },
    ],
  };

  return (
    <div className="prediction-card">
      <div className="prediction-card-header">
        <span className="prediction-card-icon">{config.icon}</span>
        <div className="prediction-card-title-group">
          <span className="prediction-card-title">{config.label}</span>
          <span className="prediction-card-desc">{config.description}</span>
        </div>
      </div>

      <div className="prediction-card-body">
        <div className="prediction-card-chart">
          <ReactECharts
            option={pieOption}
            style={{ height: 140, width: '100%' }}
            notMerge
            lazyUpdate
            opts={{ renderer: 'canvas' }}
          />
        </div>

        <div className="prediction-card-stats">
          <div className="prediction-card-stat">
            <span className="prediction-card-stat-label">总数</span>
            <span className="prediction-card-stat-value">{data.total}</span>
          </div>
          <div className="prediction-card-stat prediction-card-stat--up">
            <span className="prediction-card-stat-label">看涨</span>
            <span className="prediction-card-stat-value">
              {data.upCount}
              <span className="prediction-card-stat-percent">({upPercent}%)</span>
            </span>
          </div>
          <div className="prediction-card-stat prediction-card-stat--down">
            <span className="prediction-card-stat-label">看跌</span>
            <span className="prediction-card-stat-value">
              {data.downCount}
              <span className="prediction-card-stat-percent">({downPercent}%)</span>
            </span>
          </div>
          <div className="prediction-card-stat prediction-card-stat--flat">
            <span className="prediction-card-stat-label">震荡</span>
            <span className="prediction-card-stat-value">
              {data.flatCount}
              <span className="prediction-card-stat-percent">({flatPercent}%)</span>
            </span>
          </div>
        </div>
      </div>

      <div className="prediction-card-bar">
        <div
          className="prediction-card-bar-up"
          style={{ width: `${upPercent}%` }}
        />
        <div
          className="prediction-card-bar-down"
          style={{ width: `${downPercent}%` }}
        />
        <div
          className="prediction-card-bar-flat"
          style={{ width: `${flatPercent}%` }}
        />
      </div>
    </div>
  );
}