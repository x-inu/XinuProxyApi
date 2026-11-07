import { useEffect, useRef, useState } from 'react';
import { IconTrash2 } from '@/components/ui/icons';
import { Button } from '@/components/ui/Button';
import { ConsoleLogStream } from './ConsoleLogStream';
import { useAuthStore, useNotificationStore } from '@/stores';
import { consoleLogsApi } from '@/services/api/consoleLogs';
import { toConsoleEntry, type ConsoleEntry, type LogLevel } from '@/lib/consoleLog';
import styles from './TerminalView.module.scss';

const MAX_LINES = 200;
const LOG_LEVELS: LogLevel[] = ['debug', 'info', 'warn', 'error'];

export function TerminalView() {
  const { showNotification } = useNotificationStore();
  const apiBase = useAuthStore((state) => state.apiBase);
  const managementKey = useAuthStore((state) => state.managementKey);

  const [entries, setEntries] = useState<ConsoleEntry[]>([]);
  const [logLevel, setLogLevel] = useState<string>('info');
  const [levelBusy, setLevelBusy] = useState(false);
  const seqRef = useRef(0);

  const adoptBuffer = (raw: unknown[]): void => {
    seqRef.current = 0;
    setEntries(raw.map((item) => toConsoleEntry(item as Record<string, unknown>, seqRef.current++)));
  };

  const fetchLogLevel = async () => {
    try {
      const res = await consoleLogsApi.getLevel();
      if (typeof res.level === 'string') setLogLevel(res.level);
    } catch {
      // keep default
    }
  };

  const handleLevelChange = async (e: React.ChangeEvent<HTMLSelectElement>) => {
    const next = e.target.value;
    const prev = logLevel;
    setLogLevel(next);
    setLevelBusy(true);
    try {
      const res = await consoleLogsApi.setLevel(next);
      if (!res.success) throw new Error('Log level update was rejected');
      setLogLevel(res.level || next);
      showNotification(`Log level set to ${res.level || next} (no restart needed)`, 'success');
    } catch (err) {
      setLogLevel(prev);
      showNotification(
        `Failed to set log level: ${err instanceof Error ? err.message : String(err)}`,
        'error'
      );
    } finally {
      setLevelBusy(false);
    }
  };

  const handleClear = async () => {
    try {
      await consoleLogsApi.clearLogs();
      setEntries([]);
      showNotification('Console logs cleared', 'success');
    } catch (err) {
      showNotification(
        `Failed to clear console logs: ${err instanceof Error ? err.message : String(err)}`,
        'error'
      );
    }
  };

  useEffect(() => {
    fetchLogLevel();
  }, []);

  useEffect(() => {
    let cancelled = false;
    let controller: AbortController | null = null;

    const connect = async () => {
      try {
        controller = new AbortController();
        const streamUrl = consoleLogsApi.getStreamUrl(apiBase);
        const headers: Record<string, string> = {
          Accept: 'text/event-stream',
        };
        if (managementKey) {
          headers.Authorization = `Bearer ${managementKey}`;
        }

        const res = await fetch(streamUrl, {
          method: 'GET',
          headers,
          signal: controller.signal,
        });

        if (!res.ok) throw new Error(`HTTP ${res.status}`);

        const reader = res.body?.getReader();
        if (!reader) return;

        const decoder = new TextDecoder();
        let buffer = '';

        while (!cancelled) {
          const { done, value } = await reader.read();
          if (done) break;

          buffer += decoder.decode(value, { stream: true });
          const frames = buffer.split('\n');
          buffer = frames.pop() || '';

          for (const frame of frames) {
            const trimmed = frame.trim();
            if (!trimmed.startsWith('data: ')) continue;

            let msg: Record<string, unknown>;
            try {
              msg = JSON.parse(trimmed.slice(6));
            } catch {
              continue;
            }

            if (msg.type === 'init') {
              adoptBuffer(Array.isArray(msg.entries) ? msg.entries : []);
            } else if (msg.type === 'line') {
              setEntries((prev) =>
                [...prev, toConsoleEntry(msg.entry as Record<string, unknown>, seqRef.current++)].slice(
                  -MAX_LINES
                )
              );
            } else if (msg.type === 'clear') {
              setEntries([]);
            }
          }
        }
      } catch {
        // stream closed or aborted — reconnect below
      }

      if (!cancelled) {
        setTimeout(connect, 5000);
      }
    };

    connect();

    return () => {
      cancelled = true;
      controller?.abort();
    };
  }, [apiBase, managementKey]);

  return (
    <div className={styles.cardWrapper}>
      {/* Upper Level Bar */}
      <div className={styles.levelBar}>
        <label className={styles.levelLabel} htmlFor="console-log-level">
          Level
        </label>
        <select
          id="console-log-level"
          className={styles.levelSelect}
          value={logLevel}
          disabled={levelBusy}
          onChange={handleLevelChange}
        >
          {LOG_LEVELS.map((level) => (
            <option key={level} value={level}>
              {level}
            </option>
          ))}
        </select>
        <span className={styles.levelHint}>
          Applies immediately — lines below this level are not buffered.
        </span>

        <Button
          size="sm"
          variant="outline"
          className={styles.clearButton}
          onClick={handleClear}
        >
          <IconTrash2 size={14} />
          Clear
        </Button>
      </div>

      {/* Stream Viewer Component */}
      <ConsoleLogStream entries={entries} />
    </div>
  );
}
