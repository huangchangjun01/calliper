import api from '@/services/api';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';

export interface AccountProfile {
  id: string;
  username: string;
  email: string;
  role: 'admin' | 'user' | 'viewer';
  status: 'active' | 'disabled';
  createdAt: string;
}

export function getMyProfile() {
  return api.get<AccountProfile>('/account/me');
}

export function updateMyProfile(data: { email?: string }) {
  return api.put<AccountProfile>('/account/me', data);
}

export function changeMyPassword(data: { old_password: string; new_password: string }) {
  return api.put<{ message: string }>('/account/me/password', data);
}

export function register(data: { username: string; email: string; password: string }) {
  return api.post<{ user_id: string; message: string }>('/auth/register', data);
}

// ========== React Query hooks ==========

export function useMyProfile() {
  return useQuery<AccountProfile>({
    queryKey: ['account', 'me'],
    queryFn: getMyProfile,
    staleTime: 30_000,
  });
}

export function useUpdateMyProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: { email?: string }) => updateMyProfile(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['account', 'me'] });
    },
  });
}

export function useChangeMyPassword() {
  return useMutation({
    mutationFn: (data: { old_password: string; new_password: string }) => changeMyPassword(data),
  });
}
