import { useMemo, useState } from 'react';
import { Segmented, Table } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import dayjs from 'dayjs';
import isoWeek from 'dayjs/plugin/isoWeek';
import DataState from '@/components/common/DataState';
import InfoTip from '@/components/common/InfoTip';
import { usePredictionStats } from '@/services/predictions';
import type { PredictionStat } from '@/types';
import './index.css';

dayjs.extend(isoWeek);

type StatsBucket = 'day' | 'week' | 'month';

const BUCKET_OPTIONS = [
  { label: '日', value: 'day' },
  { label: '周', value: 'week' },
  { label: '月', value: 'month' },
];

const BUCKET_UNIT_LABEL: Record<StatsBucket, string> = {
  day: '日',
  week: '周',
  month: '月',
};

/** 将后端 bucket 键格式化为便于阅读的时段标签 */
function formatBucket(bucketKey: string, bucketType: StatsBucket): { primary: string; secondary: string } {
  if (bucketType === 'week') {
    const m = bucketKey.match(/^(\d{4})-W(\d{1,2})$/);
    if (m) {
      const start = dayjs().year(Number(m[1])).isoWeek(Number(m[2])).startOf('week');
      const end = start.add(6, 'day');
      return { primary: `${start.format('MM-DD')} ~ ${end.format('MM-DD')}`, secondary: `${m[1]} 年第 ${m[2]} 周` };
    }
    return { primary: bucketKey, secondary: '' };
  }
  if (bucketType === 'month') {
    return { primary: bucketKey, secondary: '当月' };
  }
  const d = dayjs(bucketKey);
  return { primary: d.format('MM-DD'), secondary: d.format('YYYY 年') };
}

export default function PredictionStats() {
  const [bucket, setBucket] = useState<StatsBucket>('day');

  // 默认区间：近 30 天（含今日）
  const { from, to } = useMemo(
    () => ({
      from: dayjs().subtract(29, 'day').format('YYYY-MM-DD'),
      to: dayjs().format('YYYY-MM-DD'),
    }),
    []
  );

  const { data, isLoading, error, refetch } = usePredictionStats({ bucket, from, to });

  /** 汇总数据（全部时段合计） */
  const summary = useMemo(() => {
    let total = 0;
    let correct = 0;
    let wrong = 0;
    let pending = 0;
    const accs: number[] = [];
    for (const it of data ?? []) {
      total += it.total;
      correct += it.correct;
      wrong += it.wrong;
      pending += it.pending;
      if (it.accuracy !== null) accs.push(it.accuracy);
    }
    const avgAcc = accs.length > 0 ? accs.reduce((s, v) => s + v, 0) / accs.length : null;
    return { total, correct, wrong, pending, avgAcc };
  }, [data]);

  /** 倒序：最近时段在最上方，便于查看最新趋势 */
  const items = useMemo(() => [...(data ?? [])].reverse(), [data]);

  const columns: ColumnsType<PredictionStat> = [
    {
      title: '时段',
      dataIndex: 'bucket',
      key: 'bucket',
      width: 170,
      render: (v: string) => {
        const { primary, secondary } = formatBucket(v, bucket);
        return (
          <span className="prediction-stats-bucket">
            <span className="prediction-stats-bucket-primary">{primary}</span>
            {secondary && <span className="prediction-stats-bucket-secondary">{secondary}</span>}
          </span>
        );
      },
    },
    {
      title: <InfoTip tip="该时段内全部预测记录数（含待验证/已对/已错）"><span>预测总数</span></InfoTip>,
      dataIndex: 'total',
      key: 'total',
      width: 90,
      align: 'right',
      render: (v: number) => <span className="prediction-stats-total">{v}</span>,
    },
    {
      title: <InfoTip tip="已对与已错构成已评估部分的比例，剩余为待验证"><span>判定构成</span></InfoTip>,
      key: 'composition',
      width: 190,
      render: (_: unknown, record: PredictionStat) => {
        if (record.total === 0) return <span className="prediction-stats-num">--</span>;
        const pend = record.pending;
        return (
          <div className="prediction-stats-composition">
            <div className="prediction-stats-composition-bar">
              {record.correct > 0 && (
                <i
                  className="prediction-stats-composition-correct"
                  title={`已对 ${record.correct}`}
                  style={{ width: `${(record.correct / record.total) * 100}%` }}
                />
              )}
              {record.wrong > 0 && (
                <i
                  className="prediction-stats-composition-wrong"
                  title={`已错 ${record.wrong}`}
                  style={{ width: `${(record.wrong / record.total) * 100}%` }}
                />
              )}
              {pend > 0 && (
                <i
                  className="prediction-stats-composition-pending"
                  title={`待验证 ${pend}`}
                  style={{ width: `${(pend / record.total) * 100}%` }}
                />
              )}
            </div>
            <span className="prediction-stats-composition-detail">
              <em className="prediction-stats-composition-detail-correct">对{record.correct}</em>
              <em className="prediction-stats-composition-detail-wrong">错{record.wrong}</em>
              <em className="prediction-stats-composition-detail-pending">待{pend}</em>
            </span>
          </div>
        );
      },
    },
    {
      title: <InfoTip tip="该时段已评估预测中判定正确的比例；无评估数据的时段显示为断点"><span>准确率</span></InfoTip>,
      dataIndex: 'accuracy',
      key: 'accuracy',
      width: 200,
      align: 'right',
      render: (v: number | null) => {
        if (v === null) {
          return <span className="prediction-stats-breakpoint">—（断点）</span>;
        }
        return (
          <div className="prediction-stats-accuracy">
            <div className="prediction-stats-accuracy-track">
              <div className="prediction-stats-accuracy-fill" style={{ width: `${v}%` }} />
            </div>
            <span className="prediction-stats-accuracy-value">{v.toFixed(1)}%</span>
          </div>
        );
      },
    },
  ];

  return (
    <div className="prediction-stats">
      <div className="prediction-stats-header">
        <div className="prediction-stats-header-left">
          <span className="prediction-stats-title">分时段统计</span>
          <span className="prediction-stats-range">
            {from} ~ {to}（按{BUCKET_UNIT_LABEL[bucket]}聚合）
          </span>
        </div>
        <Segmented
          size="small"
          options={BUCKET_OPTIONS}
          value={bucket}
          onChange={(v) => setBucket(v as StatsBucket)}
        />
      </div>

      {/* 汇总指标条 */}
      <div className="prediction-stats-summary">
        <div className="prediction-stats-summary-item">
          <span className="prediction-stats-summary-label">预测总数</span>
          <span className="prediction-stats-summary-value">{summary.total}</span>
        </div>
        <div className="prediction-stats-summary-item prediction-stats-summary-item--correct">
          <span className="prediction-stats-summary-label">已对</span>
          <span className="prediction-stats-summary-value">{summary.correct}</span>
        </div>
        <div className="prediction-stats-summary-item prediction-stats-summary-item--wrong">
          <span className="prediction-stats-summary-label">已错</span>
          <span className="prediction-stats-summary-value">{summary.wrong}</span>
        </div>
        <div className="prediction-stats-summary-item prediction-stats-summary-item--pending">
          <span className="prediction-stats-summary-label">待验证</span>
          <span className="prediction-stats-summary-value">{summary.pending}</span>
        </div>
        <div className="prediction-stats-summary-item prediction-stats-summary-item--accuracy">
          <span className="prediction-stats-summary-label">平均准确率</span>
          <span className="prediction-stats-summary-value">
            {summary.avgAcc === null ? '—' : `${summary.avgAcc.toFixed(1)}%`}
          </span>
        </div>
      </div>

      <div className="prediction-stats-body">
        <DataState
          loading={isLoading}
          error={error instanceof Error ? error.message : null}
          onRetry={() => refetch()}
          isEmpty={items.length === 0}
          emptyText="暂无分时段统计数据"
        >
          <Table<PredictionStat>
            rowKey="bucket"
            size="small"
            columns={columns}
            dataSource={items}
            pagination={false}
            onRow={(record) => ({
              style: { opacity: record.total === 0 ? 0.45 : 1 },
            })}
          />
        </DataState>
      </div>

      <div className="prediction-stats-footer">
        <span>说明：「—（断点）」表示该时段无已评估预测，无法计算准确率；总数为 0 的时段整行淡显；列表按时间倒序，最近时段在最上方。</span>
      </div>
    </div>
  );
}