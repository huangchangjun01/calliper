import { create } from 'zustand';
import type { User, LoginRequest } from '@/types';
import { setAuthToken } from '@/services/api';

interface AuthState {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
  setUser: (user: User) => void;
  checkAuth: () => boolean;
}

function getStoredToken(): string | null {
  try {
    return localStorage.getItem('auth_token');
  } catch {
    return null;
  }
}

function getStoredUser(): User | null {
  try {
    const raw = localStorage.getItem('auth_user');
    if (raw) {
      return JSON.parse(raw) as User;
    }
  } catch {
    // 忽略
  }
  return null;
}

const storedToken = getStoredToken();
const storedUser = storedToken ? getStoredUser() : null;

export const useAuthStore = create<AuthState>((set, get) => ({
  user: storedUser,
  token: storedToken,
  isAuthenticated: !!(storedToken && storedUser),
  isLoading: false,

  login: async (username: string, password: string) => {
    set({ isLoading: true });
    try {
      const body: LoginRequest = { username, password };
      const res = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });

      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err.error || err.message || '登录失败');
      }

      // 后端返回: { token, user_id, role }
      const data = await res.json();
      const token = data.token;
      const user: User = {
        id: data.user_id || '',
        username: username,
        email: data.email || '',
        role: data.role || 'user',
      };

      try {
        localStorage.setItem('auth_token', token);
        localStorage.setItem('auth_user', JSON.stringify(user));
      } catch {
        // 忽略
      }

      setAuthToken(token);
      set({ token, user, isAuthenticated: true, isLoading: false });
    } catch (err) {
      set({ isLoading: false });
      throw err;
    }
  },

  logout: () => {
    try {
      localStorage.removeItem('auth_token');
      localStorage.removeItem('auth_user');
    } catch {
      // 忽略
    }
    setAuthToken(null);
    set({ token: null, user: null, isAuthenticated: false, isLoading: false });
  },

  setUser: (user: User) => {
    try {
      localStorage.setItem('auth_user', JSON.stringify(user));
    } catch {
      // 忽略
    }
    set({ user });
  },

  checkAuth: () => {
    const token = get().token || getStoredToken();
    const user = get().user || getStoredUser();
    if (token && user) {
      setAuthToken(token);
      set({ token, user, isAuthenticated: true });
      return true;
    }
    return false;
  },
}));