import { useCallback, useEffect, useMemo, useState } from 'react';
import { Button, DatePicker, Input, Modal, Select, Table, Tag, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import type { ReactNode } from 'react';
import {
  CaretUpOutlined,
  CaretDownOutlined,
  MinusOutlined,
  SearchOutlined,
  ReloadOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import { useSearchParams } from 'react-router-dom';
import dayjs from 'dayjs';
import type { Dayjs } from 'dayjs';
import DataState from '@/components/common/DataState';
import InfoTip from '@/components/common/InfoTip';
import { useGeneratePredictions, useHistoryQuery } from '@/services/predictions';
import type { GenerateResponse, PredictionDirection, PredictionHistoryItem, PredictionStatus } from '@/types';
import { PERIOD_INFO, PERIOD_INFO_TIPS } from '@/constants/predictions';
import './index.css';

const HISTORY_LIMIT = 20;

/** 历史记录筛选条件在 URL 中的参数键（h 前缀避免与预测列表筛选冲突） */
const URL_KEYS = ['hsym', 'hperiod', 'hstatus', 'hfrom', 'hto', 'hoffset'] as const;

const DEFAULT_GENERATE_SYMBOLS = '000001,000002,600519,600036,000858';

const HISTORY_PERIOD_OPTIONS = [
  { value: 'short_term', label: '短期' },
  { value: 'medium_term', label: '中短期' },
  { value: 'long_term', label: '长期' },
];

const HISTORY_STATUS_OPTIONS = [
  { value: 'pending', label: '待验证' },
  { value: 'correct', label: '已对' },
  { value: 'wrong', label: '已错' },
];

const PERIOD_LABEL: Record<string, string> = {
  short: PERIOD_INFO.short.label,
  medium: PERIOD_INFO.medium.label,
  long: PERIOD_INFO.long.label,
  short_term: PERIOD_INFO.short.label,
  medium_term: PERIOD_INFO.medium.label,
  long_term: PERIOD_INFO.long.label,
};

// 前端周期筛选项 → 后端 DB 存储值映射（后端按 short/medium/long 精确匹配）
const HISTORY_PERIOD_API: Record<string, string> = {
  short_term: 'short',
  medium_term: 'medium',
  long_term: 'long',
};

const DIRECTION_CONFIG: Record<PredictionDirection, { label: string; color: string; icon: ReactNode }> = {
  up: { label: '看涨', color: 'var(--color-error)', icon: <CaretUpOutlined /> },
  down: { label: '看跌', color: 'var(--color-success)', icon: <CaretDownOutlined /> },
  flat: { label: '震荡', color: 'var(--text-tertiary)', icon: <MinusOutlined /> },
};

const STATUS_TAG: Record<PredictionStatus, { label: string; color: string }> = {
  pending: { label: '待验证', color: 'blue' },
  correct: { label: '已对', color: 'green' },
  wrong: { label: '已错', color: 'red' },
};

export default function PredictionHistory() {
  const [searchParams, setSearchParams] = useSearchParams();

  // URL 中已提交的筛选条件（驱动查询）
  const symbol = searchParams.get('hsym') ?? '';
  const period = searchParams.get('hperiod') ?? undefined;
  const status = (searchParams.get('hstatus') as PredictionStatus | null) ?? undefined;
  const from = searchParams.get('hfrom') ?? undefined;
  const to = searchParams.get('hto') ?? undefined;
  const offset = useMemo(() => {
    const raw = Number(searchParams.get('hoffset'));
    return Number.isFinite(raw) && raw > 0 ? Math.floor(raw) : 0;
  }, [searchParams]);

  const queryParams = useMemo(
    () => ({
      symbol: symbol || undefined,
      // 筛选值（short_term/…）映射为后端期望的存储值（short/medium/long）
      period: period ? (HISTORY_PERIOD_API[period] ?? period) : undefined,
      status,
      from,
      to,
      offset,
      limit: HISTORY_LIMIT,
    }),
    [symbol, period, status, from, to, offset]
  );

  const { data, isLoading, error, refetch } = useHistoryQuery(queryParams);

  // 筛选草稿：点击「查询」后才提交到 URL，避免输入过程频繁触发请求
  const [draftSymbol, setDraftSymbol] = useState(symbol);
  const [draftPeriod, setDraftPeriod] = useState<string | undefined>(period);
  const [draftStatus, setDraftStatus] = useState<PredictionStatus | undefined>(status);
  const [draftRange, setDraftRange] = useState<[Dayjs | null, Dayjs | null] | null>(() =>
    from || to ? [from ? dayjs(from) : null, to ? dayjs(to) : null] : null
  );

  // URL 中的筛选条件被外部变更（如浏览器回退）时，同步草稿
  useEffect(() => {
    setDraftSymbol(symbol);
    setDraftPeriod(period);
    setDraftStatus(status);
    setDraftRange(from || to ? [from ? dayjs(from) : null, to ? dayjs(to) : null] : null);
  }, [symbol, period, status, from, to]);

  /** 提交草稿筛选到 URL（筛选条件变化时重置页码 offset=0） */
  const applyFilters = useCallback(() => {
    const p = new URLSearchParams(searchParams);
    p.delete('hoffset');
    const fromVal = draftRange?.[0] ? draftRange[0].format('YYYY-MM-DD') : undefined;
    const toVal = draftRange?.[1] ? draftRange[1].format('YYYY-MM-DD') : undefined;
    if (draftSymbol.trim()) p.set('hsym', draftSymbol.trim());
    else p.delete('hsym');
    if (draftPeriod) p.set('hperiod', draftPeriod);
    else p.delete('hperiod');
    if (draftStatus) p.set('hstatus', draftStatus);
    else p.delete('hstatus');
    if (fromVal) p.set('hfrom', fromVal);
    else p.delete('hfrom');
    if (toVal) p.set('hto', toVal);
    else p.delete('hto');
    setSearchParams(p, { replace: true });
  }, [searchParams, setSearchParams, draftSymbol, draftPeriod, draftStatus, draftRange]);

  const resetFilters = useCallback(() => {
    setDraftSymbol('');
    setDraftPeriod(undefined);
    setDraftStatus(undefined);
    setDraftRange(null);
    const p = new URLSearchParams(searchParams);
    for (const key of URL_KEYS) p.delete(key);
    setSearchParams(p, { replace: true });
  }, [searchParams, setSearchParams]);

  /** 服务端分页：页码变化时更新 offset */
  const handlePageChange = useCallback(
    (page: number, pageSize: number) => {
      const nextOffset = (page - 1) * pageSize;
      const p = new URLSearchParams(searchParams);
      if (nextOffset > 0) p.set('hoffset', String(nextOffset));
      else p.delete('hoffset');
      setSearchParams(p, { replace: true });
    },
    [searchParams, setSearchParams]
  );

  // ===== 生成预测 =====
  const [genOpen, setGenOpen] = useState(false);
  const [genSymbols, setGenSymbols] = useState(DEFAULT_GENERATE_SYMBOLS);
  const [genResults, setGenResults] = useState<GenerateResponse['results'] | null>(null);
  const generateMutation = useGeneratePredictions();

  const handleOpenGenerate = useCallback(() => {
    setGenResults(null);
    setGenOpen(true);
  }, []);

  const handleGenerate = useCallback(async () => {
    const symbols = Array.from(
      new Set(
        genSymbols
          .split(/[,，\s]+/)
          .map((s) => s.trim())
          .filter(Boolean)
      )
    );
    if (symbols.length === 0) {
      message.warning('请输入至少一个股票代码');
      return;
    }
    try {
      const res = await generateMutation.mutateAsync({ symbols });
      setGenResults(res.results);
      message.success(`生成完成，共持久化 ${res.persisted} 条预测`);
    } catch (err) {
      message.error(err instanceof Error ? err.message : '生成预测失败');
    }
  }, [genSymbols, generateMutation]);

  const columns: ColumnsType<PredictionHistoryItem> = [
    { title: '代码', dataIndex: 'symbol', key: 'symbol', width: 90 },
    { title: '名称', dataIndex: 'name', key: 'name', width: 110, ellipsis: true },
    {
      title: <InfoTip tip={PERIOD_INFO_TIPS}>周期</InfoTip>,
      dataIndex: 'period',
      key: 'period',
      width: 90,
      render: (v: string) => PERIOD_LABEL[v] ?? v,
    },
    {
      title: '方向',
      dataIndex: 'direction',
      key: 'direction',
      width: 84,
      render: (v: PredictionDirection) => {
        const cfg = DIRECTION_CONFIG[v] ?? DIRECTION_CONFIG.flat;
        return (
          <span style={{ color: cfg.color }}>
            {cfg.icon}
            <span style={{ marginLeft: 2 }}>{cfg.label}</span>
          </span>
        );
      },
    },
    {
      title: '置信度',
      dataIndex: 'confidence',
      key: 'confidence',
      width: 80,
      align: 'right',
      render: (v: number) => `${(v * 100).toFixed(0)}%`,
    },
    {
      title: '目标价',
      dataIndex: 'target_price',
      key: 'target_price',
      width: 90,
      align: 'right',
      render: (v: number) => (v !== null && v !== undefined ? `¥${v.toFixed(2)}` : '--'),
    },
    {
      title: '实际价',
      key: 'actual_price',
      width: 130,
      align: 'right',
      render: (_: unknown, record: PredictionHistoryItem) => {
        const target = record.target_price;
        const actual = record.actual_price;
        const hasActual =
          actual !== null && actual !== undefined && !Number.isNaN(actual);
        const a = hasActual ? `¥${actual.toFixed(2)}` : '--';
        const hasTarget =
          target !== null && target !== undefined && !Number.isNaN(target);
        const diff =
          hasActual && hasTarget && target !== 0
            ? ((actual - target) / target) * 100
            : null;
        return (
          <span style={{ whiteSpace: 'nowrap' }}>
            {a}
            {diff !== null && (
              <span
                style={{
                  marginLeft: 4,
                  fontSize: 12,
                  color: diff >= 0 ? 'var(--color-error, #ef5350)' : 'var(--color-success, #26a69a)',
                }}
              >
                {diff >= 0 ? '+' : ''}
                {diff.toFixed(1)}%
              </span>
            )}
          </span>
        );
      },
    },
    {
      title: '预测时间',
      dataIndex: 'predicted_at',
      key: 'predicted_at',
      width: 165,
      render: (v: string) => new Date(v).toLocaleString('zh-CN'),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 90,
      render: (v: PredictionStatus) => {
        const cfg = STATUS_TAG[v] ?? STATUS_TAG.pending;
        return <Tag color={cfg.color}>{cfg.label}</Tag>;
      },
    },
  ];

  const items = data?.items ?? [];
  const total = data?.total ?? 0;

  return (
    <div className="prediction-history">
      <div className="prediction-history-header">
        <span className="prediction-history-count">共 {total} 条记录</span>
        <Button
          size="small"
          icon={<ThunderboltOutlined />}
          onClick={handleOpenGenerate}
        >
          生成预测
        </Button>
      </div>

      <div className="prediction-history-filters">
        <div className="prediction-history-filter">
          <span className="prediction-history-filter-label">代码</span>
          <Input
            size="small"
            allowClear
            placeholder="如 000001"
            style={{ width: 120 }}
            value={draftSymbol}
            onChange={(e) => setDraftSymbol(e.target.value)}
            onPressEnter={applyFilters}
          />
        </div>
        <div className="prediction-history-filter">
          <span className="prediction-history-filter-label">
            <InfoTip tip={PERIOD_INFO_TIPS}>周期</InfoTip>
          </span>
          <Select
            size="small"
            allowClear
            placeholder="全部"
            style={{ minWidth: 96 }}
            options={HISTORY_PERIOD_OPTIONS}
            value={draftPeriod}
            onChange={(v) => setDraftPeriod(v)}
          />
        </div>
        <div className="prediction-history-filter">
          <span className="prediction-history-filter-label">状态</span>
          <Select
            size="small"
            allowClear
            placeholder="全部"
            style={{ minWidth: 96 }}
            options={HISTORY_STATUS_OPTIONS}
            value={draftStatus}
            onChange={(v) => setDraftStatus(v)}
          />
        </div>
        <div className="prediction-history-filter">
          <span className="prediction-history-filter-label">日期</span>
          <DatePicker.RangePicker
            size="small"
            value={draftRange}
            onChange={(dates) => setDraftRange(dates)}
          />
        </div>
        <Button size="small" type="primary" icon={<SearchOutlined />} onClick={applyFilters}>
          查询
        </Button>
        <Button size="small" icon={<ReloadOutlined />} onClick={resetFilters}>
          重置
        </Button>
      </div>

      <div className="prediction-history-body">
        <DataState
          loading={isLoading}
          error={error instanceof Error ? error.message : null}
          onRetry={() => refetch()}
          isEmpty={items.length === 0}
          emptyText="暂无符合条件的历史预测"
          emptyDescription="可调整筛选条件或点击「重置」查看全部记录"
        >
          <Table<PredictionHistoryItem>
            rowKey="id"
            size="small"
            columns={columns}
            dataSource={items}
            scroll={{ x: 860 }}
            pagination={{
              current: Math.floor(offset / HISTORY_LIMIT) + 1,
              pageSize: HISTORY_LIMIT,
              total,
              showSizeChanger: false,
              showTotal: (t) => `共 ${t} 条`,
              onChange: handlePageChange,
            }}
          />
        </DataState>
      </div>

      <Modal
        title="生成预测"
        open={genOpen}
        onCancel={() => setGenOpen(false)}
        footer={[
          <Button key="close" onClick={() => setGenOpen(false)}>
            关闭
          </Button>,
          <Button
            key="generate"
            type="primary"
            loading={generateMutation.isPending}
            onClick={handleGenerate}
          >
            生成
          </Button>,
        ]}
      >
        <div className="prediction-history-gen-tip">
          输入股票代码（逗号分隔），调用模型生成新的预测：
        </div>
        <Input
          value={genSymbols}
          onChange={(e) => setGenSymbols(e.target.value)}
          placeholder="例如：000001,000002,600519"
        />
        {genResults && (
          <div className="prediction-history-gen-results">
            {genResults.map((r) => (
              <span key={r.symbol} className="prediction-history-gen-result">
                <span className="prediction-history-gen-symbol">{r.symbol}</span>
                {r.status === 'predicted' ? (
                  <Tag color="green">已生成 {r.count} 条</Tag>
                ) : (
                  <Tag>无数据</Tag>
                )}
              </span>
            ))}
          </div>
        )}
      </Modal>
    </div>
  );
}
