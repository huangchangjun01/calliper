import { create } from 'zustand';
import type { User, LoginRequest } from '@/types';

interface AuthState {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
  setUser: (user: User) => void;
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

export const useAuthStore = create<AuthState>((set) => ({
  user: storedUser,
  token: storedToken,
  isAuthenticated: !!storedToken && !!storedUser,

  login: async (username: string, password: string) => {
    const body: LoginRequest = { username, password };
    const res = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });

    if (!res.ok) {
      const err = await res.json().catch(() => ({ message: '登录失败' }));
      throw new Error(err.message || '登录失败');
    }

    const data = await res.json();
    const { token, user } = data.data || data;

    try {
      localStorage.setItem('auth_token', token);
      localStorage.setItem('auth_user', JSON.stringify(user));
    } catch {
      // 忽略
    }

    set({ token, user, isAuthenticated: true });
  },

  logout: () => {
    try {
      localStorage.removeItem('auth_token');
      localStorage.removeItem('auth_user');
    } catch {
      // 忽略
    }
    set({ token: null, user: null, isAuthenticated: false });
  },

  setUser: (user: User) => {
    try {
      localStorage.setItem('auth_user', JSON.stringify(user));
    } catch {
      // 忽略
    }
    set({ user });
  },
}));