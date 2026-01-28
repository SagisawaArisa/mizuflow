export interface User {
  id: string;
  username: string;
  role: string;
  avatar?: string;
}

export interface AuthResponse {
  access_token: string;
  refresh_token: string;
  expires_in: number;
}

export interface FeatureFlag {
  id: number;
  key: string;
  type: 'bool' | 'string' | 'json' | 'strategy' | 'number';
  value: string;
  version: number;
  namespace: string;
  env: string;
  updated_at: string;
  updated_by: string;
  // Optional/Derived fields for UI
  name?: string;
  description?: string;
  enabled?: boolean; 
}

export interface FeatureAudit {
  id: number;
  key: string;
  namespace: string;
  env: string;
  old_value: string;
  new_value: string;
  type: string;
  operator: string;
  created_at: string;
}

export interface LoginParams {
  username: string;
  password?: string; // Mock
}
