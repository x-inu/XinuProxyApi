import { apiClient } from './client';

const USAGE_HISTORY_TIMEOUT_MS = 15 * 1000;

export interface UsageHistoryModelRow {
  provider: string;
  model: string;
  requests: number;
  failed: number;
  inputTokens: number;
  outputTokens: number;
  cachedTokens: number;
  totalTokens: number;
  costUsd: number;
}

export interface UsageHistoryTimePoint {
  hour: number;
  requests: number;
  failed: number;
  totalTokens: number;
  inputTokens: number;
  outputTokens: number;
  costUsd: number;
}

export interface UsageHistoryTotals {
  requests: number;
  failed: number;
  inputTokens: number;
  outputTokens: number;
  cachedTokens: number;
  totalTokens: number;
  costUsd: number;
}

export interface UsageHistory {
  enabled: boolean;
  totals: UsageHistoryTotals;
  byModel: UsageHistoryModelRow[];
  timeline: UsageHistoryTimePoint[];
}

interface RawUsageTotals {
  requests?: number;
  failed?: number;
  input_tokens?: number;
  output_tokens?: number;
  cached_tokens?: number;
  total_tokens?: number;
  cost_usd?: number;
}

interface RawUsageModelRow extends RawUsageTotals {
  provider?: string;
  model?: string;
}

interface RawUsageTimePoint {
  hour?: number;
  requests?: number;
  failed?: number;
  total_tokens?: number;
  input_tokens?: number;
  output_tokens?: number;
  cost_usd?: number;
}

interface RawUsageHistory {
  enabled?: boolean;
  totals?: RawUsageTotals;
  by_model?: RawUsageModelRow[] | null;
  timeline?: RawUsageTimePoint[] | null;
}

const num = (value: number | undefined): number =>
  typeof value === 'number' && Number.isFinite(value) ? value : 0;

function normalizeTotals(raw: RawUsageTotals | undefined): UsageHistoryTotals {
  return {
    requests: num(raw?.requests),
    failed: num(raw?.failed),
    inputTokens: num(raw?.input_tokens),
    outputTokens: num(raw?.output_tokens),
    cachedTokens: num(raw?.cached_tokens),
    totalTokens: num(raw?.total_tokens),
    costUsd: num(raw?.cost_usd),
  };
}

function normalizeUsageHistory(raw: RawUsageHistory): UsageHistory {
  const byModel = (raw.by_model ?? []).map((row) => ({
    provider: row.provider ?? 'unknown',
    model: row.model ?? 'unknown',
    requests: num(row.requests),
    failed: num(row.failed),
    inputTokens: num(row.input_tokens),
    outputTokens: num(row.output_tokens),
    cachedTokens: num(row.cached_tokens),
    totalTokens: num(row.total_tokens),
    costUsd: num(row.cost_usd),
  }));

  const timeline = (raw.timeline ?? []).map((point) => ({
    hour: num(point.hour),
    requests: num(point.requests),
    failed: num(point.failed),
    totalTokens: num(point.total_tokens),
    inputTokens: num(point.input_tokens),
    outputTokens: num(point.output_tokens),
    costUsd: num(point.cost_usd),
  }));

  return {
    enabled: raw.enabled === true,
    totals: normalizeTotals(raw.totals),
    byModel,
    timeline,
  };
}

export const usageHistoryApi = {
  async get(): Promise<UsageHistory> {
    const raw = await apiClient.get<RawUsageHistory>('/usage-history', {
      timeout: USAGE_HISTORY_TIMEOUT_MS,
    });
    return normalizeUsageHistory(raw ?? {});
  },
  clear: () => apiClient.delete('/usage-history'),
};
