import { useQuery } from '@tanstack/react-query';
import api from '@/services/api';
import type {
  PredictionSummary,
  PredictionDetail,
  AccuracyTrend,
  StockAccuracy,
  FailureCase,
  PredictionPeriod,
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

export function usePredictionDetails() {
  return useQuery<PredictionDetail[]>({
    queryKey: ['predictions', 'details'],
    queryFn: () => api.get('/predictions/details'),
    staleTime: 60_000,
  });
}

// ========== 准确率趋势 ==========

export function useAccuracyTrend(period: PredictionPeriod = 'short') {
  return useQuery<AccuracyTrend>({
    queryKey: ['predictions', 'accuracy', period],
    queryFn: () => api.get('/predictions/accuracy', { period }),
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

export function useFailureCases() {
  return useQuery<FailureCase[]>({
    queryKey: ['predictions', 'failures'],
    queryFn: () => api.get('/predictions/failures'),
    staleTime: 60_000,
  });
}