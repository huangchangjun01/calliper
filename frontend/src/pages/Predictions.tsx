import { useCallback, useMemo } from 'react';
import { useInfiniteQuery } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import PredictionCard from '@/components/PredictionCard';
import PredictionTable from '@/components/PredictionTable';
import AccuracyChart from '@/components/AccuracyChart';
import StockAccuracyChart from '@/components/StockAccuracyChart';
import PredictionHistory from '@/components/PredictionHistory';
import PredictionStats from '@/components/PredictionStats';
import FailureAnalysis from '@/components/FailureAnalysis';
import InfoTip from '@/components/common/InfoTip';
import {
  usePredictionSummaries,
  useAccuracyTrend,
  useStockAccuracyRanking,
  useFailureCases,
} from '@/services/predictions';
import api from '@/services/api';
import type {
  PredictionPeriod,
  PredictionFilters,
  PredictionDetail,
  PredictionDirection,
  PredictionStatus,
  PaginatedItems,
} from '@/types';
import './Predictions.css';

const PAGE_SIZE = 30;

const PERIODS: PredictionPeriod[] = ['short', 'medium', 'long'];
const DIRECTIONS: PredictionDirection[] = ['up', 'down', 'flat'];
const STATUSES: PredictionStatus[] = ['pending', 'correct', 'wrong'];

/** 从 URL 搜索参数解析预测筛选条件，保证离开/刷新后 tab 与筛选状态得以保留 */
function parseFilters(searchParams: URLSearchParams): PredictionFilters {
  const f: PredictionFilters = {};
  const period = searchParams.get('period') as PredictionPeriod | null;
  const direction = searchParams.get('direction') as PredictionDirection | null;
  const status = searchParams.get('status') as PredictionStatus | null;
  const conf = searchParams.get('conf');
  const expired = searchParams.get('expired');

  if (period && PERIODS.includes(period)) f.period = period;
  if (direction && DIRECTIONS.includes(direction)) f.direction = direction;
  if (conf && !Number.isNaN(Number(conf))) f.confidenceMin = Number(conf);
  if (status && STATUSES.includes(status)) f.status = status;
  if (expired === 'true') f.expired = true;
  else if (expired === 'false') f.expired = false;
  return f;
}

function parseAccuracyPeriod(searchParams: URLSearchParams): PredictionPeriod {
  const acc = searchParams.get('acc');
  return acc === 'medium' || acc === 'long' ? acc : 'short';
}

export default function Predictions() {
  const [searchParams, setSearchParams] = useSearchParams();
  const accuracyPeriod = useMemo(() => parseAccuracyPeriod(searchParams), [searchParams]);
  const filters = useMemo(() => parseFilters(searchParams), [searchParams]);

  const { data: summaries = [] } = usePredictionSummaries();

  // 后端过滤参数（status 在客户端过滤，不外传）
  const backendParams = useMemo(
    () => ({
      period: filters.period,
      direction: filters.direction,
      confidence_min: filters.confidenceMin,
      expired: filters.expired,
    }),
    [filters.period, filters.direction, filters.confidenceMin, filters.expired]
  );

  const {
    data: pageData,
    isLoading: detailsLoading,
    isFetchingNextPage: loadingMore,
    hasNextPage,
    fetchNextPage,
  } = useInfiniteQuery<PaginatedItems<PredictionDetail>>({
    queryKey: ['predictions', 'details', backendParams],
    queryFn: ({ pageParam }) =>
      api.get('/predictions/details', {
        ...backendParams,
        offset: pageParam as number,
        limit: PAGE_SIZE,
      }),
    initialPageParam: 0,
    getNextPageParam: (lastPage) => {
      const nextOffset = lastPage.offset + lastPage.limit;
      return nextOffset < lastPage.total ? nextOffset : undefined;
    },
    staleTime: 60_000,
  });

  const details = useMemo(
    () => pageData?.pages.flatMap((p) => p.items) ?? [],
    [pageData]
  );
  const totalCount = pageData?.pages[pageData.pages.length - 1]?.total ?? details.length;

  const { data: accuracyTrend, isLoading: accuracyLoading } = useAccuracyTrend({
    period: accuracyPeriod,
  });
  const { data: stockAccuracy = [], isLoading: stockAccuracyLoading } = useStockAccuracyRanking();
  const { data: failureCases = [], isLoading: failuresLoading } = useFailureCases();

  // 将筛选条件写回 URL，保证离开/刷新后 filter 与 tab 得以保留
  const handleFiltersChange = useCallback((next: PredictionFilters) => {
    const p = new URLSearchParams(searchParams);
    const setter: Array<[string, string | undefined]> = [
      ['period', next.period],
      ['direction', next.direction],
      ['status', next.status],
    ];
    for (const [key, value] of setter) {
      if (value) p.set(key, value);
      else p.delete(key);
    }
    if (next.confidenceMin !== undefined) p.set('conf', String(next.confidenceMin));
    else p.delete('conf');
    if (next.expired === true) p.set('expired', 'true');
    else if (next.expired === false) p.set('expired', 'false');
    else p.delete('expired');
    setSearchParams(p, { replace: true });
  }, [searchParams, setSearchParams]);

  const handleAccuracyPeriodChange = useCallback((period: PredictionPeriod) => {
    const p = new URLSearchParams(searchParams);
    if (period === 'short') p.delete('acc');
    else p.set('acc', period);
    setSearchParams(p, { replace: true });
  }, [searchParams, setSearchParams]);

  const handleLoadMore = useCallback(() => {
    fetchNextPage();
  }, [fetchNextPage]);

  return (
    <div className="predictions">
      {/* 顶部：预测概览面板 */}
      <section className="predictions-section">
        <h2 className="predictions-section-title">
          <InfoTip tip="按短期/中短期/长期三个预测周期，分别统计看涨、看跌、震荡预测的数量占比，帮助快速了解模型整体观点分布。">预测概览</InfoTip>
        </h2>
        <div className="predictions-overview-grid">
          {summaries.map((summary) => (
            <PredictionCard key={summary.period} data={summary} period={summary.period} />
          ))}
        </div>
      </section>

      {/* 中部：预测列表 */}
      <section className="predictions-section">
        <PredictionTable
          items={details}
          total={totalCount}
          loading={detailsLoading}
          loadingMore={loadingMore}
          hasMore={!!hasNextPage}
          onLoadMore={handleLoadMore}
          filters={filters}
          onFiltersChange={handleFiltersChange}
        />
      </section>

      {/* 底部：预测成功率统计 */}
      <section className="predictions-section">
        <h2 className="predictions-section-title">预测成功率统计</h2>

        <div className="predictions-accuracy-grid">
          <div className="predictions-accuracy-line">
            <AccuracyChart
              data={accuracyTrend}
              loading={accuracyLoading}
              onPeriodChange={handleAccuracyPeriodChange}
              currentPeriod={accuracyPeriod}
            />
          </div>
          <div className="predictions-accuracy-bar">
            <StockAccuracyChart data={stockAccuracy} loading={stockAccuracyLoading} />
          </div>
        </div>
      </section>

      {/* 底部：预测失败归因分析（紧随成功率统计，便于发现模型失准模式） */}
      <section className="predictions-section">
        <FailureAnalysis data={failureCases} loading={failuresLoading} />
      </section>

      {/* 预测历史记录：服务端分页 + 筛选 + 生成预测 */}
      <section className="predictions-section">
        <h2 className="predictions-section-title">预测历史记录</h2>
        <PredictionHistory />
      </section>

      {/* 分时段预测统计：日/周/月聚合（组件内自带标题） */}
      <section className="predictions-section">
        <PredictionStats />
      </section>
    </div>
  );
}