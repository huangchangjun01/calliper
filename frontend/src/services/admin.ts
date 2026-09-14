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
  last_train_time: string | null;
  status: 'ready' | 'error';
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

export type TrainingPeriod = 'short_term' | 'medium_term' | 'long_term' | 'all';

export interface TrainingLog {
  id: string;
  period: 'short_term' | 'medium_term' | 'long_term';
  version: string;
  accuracy: number;
  sample_count: number;
  trigger_type: 'manual' | 'daily' | 'weekly';
  status: 'running' | 'success' | 'failed';
  started_at: string | null;
  finished_at?: string | null;
  duration_sec?: number | null;
  error_message?: string | null;
  created_at: string | null;
}

export interface TrainingJob {
  id: string;
  name: string;
  next_run: string | null;
  trigger: string;
}

// ========== 数据源配置 ==========

export function useDataSources() {
  return useQuery<DataSource[]>({
    queryKey: ['admin', 'datasources'],
    queryFn: () =>
      api
        .get<{ sources: DataSource[] }>('/admin/datasources')
        .then((res) => res?.sources ?? []),
    staleTime: 30_000,
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
    queryFn: () =>
      api
        .get<{ services: ServiceHealth[] }>('/admin/health')
        .then((res) => res?.services ?? []),
    staleTime: 15_000,
    refetchInterval: 15_000,
  });
}

export function useErrorLogs() {
  return useQuery<ErrorLog[]>({
    queryKey: ['admin', 'errors'],
    queryFn: () =>
      api
        .get<{ logs: ErrorLog[] }>('/admin/errors')
        .then((res) => res?.logs ?? []),
    staleTime: 30_000,
    refetchInterval: 30_000,
  });
}

export function useDataLatency() {
  return useQuery<DataLatency>({
    queryKey: ['admin', 'latency'],
    queryFn: () => api.get('/admin/latency'),
    staleTime: 10_000,
    refetchInterval: 10_000,
  });
}

// ========== 用户管理 ==========

export function useAdminUsers() {
  return useQuery<AdminUser[]>({
    queryKey: ['admin', 'users'],
    queryFn: () =>
      api
        .get<{ users: AdminUser[] }>('/admin/users')
        .then((res) => res?.users ?? []),
    staleTime: 60_000,
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
  return useQuery<{ models: ModelInfo[]; degraded: boolean }>({
    queryKey: ['admin', 'models'],
    queryFn: async () => {
      const res = await api.get<{ status?: string; models: ModelInfo[] }>(
        '/admin/models'
      );
      return {
        models: res?.models ?? [],
        degraded: res?.status === 'degraded',
      };
    },
    staleTime: 30_000,
    refetchInterval: 30_000,
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

// ========== 训练中心 ==========

export function useTrainingHistory(limit = 50, offset = 0) {
  return useQuery<TrainingLog[]>({
    queryKey: ['admin', 'training', 'history', limit, offset],
    queryFn: () =>
      api
        .get<{ logs: TrainingLog[] }>('/admin/training/history', {
          limit,
          offset,
        })
        .then((res) => res?.logs ?? []),
    staleTime: 30_000,
    // 存在运行中的训练时每 5 秒轮询一次，刷新状态；全部结束后自动停止轮询。
    refetchInterval: (query) => {
      const logs = query.state.data as TrainingLog[] | undefined;
      return logs?.some((l) => l.status === 'running') ? 5000 : false;
    },
  });
}

export function useTrainingSchedule() {
  return useQuery<TrainingJob[]>({
    queryKey: ['admin', 'training', 'schedule'],
    queryFn: () =>
      api
        .get<{ status: string; jobs: TrainingJob[] }>('/admin/training/schedule')
        .then((res) =>
          res?.status === 'degraded' ? [] : (res?.jobs ?? []),
        ),
    staleTime: 30_000,
  });
}

export function useRunTraining() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (period: TrainingPeriod) =>
      api.post('/admin/training/run', { period }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'models'] });
      queryClient.invalidateQueries({
        queryKey: ['admin', 'training', 'history'],
      });
    },
  });
}

export function useRollbackModel() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ period, version }: { period: string; version: string }) =>
      api.post('/admin/training/rollback', { period, version }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'models'] });
      queryClient.invalidateQueries({
        queryKey: ['admin', 'training', 'history'],
      });
    },
  });
}