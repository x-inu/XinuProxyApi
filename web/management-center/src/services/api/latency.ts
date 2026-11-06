import { apiClient } from './client';

export interface LatencyBucket {
  provider: string;
  model: string;
  total: number;
  failed: number;
  errorRate: string;
  p50Ms: number;
  p95Ms: number;
  p99Ms: number;
  avgMs: number;
  samples: number;
}

export interface LatencyOverall {
  total: number;
  failed: number;
  errorRate: string;
  p50Ms: number;
  p95Ms: number;
  p99Ms: number;
  avgMs: number;
  samples: number;
}

export interface LatencyStats {
  enabled: boolean;
  overall: LatencyOverall;
  byModel: LatencyBucket[];
}

export const latencyApi = {
  async get(): Promise<LatencyStats> {
    return apiClient.get<LatencyStats>('/latency-stats');
  },
};
