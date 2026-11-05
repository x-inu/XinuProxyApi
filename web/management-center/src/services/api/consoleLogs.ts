import { apiClient } from './client';
import { computeApiUrl } from '@/utils/connection';

export type ConsoleLogLevel = 'debug' | 'info' | 'warn' | 'error';

export interface ConsoleLogEntry {
  time: string;
  level: ConsoleLogLevel;
  line: string;
  request_id?: string;
  caller?: string;
  message?: string;
}

export interface ConsoleLogsResponse {
  success: boolean;
  logs: ConsoleLogEntry[];
}

export interface ConsoleLogLevelResponse {
  success: boolean;
  level: string;
}

export const consoleLogsApi = {
  fetchLogs: (): Promise<ConsoleLogsResponse> => apiClient.get('/console-logs'),

  clearLogs: (): Promise<{ success: boolean }> => apiClient.delete('/console-logs'),

  getLevel: (): Promise<ConsoleLogLevelResponse> => apiClient.get('/console-logs/level'),

  setLevel: (level: string): Promise<ConsoleLogLevelResponse> =>
    apiClient.put('/console-logs/level', { level }),

  getStreamUrl: (apiBase: string, managementKey?: string): string => {
    const base = computeApiUrl(apiBase);
    const url = new URL(`${base}/console-logs/stream`, window.location.href);
    if (managementKey) {
      url.searchParams.set('key', managementKey);
    }
    return url.toString();
  },
};
