import { Spin } from 'antd';
import {
  CaretUpOutlined,
  CaretDownOutlined,
  MinusOutlined,
} from '@ant-design/icons';
import type { FailureCase, PredictionDirection } from '@/types';
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

const REASON_COLORS: Record<string, string> = {
  '财报发布': '#faad14',
  '行业异动': '#ff7a45',
  '突发事件': '#f5222d',
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
        <span className="failure-analysis-title">预测失败归因分析</span>
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
                <th>预测价</th>
                <th>实际价</th>
                <th>日期</th>
                <th>可能原因</th>
              </tr>
            </thead>
            <tbody>
              {data.map((item) => {
                const predCfg = DIRECTION_CONFIG[item.predictedDirection];
                const actualCfg = DIRECTION_CONFIG[item.actualDirection];
                const priceDiff = item.actualPrice - item.predictedPrice;
                const priceDiffPercent = item.predictedPrice > 0
                  ? ((priceDiff / item.predictedPrice) * 100)
                  : 0;

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
                    <td className="col-price">¥{item.predictedPrice.toFixed(2)}</td>
                    <td className={`col-price ${priceDiff > 0 ? 'price-up' : priceDiff < 0 ? 'price-down' : ''}`}>
                      ¥{item.actualPrice.toFixed(2)}
                      <span className="failure-analysis-price-diff">
                        ({priceDiffPercent >= 0 ? '+' : ''}{priceDiffPercent.toFixed(1)}%)
                      </span>
                    </td>
                    <td className="col-date">{item.date}</td>
                    <td>
                      <div className="failure-analysis-reasons">
                        {item.reasons.map((reason, idx) => (
                          <span
                            key={idx}
                            className="failure-analysis-reason-tag"
                            style={{
                              color: REASON_COLORS[reason] || 'var(--text-tertiary)',
                              background: `${REASON_COLORS[reason] || 'var(--text-tertiary)'}15`,
                              borderColor: `${REASON_COLORS[reason] || 'var(--text-tertiary)'}30`,
                            }}
                          >
                            {reason}
                          </span>
                        ))}
                      </div>
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