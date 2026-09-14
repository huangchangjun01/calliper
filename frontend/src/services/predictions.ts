import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import api from '@/services/api';
import type {
  PredictionSummary,
  PredictionDetail,
  PredictionDirection,
  AccuracyTrend,
  StockAccuracy,
  FailureCase,
  PredictionPeriod,
  PredictionStatus,
  PaginatedItems,
  DashboardData,
  HistoryResponse,
  PredictionStat,
  GenerateResponse,
} from '@/types';

// ========== 预测概览 ==========

export function usePredictionSummaries() {
  return useQuery<PredictionSummary[]>({
    queryKey: ['predictions', 'summaries'],
    queryFn: () => api.get('/predictions/summaries'),
    staleTime: 60_000,
  });
}

// ========== 预测详情列表 ==========

export interface PredictionDetailsParams {
  offset?: number;
  limit?: number;
  period?: PredictionPeriod;
  direction?: PredictionDirection;
  confidence_min?: number;
  expired?: boolean;
}

export function usePredictionDetails(params: PredictionDetailsParams = {}) {
  return useQuery<PaginatedItems<PredictionDetail>>({
    queryKey: ['predictions', 'details', params],
    queryFn: () => api.get('/predictions/details', { ...params }),
    staleTime: 60_000,
  });
}

/** 预测历史（复用细节条目，语义化命名） */
export function usePredictionHistory(params: PredictionDetailsParams = {}) {
  return usePredictionDetails(params);
}

// ========== 预测历史记录（GET /predictions/history，服务端分页） ==========

export interface HistoryQueryParams {
  symbol?: string;
  /** short_term | medium_term | long_term */
  period?: string;
  status?: PredictionStatus;
  /** YYYY-MM-DD */
  from?: string;
  /** YYYY-MM-DD */
  to?: string;
  offset: number;
  limit: number;
}

export function useHistoryQuery(params: HistoryQueryParams) {
  return useQuery<HistoryResponse>({
    queryKey: ['predictions', 'history', params],
    queryFn: () => api.get('/predictions/history', { ...params }),
    staleTime: 30_000,
  });
}

// ========== 分时段预测统计（GET /predictions/stats） ==========

export interface PredictionStatsParams {
  /** day | week | month */
  bucket: 'day' | 'week' | 'month';
  /** YYYY-MM-DD */
  from?: string;
  /** YYYY-MM-DD */
  to?: string;
}

export function usePredictionStats(params: PredictionStatsParams) {
  return useQuery<PredictionStat[]>({
    queryKey: ['predictions', 'stats', params],
    queryFn: () => api.get('/predictions/stats', { ...params }),
    staleTime: 60_000,
  });
}

// ========== 生成预测（POST /predictions/generate） ==========

export function useGeneratePredictions() {
  const queryClient = useQueryClient();
  return useMutation<GenerateResponse, Error, { symbols: string[] }>({
    mutationFn: (vars) => api.post('/predictions/generate', vars),
    onSuccess: () => {
      // 新预测落地后，历史/详情/统计等预测类查询全部失效重建
      queryClient.invalidateQueries({ queryKey: ['predictions'] });
      queryClient.invalidateQueries({ queryKey: ['dashboard'] });
    },
  });
}

// ========== 准确率趋势 ==========

export function useAccuracyTrend({
  period = 'short',
  days,
}: { period?: PredictionPeriod; days?: number } = {}) {
  return useQuery<AccuracyTrend>({
    queryKey: ['predictions', 'accuracy', period, days],
    queryFn: () => api.get('/predictions/accuracy', { period, days }),
    staleTime: 60_000,
  });
}

// ========== 股票准确率排行 ==========

export function useStockAccuracyRanking() {
  return useQuery<StockAccuracy[]>({
    queryKey: ['predictions', 'stock-accuracy'],
    queryFn: () => api.get('/predictions/stock-accuracy'),
    staleTime: 60_000,
  });
}

// ========== 失败案例 ==========

export function useFailureCases(limit?: number) {
  return useQuery<FailureCase[]>({
    queryKey: ['predictions', 'failures', limit],
    queryFn: () => api.get('/predictions/failures', { limit }),
    staleTime: 60_000,
  });
}

// ========== Dashboard 决策支持 ==========

export function useDashboard() {
  return useQuery<DashboardData>({
    queryKey: ['dashboard'],
    queryFn: () => api.get('/dashboard'),
    staleTime: 60_000,
    refetchInterval: 60_000,
  });
}