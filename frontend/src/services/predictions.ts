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
    placeholderData: getMockSummaries(),
  });
}

// ========== 预测详情列表 ==========

export function usePredictionDetails() {
  return useQuery<PredictionDetail[]>({
    queryKey: ['predictions', 'details'],
    queryFn: () => api.get('/predictions/details'),
    staleTime: 60_000,
    placeholderData: getMockDetails(),
  });
}

// ========== 准确率趋势 ==========

export function useAccuracyTrend(period: PredictionPeriod = 'short') {
  return useQuery<AccuracyTrend>({
    queryKey: ['predictions', 'accuracy', period],
    queryFn: () => api.get('/predictions/accuracy', { period }),
    staleTime: 60_000,
    placeholderData: getMockAccuracyTrend(period),
  });
}

// ========== 股票准确率排行 ==========

export function useStockAccuracyRanking() {
  return useQuery<StockAccuracy[]>({
    queryKey: ['predictions', 'stock-accuracy'],
    queryFn: () => api.get('/predictions/stock-accuracy'),
    staleTime: 60_000,
    placeholderData: getMockStockAccuracy(),
  });
}

// ========== 失败案例 ==========

export function useFailureCases() {
  return useQuery<FailureCase[]>({
    queryKey: ['predictions', 'failures'],
    queryFn: () => api.get('/predictions/failures'),
    staleTime: 60_000,
    placeholderData: getMockFailures(),
  });
}

// ========== Mock 数据 ==========

function getMockSummaries(): PredictionSummary[] {
  return [
    { period: 'short', periodLabel: '短期预测', total: 48, upCount: 22, downCount: 18, flatCount: 8 },
    { period: 'medium', periodLabel: '中短期预测', total: 48, upCount: 28, downCount: 12, flatCount: 8 },
    { period: 'long', periodLabel: '长期预测', total: 48, upCount: 32, downCount: 10, flatCount: 6 },
  ];
}

function getMockDetails(): PredictionDetail[] {
  const stocks = [
    { symbol: '600519.SH', name: '贵州茅台' },
    { symbol: '000858.SZ', name: '五 粮 液' },
    { symbol: '300750.SZ', name: '宁德时代' },
    { symbol: '601318.SH', name: '中国平安' },
    { symbol: '000333.SZ', name: '美的集团' },
    { symbol: '600036.SH', name: '招商银行' },
    { symbol: '601012.SH', name: '隆基绿能' },
    { symbol: '002594.SZ', name: '比 亚 迪' },
    { symbol: '300059.SZ', name: '东方财富' },
    { symbol: '600276.SH', name: '恒瑞医药' },
    { symbol: '000651.SZ', name: '格力电器' },
    { symbol: '601166.SH', name: '兴业银行' },
  ];

  const directions: Array<'up' | 'down' | 'flat'> = ['up', 'down', 'flat'];
  const keyFactorsPool = [
    ['MACD金叉', '成交量放大', '北向资金流入'],
    ['KDJ超卖', '布林下轨支撑', '估值修复'],
    ['RSI多头', '均线多头排列', '行业政策利好'],
    ['MACD死叉', '量价背离', '主力资金流出'],
    ['横盘整理', '布林收窄', '等待方向选择'],
    ['突破前高', '资金持续流入', '业绩超预期'],
    ['跌破支撑', '恐慌抛售', '行业利空'],
    ['底部放量', '机构增持', '技术反弹需求'],
  ];

  return stocks.map((s, i) => {
    const dirIdx = i % 3;
    return {
      id: `pred-${i + 1}`,
      symbol: s.symbol,
      name: s.name,
      short: {
        direction: directions[(dirIdx + (i % 2)) % 3],
        confidence: 0.65 + Math.random() * 0.3,
      },
      medium: {
        direction: directions[(dirIdx + ((i + 1) % 2)) % 3],
        confidence: 0.55 + Math.random() * 0.35,
      },
      long: {
        direction: directions[(dirIdx + ((i + 2) % 2)) % 3],
        confidence: 0.5 + Math.random() * 0.4,
      },
      targetPrice: 100 + Math.random() * 500,
      keyFactors: keyFactorsPool[i % keyFactorsPool.length],
      modelVersion: `v${2 + Math.floor(i / 4)}.${i % 4}.0`,
      updatedAt: new Date(Date.now() - i * 3600000).toISOString(),
    };
  });
}

function getMockAccuracyTrend(period: PredictionPeriod): AccuracyTrend {
  const days = 30;
  const data = [];
  const baseAccuracy = period === 'short' ? 0.62 : period === 'medium' ? 0.68 : 0.72;

  for (let i = days - 1; i >= 0; i--) {
    const date = new Date(Date.now() - i * 86400000);
    const noise = (Math.random() - 0.5) * 0.15;
    const accuracy = Math.min(1, Math.max(0.3, baseAccuracy + noise));
    data.push({
      date: date.toISOString().slice(0, 10),
      accuracy: Math.round(accuracy * 10000) / 10000,
      totalPredictions: 40 + Math.floor(Math.random() * 20),
      correctPredictions: Math.floor(accuracy * (40 + Math.floor(Math.random() * 20))),
    });
  }

  const labels: Record<PredictionPeriod, string> = {
    short: '短期',
    medium: '中短期',
    long: '长期',
  };

  return { period, periodLabel: labels[period], data };
}

function getMockStockAccuracy(): StockAccuracy[] {
  const stocks = [
    { symbol: '600519.SH', name: '贵州茅台' },
    { symbol: '000858.SZ', name: '五 粮 液' },
    { symbol: '300750.SZ', name: '宁德时代' },
    { symbol: '601318.SH', name: '中国平安' },
    { symbol: '000333.SZ', name: '美的集团' },
    { symbol: '600036.SH', name: '招商银行' },
    { symbol: '601012.SH', name: '隆基绿能' },
    { symbol: '002594.SZ', name: '比 亚 迪' },
    { symbol: '300059.SZ', name: '东方财富' },
    { symbol: '600276.SH', name: '恒瑞医药' },
  ];

  return stocks.map((s) => {
    const total = 50 + Math.floor(Math.random() * 100);
    const accuracy = 0.45 + Math.random() * 0.4;
    return {
      symbol: s.symbol,
      name: s.name,
      accuracy: Math.round(accuracy * 10000) / 10000,
      totalPredictions: total,
      correctPredictions: Math.floor(accuracy * total),
    };
  }).sort((a, b) => b.accuracy - a.accuracy);
}

function getMockFailures(): FailureCase[] {
  const cases: FailureCase[] = [
    {
      id: 'fail-1',
      symbol: '600519.SH',
      name: '贵州茅台',
      predictedDirection: 'up',
      actualDirection: 'down',
      predictedPrice: 1850,
      actualPrice: 1720,
      date: '2026-07-22',
      reasons: ['行业异动', '北向资金大幅流出'],
    },
    {
      id: 'fail-2',
      symbol: '300750.SZ',
      name: '宁德时代',
      predictedDirection: 'up',
      actualDirection: 'down',
      predictedPrice: 260,
      actualPrice: 238,
      date: '2026-07-21',
      reasons: ['行业异动', '竞争格局变化'],
    },
    {
      id: 'fail-3',
      symbol: '601012.SH',
      name: '隆基绿能',
      predictedDirection: 'flat',
      actualDirection: 'down',
      predictedPrice: 42,
      actualPrice: 36.5,
      date: '2026-07-20',
      reasons: ['财报发布', '业绩不及预期'],
    },
    {
      id: 'fail-4',
      symbol: '000858.SZ',
      name: '五 粮 液',
      predictedDirection: 'down',
      actualDirection: 'up',
      predictedPrice: 148,
      actualPrice: 162,
      date: '2026-07-19',
      reasons: ['突发事件', '消费刺激政策出台'],
    },
    {
      id: 'fail-5',
      symbol: '002594.SZ',
      name: '比 亚 迪',
      predictedDirection: 'up',
      actualDirection: 'flat',
      predictedPrice: 320,
      actualPrice: 305,
      date: '2026-07-18',
      reasons: ['财报发布', '销量数据低于预期'],
    },
    {
      id: 'fail-6',
      symbol: '600276.SH',
      name: '恒瑞医药',
      predictedDirection: 'up',
      actualDirection: 'down',
      predictedPrice: 48,
      actualPrice: 43.2,
      date: '2026-07-17',
      reasons: ['行业异动', '集采政策影响'],
    },
    {
      id: 'fail-7',
      symbol: '601318.SH',
      name: '中国平安',
      predictedDirection: 'flat',
      actualDirection: 'up',
      predictedPrice: 52,
      actualPrice: 56.8,
      date: '2026-07-16',
      reasons: ['突发事件', '保险资金入市利好'],
    },
    {
      id: 'fail-8',
      symbol: '300059.SZ',
      name: '东方财富',
      predictedDirection: 'down',
      actualDirection: 'up',
      predictedPrice: 18,
      actualPrice: 20.5,
      date: '2026-07-15',
      reasons: ['行业异动', '券商板块集体拉升'],
    },
  ];
  return cases;
}