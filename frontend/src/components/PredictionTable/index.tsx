import { useMemo, useState, useRef, useEffect } from 'react';
import { Empty, Select, Button, Spin } from 'antd';
import { useNavigate } from 'react-router-dom';
import {
  CaretUpOutlined,
  CaretDownOutlined,
  MinusOutlined,
  DownOutlined,
  RightOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import type { PredictionDetail, PredictionDirection, PredictionFilters, PredictionPeriod, PredictionStatus } from '@/types';
import InfoTip from '@/components/common/InfoTip';
import { PERIOD_INFO_TIPS } from '@/constants/predictions';
import './index.css';

interface PredictionTableProps {
  items: PredictionDetail[];
  total: number;
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  onLoadMore: () => void;
  filters: PredictionFilters;
  onFiltersChange: (next: PredictionFilters) => void;
}

const DIRECTION_CONFIG: Record<PredictionDirection, { label: string; color: string; icon: React.ReactNode }> = {
  up: { label: '看涨', color: 'var(--color-error)', icon: <CaretUpOutlined /> },
  down: { label: '看跌', color: 'var(--color-success)', icon: <CaretDownOutlined /> },
  flat: { label: '震荡', color: 'var(--text-tertiary)', icon: <MinusOutlined /> },
};

const STATUS_CONFIG: Record<PredictionStatus, { label: string; color: string; background: string }> = {
  pending: { label: '待验证', color: '#1677ff', background: 'rgba(22,119,255,0.1)' },
  correct: { label: '已对', color: '#52c41a', background: 'rgba(82,196,26,0.12)' },
  wrong: { label: '已错', color: '#ff4d4f', background: 'rgba(255,77,79,0.12)' },
};

const PERIOD_OPTIONS: { value: PredictionPeriod; label: string }[] = [
  { value: 'short', label: '短期' },
  { value: 'medium', label: '中短期' },
  { value: 'long', label: '长期' },
];

const STATUS_OPTIONS: { value: PredictionStatus; label: string }[] = [
  { value: 'pending', label: '待验证' },
  { value: 'correct', label: '已对' },
  { value: 'wrong', label: '已错' },
];

const CONFIDENCE_OPTIONS: { value: number; label: string }[] = [
  { value: 0.5, label: '≥50%' },
  { value: 0.6, label: '≥60%' },
  { value: 0.7, label: '≥70%' },
  { value: 0.8, label: '≥80%' },
];

const EXPIRED_OPTIONS: { value: boolean; label: string }[] = [
  { value: false, label: '仅未过期' },
  { value: true, label: '仅已过期' },
];

const PERIOD_LABEL: Record<PredictionPeriod, string> = {
  short: '短期',
  medium: '中短期',
  long: '长期',
};

/* ========== 长列表窗口化渲染参数 ========== */
const COL_COUNT = 10;
const ROW_HEIGHT = 56; // 数据行估算高度
const DETAIL_BLOCK_HEIGHT = 168; // 展开详情块固定高度（内容超高时内部滚动）
const SCROLL_VIEWPORT = 560; // 滚动视口高度
const OVERSCAN = 6; // 视口上下额外渲染的缓冲行数

interface VirtualRow {
  kind: 'data' | 'detail';
  item: PredictionDetail;
}

export default function PredictionTable({
  items,
  total,
  loading,
  loadingMore,
  hasMore,
  onLoadMore,
  filters,
  onFiltersChange,
}: PredictionTableProps) {
  const [expandedKeys, setExpandedKeys] = useState<Set<string>>(new Set());
  const [windowStart, setWindowStart] = useState(0);
  const scrollRef = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();

  // 状态筛选为客户端过滤（后端不提供 status 参数）
  const visibleItems = useMemo(() => {
    if (!filters.status) return items;
    return items.filter((it) => it.status === filters.status);
  }, [items, filters.status]);

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

  // 将「数据行 + 展开详情块」拉平为统一样式列表，便于累计高度定位
  const flattened = useMemo<VirtualRow[]>(() => {
    const rows: VirtualRow[] = [];
    for (const item of visibleItems) {
      rows.push({ kind: 'data', item });
      if (expandedKeys.has(item.id)) rows.push({ kind: 'detail', item });
    }
    return rows;
  }, [visibleItems, expandedKeys]);

  // 每个槽位的估算高度 + 累计偏移量
  const offsets = useMemo(() => {
    const off = new Array<number>(flattened.length + 1);
    off[0] = 0;
    for (let i = 0; i < flattened.length; i++) {
      off[i + 1] = off[i] + (flattened[i].kind === 'detail' ? DETAIL_BLOCK_HEIGHT : ROW_HEIGHT);
    }
    return off;
  }, [flattened]);

  // 过滤条件变化导致列表收缩时，将可视起点收敛回有效范围
  useEffect(() => {
    const totalSlots = flattened.length;
    setWindowStart((start) => Math.max(0, Math.min(start, Math.max(0, totalSlots - 1))));
  }, [filters, flattened.length]);

  const handleScroll = () => {
    const el = scrollRef.current;
    if (!el) return;
    const scrollTop = el.scrollTop;
    const off = offsets;
    // 二分定位首个累计偏移 > scrollTop 的槽位
    let lo = 0;
    let hi = flattened.length - 1;
    let ans = 0;
    while (lo <= hi) {
      const mid = (lo + hi) >> 1;
      if (off[mid] <= scrollTop) {
        ans = mid;
        lo = mid + 1;
      } else {
        hi = mid - 1;
      }
    }
    setWindowStart(ans);
  };

  const renderDirection = (direction: PredictionDirection, confidence: number) => {
    // 容错：未知/历史方向值回退为"震荡"，避免渲染崩溃
    const cfg = DIRECTION_CONFIG[direction] ?? DIRECTION_CONFIG.flat;
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

  const renderStatus = (status: PredictionStatus) => {
    const cfg = STATUS_CONFIG[status];
    return (
      <span
        className="prediction-table-status-badge"
        style={{ color: cfg.color, background: cfg.background, borderColor: `${cfg.color}55` }}
      >
        {cfg.label}
      </span>
    );
  };

  /** 目标价 / 实际价对比：显示两价及目标与实际偏离百分比（实际价来自 TSDB 最近收盘） */
  const renderPriceCompare = (target: number | null, actual: number | null) => {
    const hasTarget = target !== null && target !== undefined && !Number.isNaN(target);
    const hasActual = actual !== null && actual !== undefined && !Number.isNaN(actual);
    const t = hasTarget ? `¥${target.toFixed(2)}` : '--';
    const a = hasActual ? `¥${actual.toFixed(2)}` : '--';
    const diff = hasTarget && hasActual && target !== 0 ? ((actual - target) / target) * 100 : null;
    return (
      <span className="prediction-table-price-compare">
        <span>
          {t}
          <i className="prediction-table-price-sep">/</i>
          {a}
        </span>
        {diff !== null && (
          <em className={diff >= 0 ? 'diff-up' : 'diff-down'}>
            {diff >= 0 ? '+' : ''}
            {diff.toFixed(1)}%
          </em>
        )}
      </span>
    );
  };

  if (loading) {
    return (
      <div className="prediction-table-loading">
        <Spin size="small" />
        <span style={{ color: 'var(--text-tertiary)', fontSize: 13 }}>加载预测数据...</span>
      </div>
    );
  }

  const totalHeight = offsets[flattened.length] || 0;
  const visibleCount = Math.ceil(SCROLL_VIEWPORT / ROW_HEIGHT) + OVERSCAN;
  const start = Math.min(windowStart, Math.max(0, flattened.length - 1));
  const end = Math.min(flattened.length, start + visibleCount);
  const topPad = offsets[start];
  const bottomPad = totalHeight - offsets[end];
  const windowRows = flattened.slice(start, end);

  return (
    <div className="prediction-table">
      <div className="prediction-table-header">
        <span className="prediction-table-header-title">预测列表</span>
        <span className="prediction-table-header-count">{visibleItems.length} / {total} 条记录</span>
      </div>

      {/* 筛选工具栏 */}
      <div className="prediction-table-filters">
        <div className="prediction-table-filter">
          <span className="prediction-table-filter-label">
            <InfoTip tip={PERIOD_INFO_TIPS}>周期</InfoTip>
          </span>
          <Select
            size="small"
            allowClear
            placeholder="全部"
            style={{ minWidth: 96 }}
            options={PERIOD_OPTIONS}
            value={filters.period}
            onChange={(period) => onFiltersChange({ ...filters, period })}
          />
        </div>
        <div className="prediction-table-filter">
          <span className="prediction-table-filter-label">方向</span>
          <Select
            size="small"
            allowClear
            placeholder="全部"
            style={{ minWidth: 96 }}
            options={[
              { value: 'up', label: '看涨' },
              { value: 'down', label: '看跌' },
              { value: 'flat', label: '震荡' },
            ]}
            value={filters.direction}
            onChange={(direction) => onFiltersChange({ ...filters, direction })}
          />
        </div>
        <div className="prediction-table-filter">
          <span className="prediction-table-filter-label">置信度</span>
          <Select
            size="small"
            allowClear
            placeholder="全部"
            style={{ minWidth: 96 }}
            options={CONFIDENCE_OPTIONS}
            value={filters.confidenceMin}
            onChange={(confidenceMin) => onFiltersChange({ ...filters, confidenceMin })}
          />
        </div>
        <div className="prediction-table-filter">
          <span className="prediction-table-filter-label">状态</span>
          <Select
            size="small"
            allowClear
            placeholder="全部"
            style={{ minWidth: 96 }}
            options={STATUS_OPTIONS}
            value={filters.status}
            onChange={(status) => onFiltersChange({ ...filters, status })}
          />
        </div>
        <div className="prediction-table-filter">
          <span className="prediction-table-filter-label">过期</span>
          <Select
            size="small"
            allowClear
            placeholder="全部"
            style={{ minWidth: 120 }}
            options={EXPIRED_OPTIONS}
            value={filters.expired}
            onChange={(expired) => onFiltersChange({ ...filters, expired })}
          />
        </div>
        <Button
          size="small"
          icon={<ReloadOutlined />}
          onClick={() => onFiltersChange({})}
        >
          重置
        </Button>
      </div>

      {visibleItems.length === 0 ? (
        <div style={{ padding: '48px 16px' }}>
          <Empty description="暂无符合条件的历史预测" image={Empty.PRESENTED_IMAGE_SIMPLE}>
            <span style={{ color: 'var(--text-tertiary)', fontSize: 12 }}>
              可调整筛选条件或点击「重置」查看全部预测记录
            </span>
          </Empty>
        </div>
      ) : (
        <div
          className="prediction-table-wrapper"
          ref={scrollRef}
          onScroll={handleScroll}
          style={{ maxHeight: SCROLL_VIEWPORT, overflow: 'auto' }}
        >
          <table>
            <thead>
              <tr>
                <th className="col-expand"></th>
                <th className="col-symbol">代码</th>
                <th className="col-name">名称</th>
                <th>
                  <InfoTip tip={PERIOD_INFO_TIPS}>周期</InfoTip>
                </th>
                <th>预测方向</th>
                <th>置信度</th>
                <th className="col-right">目标/实际价</th>
                <th>预测时间</th>
                <th>状态</th>
                <th className="col-expired">过期</th>
              </tr>
            </thead>
            <tbody>
              {topPad > 0 && (
                <tr key="spacer-top" style={{ height: topPad }} aria-hidden>
                  <td colSpan={COL_COUNT} style={{ height: topPad }} />
                </tr>
              )}
              {windowRows.map((row) => {
                const item = row.item;
                if (row.kind === 'detail') {
                  return (
                    <tr
                      key={`detail-${item.id}`}
                      className="prediction-table-detail-row"
                      style={{ height: DETAIL_BLOCK_HEIGHT }}
                    >
                      <td colSpan={COL_COUNT} style={{ padding: 0 }}>
                        <div
                          className="prediction-table-detail"
                          style={{ height: DETAIL_BLOCK_HEIGHT, overflow: 'auto', boxSizing: 'border-box' }}
                        >
                          <div className="prediction-table-detail-grid">
                            <div className="prediction-table-detail-item">
                              <span className="prediction-table-detail-label">有效期至</span>
                              <span className="prediction-table-detail-value">
                                {new Date(item.valid_until).toLocaleString('zh-CN')}
                              </span>
                            </div>
                            <div className="prediction-table-detail-item">
                              <span className="prediction-table-detail-label">模型版本</span>
                              <span className="prediction-table-detail-value">{item.model_version}</span>
                            </div>
                            <div className="prediction-table-detail-item">
                              <span className="prediction-table-detail-label">判定结果</span>
                              <span className="prediction-table-detail-value">
                                {item.is_correct === null ? '尚未判定' : item.is_correct ? '预测正确' : '预测错误'}
                              </span>
                            </div>
                          </div>
                          <div className="prediction-table-detail-factors">
                            <span className="prediction-table-detail-label">关键因子</span>
                            {item.key_factors && item.key_factors.length > 0 ? (
                              <div className="prediction-table-detail-tags">
                                {item.key_factors.map((factor, idx) => (
                                  <span key={idx} className="prediction-table-detail-tag">
                                    {factor}
                                  </span>
                                ))}
                              </div>
                            ) : (
                              <span className="prediction-table-detail-value">暂无</span>
                            )}
                          </div>
                        </div>
                      </td>
                    </tr>
                  );
                }
                const isExpanded = expandedKeys.has(item.id);
                return (
                  <tr
                    key={item.id}
                    className={isExpanded ? 'row-expanded' : 'row-clickable'}
                    style={{ height: ROW_HEIGHT, cursor: 'pointer' }}
                    onClick={() => navigate(`/stocks/${item.symbol}`)}
                  >
                    <td className="col-expand">
                      <button
                        className="prediction-table-expand-btn"
                        onClick={(e) => {
                          e.stopPropagation();
                          toggleExpand(item.id);
                        }}
                      >
                        {isExpanded ? <DownOutlined /> : <RightOutlined />}
                      </button>
                    </td>
                    <td className="col-symbol">{item.symbol}</td>
                    <td className="col-name">{item.name}</td>
                    <td>{PERIOD_LABEL[item.period]}</td>
                    <td>{renderDirection(item.direction, item.confidence)}</td>
                    <td className="col-conf">
                      {(item.confidence * 100).toFixed(0)}%
                    </td>
                    <td className="col-right col-price">
                      {renderPriceCompare(item.target_price, item.actual_price ?? null)}
                    </td>
                    <td className="col-date">
                      {new Date(item.predicted_at).toLocaleString('zh-CN')}
                    </td>
                    <td>{renderStatus(item.status)}</td>
                    <td className="col-expired">
                      <span className={item.expired ? 'prediction-table-expired' : 'prediction-table-valid'}>
                        {item.expired ? '已过期' : '有效'}
                      </span>
                    </td>
                  </tr>
                );
              })}
              {bottomPad > 0 && (
                <tr key="spacer-bottom" style={{ height: bottomPad }} aria-hidden>
                  <td colSpan={COL_COUNT} style={{ height: bottomPad }} />
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {/* 加载更多 */}
      {hasMore && (
        <div className="prediction-table-loadmore">
          <Button
            type="primary"
            ghost
            size="small"
            loading={loadingMore}
            onClick={onLoadMore}
          >
            加载更多
          </Button>
        </div>
      )}
    </div>
  );
}