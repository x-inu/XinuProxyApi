import { apiClient } from './client';

export interface CacheStats {
  enabled: boolean;
  memoryEntries: number;
  dbEntries: number;
  hits: number;
  misses: number;
  hitRate: string;
  tokensSaved: number;
}

export const cacheApi = {
  async stats(): Promise<CacheStats> {
    return apiClient.get<CacheStats>('/semantic-cache/stats');
  },
  async list(page = 1, limit = 20, search = '', model = ''): Promise<{ entries: any[]; total: number }> {
    const params = new URLSearchParams({ page: String(page), limit: String(limit) });
    if (search) params.set('search', search);
    if (model) params.set('model', model);
    return apiClient.get(`/semantic-cache/entries?${params}`);
  },
  async invalidateModel(model: string): Promise<{ removed: number }> {
    return apiClient.post('/semantic-cache/invalidate-model', null, { params: { model } });
  },
  async clear(): Promise<{ status: string }> {
    return apiClient.delete('/semantic-cache');
  },
  async deleteEntry(id: string): Promise<{ status: string }> {
    return apiClient.delete(`/semantic-cache/entries/${encodeURIComponent(id)}`);
  },
};
