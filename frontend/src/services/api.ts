import type { ApiResponse } from '@/types';

const BASE_URL = '/api/v1';

let authToken: string | null = null;

export function setAuthToken(token: string | null) {
  authToken = token;
}

// 初始化时从 localStorage 恢复 token
try {
  const stored = localStorage.getItem('auth_token');
  if (stored) {
    authToken = stored;
  }
} catch {
  // 忽略
}

interface RequestOptions extends Omit<RequestInit, 'body'> {
  body?: unknown;
  params?: Record<string, string | number | boolean | undefined>;
}

class ApiError extends Error {
  code: number;

  constructor(code: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
  }
}

async function request<T = unknown>(
  endpoint: string,
  options: RequestOptions = {}
): Promise<T> {
  const { body, params, headers: customHeaders, ...rest } = options;

  // 处理查询参数
  let url = `${BASE_URL}${endpoint}`;
  if (params) {
    const searchParams = new URLSearchParams();
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined) {
        searchParams.set(key, String(value));
      }
    }
    const qs = searchParams.toString();
    if (qs) {
      url += `?${qs}`;
    }
  }

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(customHeaders as Record<string, string>),
  };

  // 自动附加 Authorization header
  if (authToken) {
    headers['Authorization'] = `Bearer ${authToken}`;
  }

  const fetchOptions: RequestInit = {
    ...rest,
    headers,
  };

  if (body !== undefined) {
    fetchOptions.body = JSON.stringify(body);
  }

  const res = await fetch(url, fetchOptions);

  if (!res.ok) {
    if (res.status === 401) {
      // 令牌过期，清除本地存储
      try {
        localStorage.removeItem('auth_token');
        localStorage.removeItem('auth_user');
      } catch {
        // 忽略
      }
      authToken = null;
    }

    const errorData = await res.json().catch(() => ({
      code: res.status,
      message: '请求失败',
    }));
    throw new ApiError(errorData.code || res.status, errorData.message || '请求失败');
  }

  const json: ApiResponse<T> = await res.json();
  return json.data;
}

// 便捷方法
export const api = {
  get<T = unknown>(endpoint: string, params?: RequestOptions['params']) {
    return request<T>(endpoint, { method: 'GET', params });
  },

  post<T = unknown>(endpoint: string, body?: unknown) {
    return request<T>(endpoint, { method: 'POST', body });
  },

  put<T = unknown>(endpoint: string, body?: unknown) {
    return request<T>(endpoint, { method: 'PUT', body });
  },

  patch<T = unknown>(endpoint: string, body?: unknown) {
    return request<T>(endpoint, { method: 'PATCH', body });
  },

  delete<T = unknown>(endpoint: string, params?: RequestOptions['params']) {
    return request<T>(endpoint, { method: 'DELETE', params });
  },
};

export { ApiError };
export default api;