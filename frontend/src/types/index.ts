// ========== 股票相关类型 ==========

/** 实时行情数据 */
export interface StockQuote {
  symbol: string;
  name: string;
  price: number;
  change: number;
  changePercent: number;
  open: number;
  high: number;
  low: number;
  preClose: number;
  volume: number;
  amount: number;
  timestamp: number;
}

/** 股票基本信息 */
export interface Stock {
  symbol: string;
  name: string;
  exchange: string;
  industry: string;
  marketCap: number;
  listingDate: string;
  description: string;
}

/** 股票搜索结果项 */
export interface StockSearchItem {
  symbol: string;
  name: string;
  exchange: string;
  industry: string;
  marketCap: number;
  price: number;
  changePercent: number;
}

/** K线数据 */
export interface MarketData {
  symbol: string;
  period: '1m' | '5m' | '15m' | '30m' | '60m' | '1d' | '1w' | '1M';
  data: KlineItem[];
}

export interface KlineItem {
  timestamp: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
  amount: number;
}

// ========== 交易相关类型 ==========

/** 订单 */
export interface Order {
  id: string;
  symbol: string;
  type: 'market' | 'limit';
  side: 'buy' | 'sell';
  price: number;
  quantity: number;
  filledQuantity: number;
  status: OrderStatus;
  createdAt: string;
  updatedAt: string;
}

export type OrderStatus = 'pending' | 'partial' | 'filled' | 'cancelled' | 'rejected';

/** 持仓 */
export interface Position {
  symbol: string;
  name: string;
  quantity: number;
  avgCost: number;
  currentPrice: number;
  marketValue: number;
  profit: number;
  profitPercent: number;
}

/** 下单请求 */
export interface OrderRequest {
  symbol: string;
  side: 'buy' | 'sell';
  type: 'market' | 'limit';
  price?: number;
  quantity: number;
  password?: string;
}

/** 账户信息 */
export interface AccountInfo {
  totalAsset: number;
  availableCash: number;
  marketValue: number;
  todayProfit: number;
  todayProfitPercent: number;
  totalProfit: number;
  totalProfitPercent: number;
  riskLevel: string;
}

/** 模拟交易状态 */
export interface SimStatus {
  running: boolean;
  account: AccountInfo;
  decisions: SimDecision[];
  records: SimRecord[];
  riskControl: RiskControl;
}

/** 模拟交易决策 */
export interface SimDecision {
  id: string;
  symbol: string;
  name: string;
  side: 'buy' | 'sell';
  price: number;
  quantity: number;
  confidence: number;
  reason: string;
  createdAt: string;
}

/** 模拟交易记录 */
export interface SimRecord {
  id: string;
  symbol: string;
  name: string;
  side: 'buy' | 'sell';
  price: number;
  quantity: number;
  profit: number;
  profitPercent: number;
  createdAt: string;
}

/** 风险控制 */
export interface RiskControl {
  maxDailyLoss: number;
  currentDailyLoss: number;
  maxPositionRatio: number;
  currentPositionRatio: number;
  maxSingleStockRatio: number;
  status: 'normal' | 'warning' | 'danger';
}

// ========== 预测相关类型 ==========

/** 预测方向 */
export type PredictionDirection = 'up' | 'down' | 'flat';

/** 预测周期 */
export type PredictionPeriod = 'short' | 'medium' | 'long';

/** 预测结果 */
export interface Prediction {
  id: string;
  symbol: string;
  modelName: string;
  predictedPrice: number;
  confidence: number;
  direction: PredictionDirection;
  timeframe: string;
  createdAt: string;
  features: Record<string, number>;
}

/** 预测概览汇总 */
export interface PredictionSummary {
  period: PredictionPeriod;
  periodLabel: string;
  total: number;
  upCount: number;
  downCount: number;
  flatCount: number;
}

/** 预测详情（展开行） */
export interface PredictionDetail {
  id: string;
  symbol: string;
  name: string;
  short: { direction: PredictionDirection; confidence: number };
  medium: { direction: PredictionDirection; confidence: number };
  long: { direction: PredictionDirection; confidence: number };
  targetPrice: number | null;
  keyFactors: string[];
  modelVersion: string;
  updatedAt: string;
}

/** 准确率数据点 */
export interface AccuracyDataPoint {
  date: string;
  accuracy: number;
  totalPredictions: number;
  correctPredictions: number;
}

/** 准确率趋势 */
export interface AccuracyTrend {
  period: PredictionPeriod;
  periodLabel: string;
  data: AccuracyDataPoint[];
}

/** 股票准确率排行 */
export interface StockAccuracy {
  symbol: string;
  name: string;
  accuracy: number;
  totalPredictions: number;
  correctPredictions: number;
}

/** 预测失败案例 */
export interface FailureCase {
  id: string;
  symbol: string;
  name: string;
  predictedDirection: PredictionDirection;
  actualDirection: PredictionDirection;
  predictedPrice: number;
  actualPrice: number;
  date: string;
  reasons: string[];
}

// ========== 用户相关类型 ==========

export interface User {
  id: string;
  username: string;
  email: string;
  role: 'admin' | 'user' | 'viewer';
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  user: User;
}

// ========== API 响应类型 ==========

export interface ApiResponse<T = unknown> {
  code: number;
  message: string;
  data: T;
}

export interface PaginatedData<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
}

export interface PaginatedResponse<T> extends ApiResponse<PaginatedData<T>> {}

// ========== WebSocket 消息类型 ==========

export interface WsMessage<T = unknown> {
  channel: string;
  type: 'subscribe' | 'unsubscribe' | 'data' | 'quote' | 'error' | 'heartbeat';
  data: T;
  timestamp: number;
}

export interface WsSubscribePayload {
  channel: string;
  symbols?: string[];
}

// ========== 通用类型 ==========

export type ThemeMode = 'light' | 'dark';

export type SidebarMenuItem = {
  key: string;
  label: string;
  icon: string;
  path: string;
};