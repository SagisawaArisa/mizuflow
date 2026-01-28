import api from '@/lib/axios';
import type { LoginParams, AuthResponse, User } from '@/types';

export const authService = {
  login: async (params: LoginParams): Promise<AuthResponse> => {
    const { data } = await api.post<AuthResponse>('/auth/login', params);
    return data;
  },
  logout: async () => {
    await api.post('/auth/logout');
    localStorage.removeItem('access_token');
    localStorage.removeItem('refresh_token');
  },
  getProfile: async (): Promise<User> => {
    const { data } = await api.get<User>('/auth/me');
    return data;
  },
  isAuthenticated: () => {
    return !!localStorage.getItem('access_token');
  }
};
