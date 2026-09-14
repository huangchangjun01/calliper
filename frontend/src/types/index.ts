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

/** 预测状态 */
export type PredictionStatus = 'pending' | 'correct' | 'wrong';

/** 预测详情记录（对应 GET /api/v1/predictions/details） */
export interface PredictionDetail {
  id: string;
  symbol: string;
  name: string;
  period: PredictionPeriod;
  direction: PredictionDirection;
  confidence: number;
  target_price: number | null;
  actual_price: number | null;
  predicted_at: string;
  valid_until: string;
  expired: boolean;
  status: PredictionStatus;
  is_correct: boolean | null;
  model_version: string;
  key_factors: string[];
}

/** 预测列表筛选条件 */
export interface PredictionFilters {
  period?: PredictionPeriod;
  direction?: PredictionDirection;
  confidenceMin?: number;
  expired?: boolean;
  status?: PredictionStatus;
}

/** 预测历史记录（GET /api/v1/predictions/history） */
export interface PredictionHistoryItem {
  id: number;
  symbol: string;
  name: string;
  period: string;
  direction: PredictionDirection;
  confidence: number;
  target_price: number;
  actual_price: number | null;
  predicted_at: string;
  valid_until: string;
  status: PredictionStatus;
}

/** 分时段预测统计（GET /api/v1/predictions/stats），accuracy 为 null 表示该时段无已评估预测（断点） */
export interface PredictionStat {
  bucket: string;
  total: number;
  correct: number;
  wrong: number;
  pending: number;
  accuracy: number | null;
}

/** 预测历史分页响应（GET /api/v1/predictions/history） */
export type HistoryResponse = PaginatedItems<PredictionHistoryItem>;

/** 生成预测响应（POST /api/v1/predictions/generate） */
export interface GenerateResponse {
  status: string;
  results: {
    symbol: string;
    status: 'predicted' | 'no_data';
    count: number;
  }[];
  persisted: number;
}

/** 准确率趋势点（GET /api/v1/predictions/accuracy），accuracy 为 null 表示无数据日（断点） */
export interface AccuracyTrendPoint {
  date: string;
  accuracy: number | null;
}

/** 准确率趋势 */
export type AccuracyTrend = AccuracyTrendPoint[];

/** 各股票准确率（GET /api/v1/predictions/stock-accuracy） */
export interface StockAccuracy {
  symbol: string;
  accuracy: number;
  total_predictions: number;
}

/** 预测失败案例（GET /api/v1/predictions/failures） */
export interface FailureCase {
  id: string;
  symbol: string;
  name: string;
  predicted_direction: PredictionDirection;
  actual_direction: PredictionDirection;
  period: PredictionPeriod;
  predicted_at: string;
  summary: string;
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

/** 分页数据包装（offset/limit/total 形式，后端通用） */
export interface PaginatedItems<T> {
  items: T[];
  total: number;
  limit: number;
  offset: number;
}

// ========== Dashboard 决策支持相关类型 ==========

/** 数据源降级（不可用）标识 */
export interface UnavailableSection {
  status: 'unavailable';
}

/** 高置信度标的 */
export interface HighConfidenceItem {
  symbol: string;
  name: string;
  period: PredictionPeriod;
  direction: PredictionDirection;
  confidence: number;
  target_price: number | null;
}

/** 预测准确率摘要 */
export interface AccuracySummary {
  accuracy_7d: number | null;
  accuracy_30d: number | null;
  accuracy_total: number | null;
  total_evaluated: number | null;
}

/** 风险提示 */
export interface RiskAlert {
  symbol?: string;
  name?: string;
  level?: string;
  type?: string;
  message?: string;
  content?: string;
}

/** 模型健康状态 */
export interface ModelHealth {
  suspend: boolean;
  reason: string | null;
  consecutive_below_threshold: number;
}

/** 系统状态 */
export interface SystemStatus {
  last_sync: string | null;
  model_health: ModelHealth;
  thresholds: {
    suspend_threshold: number;
    retrain_threshold: number;
    high_confidence_threshold: number;
  };
  degraded: boolean;
}

/** Dashboard 数据（各区块可能降级为 unavailable） */
export interface DashboardData {
  high_confidence: HighConfidenceItem[] | UnavailableSection;
  accuracy_summary: AccuracySummary | UnavailableSection;
  risk_alerts: RiskAlert[] | UnavailableSection;
  system_status: SystemStatus | UnavailableSection;
}

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