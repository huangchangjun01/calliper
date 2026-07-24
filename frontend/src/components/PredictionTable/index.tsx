import { useState, useMemo } from 'react';
import { Spin } from 'antd';
import {
  CaretUpOutlined,
  CaretDownOutlined,
  MinusOutlined,
  DownOutlined,
  RightOutlined,
} from '@ant-design/icons';
import type { PredictionDetail, PredictionDirection } from '@/types';
import './index.css';

interface PredictionTableProps {
  data: PredictionDetail[];
  loading: boolean;
}

type SortField = 'symbol' | 'short' | 'medium' | 'long';
type SortOrder = 'asc' | 'desc';

const DIRECTION_CONFIG: Record<PredictionDirection, { label: string; color: string; icon: React.ReactNode }> = {
  up: { label: '看涨', color: 'var(--color-error)', icon: <CaretUpOutlined /> },
  down: { label: '看跌', color: 'var(--color-success)', icon: <CaretDownOutlined /> },
  flat: { label: '震荡', color: 'var(--text-tertiary)', icon: <MinusOutlined /> },
};

export default function PredictionTable({ data, loading }: PredictionTableProps) {
  const [expandedKeys, setExpandedKeys] = useState<Set<string>>(new Set());
  const [sortField, setSortField] = useState<SortField | null>(null);
  const [sortOrder, setSortOrder] = useState<SortOrder>('desc');

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortOrder((prev) => (prev === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortField(field);
      setSortOrder('desc');
    }
  };

  const sortedData = useMemo(() => {
    if (!sortField) return data;
    return [...data].sort((a, b) => {
      let valA: number | string;
      let valB: number | string;

      if (sortField === 'symbol') {
        valA = a.symbol;
        valB = b.symbol;
      } else {
        valA = a[sortField].confidence;
        valB = b[sortField].confidence;
      }

      if (valA < valB) return sortOrder === 'asc' ? -1 : 1;
      if (valA > valB) return sortOrder === 'asc' ? 1 : -1;
      return 0;
    });
  }, [data, sortField, sortOrder]);

  const toggleExpand = (id: string) => {
    setExpandedKeys((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const renderDirection = (direction: PredictionDirection, confidence: number) => {
    const cfg = DIRECTION_CONFIG[direction];
    return (
      <div className="prediction-table-direction">
        <span className="prediction-table-direction-tag" style={{ color: cfg.color }}>
          {cfg.icon}
          <span>{cfg.label}</span>
        </span>
        <div className="prediction-table-confidence">
          <div className="prediction-table-confidence-bar">
            <div
              className="prediction-table-confidence-fill"
              style={{
                width: `${(confidence * 100).toFixed(0)}%`,
                backgroundColor: cfg.color,
              }}
            />
          </div>
          <span className="prediction-table-confidence-value">
            {(confidence * 100).toFixed(0)}%
          </span>
        </div>
      </div>
    );
  };

  const renderSortIcon = (field: SortField) => {
    if (sortField !== field) return null;
    return (
      <span className="prediction-table-sort-icon">
        {sortOrder === 'asc' ? <CaretUpOutlined /> : <CaretDownOutlined />}
      </span>
    );
  };

  if (loading) {
    return (
      <div className="prediction-table-loading">
        <Spin size="default" />
        <span>加载预测数据...</span>
      </div>
    );
  }

  return (
    <div className="prediction-table">
      <div className="prediction-table-header">
        <span className="prediction-table-header-title">预测列表</span>
        <span className="prediction-table-header-count">{data.length} 只股票</span>
      </div>
      <div className="prediction-table-wrapper">
        <table>
          <thead>
            <tr>
              <th className="col-expand"></th>
              <th
                className="col-symbol sortable"
                onClick={() => handleSort('symbol')}
              >
                代码 {renderSortIcon('symbol')}
              </th>
              <th className="col-name">名称</th>
              <th
                className="col-direction sortable"
                onClick={() => handleSort('short')}
              >
                短期预测 {renderSortIcon('short')}
              </th>
              <th
                className="col-direction sortable"
                onClick={() => handleSort('medium')}
              >
                中短期预测 {renderSortIcon('medium')}
              </th>
              <th
                className="col-direction sortable"
                onClick={() => handleSort('long')}
              >
                长期预测 {renderSortIcon('long')}
              </th>
            </tr>
          </thead>
          <tbody>
            {sortedData.map((item) => {
              const isExpanded = expandedKeys.has(item.id);
              return (
                <tr key={item.id} className={isExpanded ? 'row-expanded' : ''}>
                  <td className="col-expand">
                    <button
                      className="prediction-table-expand-btn"
                      onClick={() => toggleExpand(item.id)}
                    >
                      {isExpanded ? <DownOutlined /> : <RightOutlined />}
                    </button>
                  </td>
                  <td className="col-symbol">{item.symbol}</td>
                  <td className="col-name">{item.name}</td>
                  <td>{renderDirection(item.short.direction, item.short.confidence)}</td>
                  <td>{renderDirection(item.medium.direction, item.medium.confidence)}</td>
                  <td>{renderDirection(item.long.direction, item.long.confidence)}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {/* 展开行详情 */}
      {sortedData.map((item) => {
        const isExpanded = expandedKeys.has(item.id);
        if (!isExpanded) return null;
        return (
          <div key={`detail-${item.id}`} className="prediction-table-detail">
            <div className="prediction-table-detail-grid">
              <div className="prediction-table-detail-item">
                <span className="prediction-table-detail-label">目标价位</span>
                <span className="prediction-table-detail-value">
                  {item.targetPrice !== null ? `¥${item.targetPrice.toFixed(2)}` : '暂无'}
                </span>
              </div>
              <div className="prediction-table-detail-item">
                <span className="prediction-table-detail-label">模型版本</span>
                <span className="prediction-table-detail-value">{item.modelVersion}</span>
              </div>
              <div className="prediction-table-detail-item">
                <span className="prediction-table-detail-label">更新时间</span>
                <span className="prediction-table-detail-value">
                  {new Date(item.updatedAt).toLocaleString('zh-CN')}
                </span>
              </div>
            </div>
            <div className="prediction-table-detail-factors">
              <span className="prediction-table-detail-label">关键因子</span>
              <div className="prediction-table-detail-tags">
                {item.keyFactors.map((factor, idx) => (
                  <span key={idx} className="prediction-table-detail-tag">
                    {factor}
                  </span>
                ))}
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}