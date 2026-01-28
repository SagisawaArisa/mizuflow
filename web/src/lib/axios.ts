import axios from 'axios';

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || '/v1',
  headers: {
    'Content-Type': 'application/json',
  },
});

// Request interceptor to add Access Token to headers
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('access_token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// Response interceptor to handle Token Refresh
api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    // Prevent infinite loop
    if (error.response?.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true;

      try {
        const refreshToken = localStorage.getItem('refresh_token');
        if (!refreshToken) {
            throw new Error('No refresh token');
        }

        const { data } = await axios.post('/v1/auth/refresh', { refresh_token: refreshToken });
        
        localStorage.setItem('access_token', data.access_token);
        if (data.refresh_token) {
             localStorage.setItem('refresh_token', data.refresh_token);
        }

        api.defaults.headers.common['Authorization'] = `Bearer ${data.access_token}`;
        return api(originalRequest);
      } catch (refreshError) {
        // Logout if refresh fails
        localStorage.removeItem('access_token');
        localStorage.removeItem('refresh_token');
        window.location.href = '/login';
        return Promise.reject(refreshError);
      }
    }
    return Promise.reject(error);
  }
);

export default api;
