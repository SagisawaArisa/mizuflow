import api from '@/lib/axios';
import type { FeatureFlag, FeatureAudit } from '@/types';

export const featureService = {
  getAll: async (namespace = 'default', env = 'dev') => {
    const { data } = await api.get<FeatureFlag[]>(`/features?namespace=${namespace}&env=${env}`);
    return data;
  },
  
  get: async (key: string, namespace = 'default', env = 'dev') => {
    // Backend returns the FeatureItem directly (flattened)
    const { data } = await api.get<FeatureFlag>(`/feature/${key}?namespace=${namespace}&env=${env}`);
    return data;
  },

  create: async (feature: FeatureFlag) => {
    const { data } = await api.post('/feature', feature);
    return data;
  },
  
  update: async (feature: FeatureFlag) => {
      // Create actually updates if key exists based on SaveFeature implementation
      return featureService.create(feature);
  },

  rollback: async (key: string, auditId: number, namespace = 'default', env = 'dev') => {
    const { data } = await api.post(`/feature/${key}/rollback`, { audit_id: auditId, namespace, env });
    return data;
  },
  
  getAudits: async (key: string) => {
      const { data } = await api.get<FeatureAudit[]>(`/feature/${key}/audits`);
      return data;
  }
};
