import { useCallback, useEffect, useState } from 'react';
import { usageHistoryApi, type UsageHistory } from '@/services/api';
import { useAuthStore } from '@/stores';

const EMPTY_USAGE: UsageHistory = {
  enabled: false,
  totals: {
    requests: 0,
    failed: 0,
    inputTokens: 0,
    outputTokens: 0,
    cachedTokens: 0,
    totalTokens: 0,
    costUsd: 0,
  },
  byModel: [],
  timeline: [],
};

export function useUsageOverview() {
  const connectionStatus = useAuthStore((state) => state.connectionStatus);
  const connected = connectionStatus === 'connected';

  const [usage, setUsage] = useState<UsageHistory>(EMPTY_USAGE);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);

  const load = useCallback(async () => {
    if (!connected) return;
    setLoading(true);
    setError(false);
    try {
      const result = await usageHistoryApi.get();
      setUsage(result);
    } catch {
      setUsage(EMPTY_USAGE);
      setError(true);
    } finally {
      setLoading(false);
    }
  }, [connected]);

  const clear = useCallback(async () => {
    if (!connected) return;
    await usageHistoryApi.clear();
    await load();
  }, [connected, load]);

  useEffect(() => {
    if (!connected) {
      setUsage(EMPTY_USAGE);
      return;
    }
    void load();
  }, [connected, load]);

  const refresh = useCallback(async () => {
    await load();
  }, [load]);

  return {
    connectionStatus,
    connected,
    usage,
    loading,
    error,
    refresh,
    clear,
  };
}
