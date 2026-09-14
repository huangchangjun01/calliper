import { Spin } from 'antd';
import {
  CaretUpOutlined,
  CaretDownOutlined,
  MinusOutlined,
} from '@ant-design/icons';
import type { FailureCase, PredictionDirection } from '@/types';
import InfoTip from '@/components/common/InfoTip';
import { PERIOD_INFO } from '@/constants/predictions';
import './index.css';

interface FailureAnalysisProps {
  data: FailureCase[];
  loading: boolean;
}

const DIRECTION_CONFIG: Record<PredictionDirection, { label: string; color: string; icon: React.ReactNode }> = {
  up: { label: '看涨', color: 'var(--color-error)', icon: <CaretUpOutlined /> },
  down: { label: '看跌', color: 'var(--color-success)', icon: <CaretDownOutlined /> },
  flat: { label: '震荡', color: 'var(--text-tertiary)', icon: <MinusOutlined /> },
};

const PERIOD_LABEL: Record<string, string> = {
  short: PERIOD_INFO.short.label,
  medium: PERIOD_INFO.medium.label,
  long: PERIOD_INFO.long.label,
  short_term: PERIOD_INFO.short.label,
  medium_term: PERIOD_INFO.medium.label,
  long_term: PERIOD_INFO.long.label,
};

export default function FailureAnalysis({ data, loading }: FailureAnalysisProps) {
  if (loading) {
    return (
      <div className="failure-analysis">
        <div className="failure-analysis-loading">
          <Spin size="default" />
          <span>加载失败分析数据...</span>
        </div>
      </div>
    );
  }

  return (
    <div className="failure-analysis">
      <div className="failure-analysis-header">
        <span className="failure-analysis-title">
          <InfoTip tip="对连续判定失败（≥3 次）或命中异常波动的预测进行归因，给出推测性原因，帮助定位模型失准模式。">预测失败归因分析</InfoTip>
        </span>
        <span className="failure-analysis-count">{data.length} 条记录</span>
      </div>

      <div className="failure-analysis-list">
        {data.length === 0 ? (
          <div className="failure-analysis-empty">
            <span>暂无预测失败记录</span>
          </div>
        ) : (
          <table>
            <thead>
              <tr>
                <th>代码</th>
                <th>名称</th>
                <th>预测方向</th>
                <th>实际方向</th>
                <th>周期</th>
                <th>预测时间</th>
                <th>归因摘要</th>
              </tr>
            </thead>
            <tbody>
              {data.map((item) => {
                const predCfg = DIRECTION_CONFIG[item.predicted_direction];
                const actualCfg = DIRECTION_CONFIG[item.actual_direction];

                return (
                  <tr key={item.id}>
                    <td className="col-symbol">{item.symbol}</td>
                    <td className="col-name">{item.name}</td>
                    <td>
                      <span className="failure-analysis-direction" style={{ color: predCfg.color }}>
                        {predCfg.icon}
                        {predCfg.label}
                      </span>
                    </td>
                    <td>
                      <span className="failure-analysis-direction" style={{ color: actualCfg.color }}>
                        {actualCfg.icon}
                        {actualCfg.label}
                      </span>
                    </td>
                    <td>{PERIOD_LABEL[item.period] ?? item.period}</td>
                    <td className="col-date">
                      {new Date(item.predicted_at).toLocaleString('zh-CN')}
                    </td>
                    <td>
                      <span className="failure-analysis-summary">{item.summary || '--'}</span>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}