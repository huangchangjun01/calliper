import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import api from '@/services/api';

// ========== 类型定义 ==========

export interface DataSource {
  id: string;
  name: string;
  type: 'rest' | 'websocket' | 'grpc';
  status: 'running' | 'stopped' | 'error';
  apiKey?: string;
  collectFrequency: string;
  enabled: boolean;
  lastSyncTime: string | null;
  healthy: boolean;
}

export interface DataSourceConfig {
  apiKey: string;
  collectFrequency: string;
  enabled: boolean;
}

export interface ServiceHealth {
  name: string;
  service: 'gateway' | 'market' | 'prediction' | 'engine';
  status: 'running' | 'stopped';
  latency: number;
  lastHeartbeat: string;
}

export interface ErrorLog {
  id: string;
  timestamp: string;
  service: string;
  message: string;
}

export interface DataLatency {
  kafkaLag: number;
  redisHitRate: number;
  updateTime: string;
}

export interface AdminUser {
  id: string;
  username: string;
  email: string;
  role: 'admin' | 'user' | 'viewer';
  status: 'active' | 'disabled';
  createdAt: string;
}

export interface CreateUserRequest {
  username: string;
  email: string;
  password: string;
  role: string;
}

export interface UpdateUserRequest {
  role?: string;
  status?: 'active' | 'disabled';
}

export interface ModelInfo {
  id: string;
  name: string;
  period: 'short' | 'medium' | 'long';
  version: string;
  accuracy: number;
  lastTrainTime: string | null;
  status: 'idle' | 'training' | 'evaluating' | 'ready' | 'error';
  params: ShortModelParams | MediumModelParams | LongModelParams;
}

export interface ShortModelParams {
  hidden_size: number;
  num_layers: number;
  dropout: number;
}

export interface MediumModelParams {
  xgb_max_depth: number;
  xgb_learning_rate: number;
  lgb_num_leaves: number;
}

export interface LongModelParams {
  d_model: number;
  nhead: number;
  num_layers: number;
}

// ========== 数据源配置 ==========

export function useDataSources() {
  return useQuery<DataSource[]>({
    queryKey: ['admin', 'datasources'],
    queryFn: () => api.get('/admin/datasources'),
    staleTime: 30_000,
    placeholderData: getMockDataSources(),
  });
}

export function useUpdateDataSource() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, config }: { id: string; config: DataSourceConfig }) =>
      api.put(`/admin/datasources/${id}`, config),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'datasources'] });
    },
  });
}

export function useTriggerSync() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post(`/admin/datasources/${id}/sync`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'datasources'] });
    },
  });
}

export function useToggleDataSource() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      api.patch(`/admin/datasources/${id}`, { enabled }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'datasources'] });
    },
  });
}

// ========== 系统监控 ==========

export function useServiceHealth() {
  return useQuery<ServiceHealth[]>({
    queryKey: ['admin', 'health'],
    queryFn: () => api.get('/admin/health'),
    staleTime: 15_000,
    refetchInterval: 15_000,
    placeholderData: getMockServiceHealth(),
  });
}

export function useErrorLogs() {
  return useQuery<ErrorLog[]>({
    queryKey: ['admin', 'errors'],
    queryFn: () => api.get('/admin/errors'),
    staleTime: 30_000,
    refetchInterval: 30_000,
    placeholderData: getMockErrorLogs(),
  });
}

export function useDataLatency() {
  return useQuery<DataLatency>({
    queryKey: ['admin', 'latency'],
    queryFn: () => api.get('/admin/latency'),
    staleTime: 10_000,
    refetchInterval: 10_000,
    placeholderData: getMockDataLatency(),
  });
}

// ========== 用户管理 ==========

export function useAdminUsers() {
  return useQuery<AdminUser[]>({
    queryKey: ['admin', 'users'],
    queryFn: () => api.get('/admin/users'),
    staleTime: 60_000,
    placeholderData: getMockAdminUsers(),
  });
}

export function useCreateUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (user: CreateUserRequest) => api.post('/admin/users', user),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'users'] });
    },
  });
}

export function useUpdateUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateUserRequest }) =>
      api.put(`/admin/users/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'users'] });
    },
  });
}

export function useDeleteUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/admin/users/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'users'] });
    },
  });
}

// ========== 模型管理 ==========

export function useModels() {
  return useQuery<ModelInfo[]>({
    queryKey: ['admin', 'models'],
    queryFn: () => api.get('/admin/models'),
    staleTime: 30_000,
    placeholderData: getMockModels(),
  });
}

export function useTrainModel() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post(`/admin/models/${id}/train`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'models'] });
    },
  });
}

export function useEvaluateModel() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post(`/admin/models/${id}/evaluate`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'models'] });
    },
  });
}

export function useUpdateModelParams() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, params }: { id: string; params: Record<string, number> }) =>
      api.put(`/admin/models/${id}/params`, params),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'models'] });
    },
  });
}

export function useTriggerPrediction() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post(`/admin/models/${id}/predict`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'models'] });
    },
  });
}

// ========== Mock 数据 ==========

function getMockDataSources(): DataSource[] {
  return [
    {
      id: 'ds-1',
      name: '东方财富行情',
      type: 'rest',
      status: 'running',
      apiKey: 'ak-****1234',
      collectFrequency: '1m',
      enabled: true,
      lastSyncTime: new Date(Date.now() - 30_000).toISOString(),
      healthy: true,
    },
    {
      id: 'ds-2',
      name: 'Wind 实时 WS',
      type: 'websocket',
      status: 'running',
      apiKey: 'ak-****5678',
      collectFrequency: '5m',
      enabled: true,
      lastSyncTime: new Date(Date.now() - 120_000).toISOString(),
      healthy: true,
    },
    {
      id: 'ds-3',
      name: 'Tushare 数据',
      type: 'rest',
      status: 'stopped',
      apiKey: '',
      collectFrequency: '15m',
      enabled: false,
      lastSyncTime: null,
      healthy: false,
    },
    {
      id: 'ds-4',
      name: '聚宽数据源',
      type: 'grpc',
      status: 'error',
      apiKey: 'ak-****9012',
      collectFrequency: '30m',
      enabled: true,
      lastSyncTime: new Date(Date.now() - 600_000).toISOString(),
      healthy: false,
    },
  ];
}

function getMockServiceHealth(): ServiceHealth[] {
  return [
    {
      name: 'Gateway 网关',
      service: 'gateway',
      status: 'running',
      latency: 12,
      lastHeartbeat: new Date().toISOString(),
    },
    {
      name: '行情服务',
      service: 'market',
      status: 'running',
      latency: 45,
      lastHeartbeat: new Date().toISOString(),
    },
    {
      name: '预测服务',
      service: 'prediction',
      status: 'running',
      latency: 230,
      lastHeartbeat: new Date().toISOString(),
    },
    {
      name: '交易引擎',
      service: 'engine',
      status: 'stopped',
      latency: 0,
      lastHeartbeat: new Date(Date.now() - 300_000).toISOString(),
    },
  ];
}

function getMockErrorLogs(): ErrorLog[] {
  return [
    {
      id: 'err-1',
      timestamp: new Date(Date.now() - 60_000).toISOString(),
      service: '预测服务',
      message: '模型加载超时: short_lstm_v2.3',
    },
    {
      id: 'err-2',
      timestamp: new Date(Date.now() - 180_000).toISOString(),
      service: '行情服务',
      message: 'WebSocket 连接断开, 正在重连...',
    },
    {
      id: 'err-3',
      timestamp: new Date(Date.now() - 300_000).toISOString(),
      service: '交易引擎',
      message: '订单执行失败: 余额不足',
    },
    {
      id: 'err-4',
      timestamp: new Date(Date.now() - 600_000).toISOString(),
      service: '行情服务',
      message: 'K线数据同步延迟超过阈值: 600519.SH',
    },
    {
      id: 'err-5',
      timestamp: new Date(Date.now() - 900_000).toISOString(),
      service: 'Gateway 网关',
      message: '上游 API 限流, 请求被拒绝',
    },
  ];
}

function getMockDataLatency(): DataLatency {
  return {
    kafkaLag: 125,
    redisHitRate: 0.943,
    updateTime: new Date().toISOString(),
  };
}

function getMockAdminUsers(): AdminUser[] {
  return [
    {
      id: 'u-1',
      username: 'admin',
      email: 'admin@quant.com',
      role: 'admin',
      status: 'active',
      createdAt: '2026-01-15T08:00:00Z',
    },
    {
      id: 'u-2',
      username: 'zhangsan',
      email: 'zhangsan@quant.com',
      role: 'user',
      status: 'active',
      createdAt: '2026-02-20T10:30:00Z',
    },
    {
      id: 'u-3',
      username: 'lisi',
      email: 'lisi@quant.com',
      role: 'user',
      status: 'active',
      createdAt: '2026-03-10T14:00:00Z',
    },
    {
      id: 'u-4',
      username: 'wangwu',
      email: 'wangwu@quant.com',
      role: 'viewer',
      status: 'active',
      createdAt: '2026-04-05T09:15:00Z',
    },
    {
      id: 'u-5',
      username: 'zhaoliu',
      email: 'zhaoliu@quant.com',
      role: 'user',
      status: 'disabled',
      createdAt: '2026-05-18T16:45:00Z',
    },
  ];
}

function getMockModels(): ModelInfo[] {
  return [
    {
      id: 'm-1',
      name: 'LSTM 短期预测',
      period: 'short',
      version: 'v2.3.0',
      accuracy: 0.683,
      lastTrainTime: new Date(Date.now() - 86_400_000).toISOString(),
      status: 'ready',
      params: { hidden_size: 128, num_layers: 2, dropout: 0.2 },
    },
    {
      id: 'm-2',
      name: 'XGBoost 中短期',
      period: 'medium',
      version: 'v1.8.0',
      accuracy: 0.712,
      lastTrainTime: new Date(Date.now() - 172_800_000).toISOString(),
      status: 'ready',
      params: { xgb_max_depth: 6, xgb_learning_rate: 0.05, lgb_num_leaves: 31 },
    },
    {
      id: 'm-3',
      name: 'LightGBM 中短期',
      period: 'medium',
      version: 'v1.5.0',
      accuracy: 0.695,
      lastTrainTime: new Date(Date.now() - 172_800_000).toISOString(),
      status: 'idle',
      params: { xgb_max_depth: 5, xgb_learning_rate: 0.08, lgb_num_leaves: 63 },
    },
    {
      id: 'm-4',
      name: 'Transformer 长期',
      period: 'long',
      version: 'v1.2.0',
      accuracy: 0.738,
      lastTrainTime: new Date(Date.now() - 259_200_000).toISOString(),
      status: 'ready',
      params: { d_model: 256, nhead: 8, num_layers: 4 },
    },
  ];
}