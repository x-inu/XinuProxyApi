import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  IconArrowDown,
  IconCopy,
  IconDownload,
  IconPause,
  IconPlay,
  IconSearch,
  IconTerminal,
  IconTrash2,
} from '@/components/ui/icons';
import { Button } from '@/components/ui/Button';
import { useAuthStore, useNotificationStore } from '@/stores';
import {
  consoleLogsApi,
  type ConsoleLogEntry,
  type ConsoleLogLevel,
} from '@/services/api/consoleLogs';
import { copyToClipboard } from '@/utils/clipboard';
import { downloadBlob } from '@/utils/download';
import styles from './ConsoleLogStream.module.scss';

const MAX_STORED_LINES = 1000;
const LOG_LEVELS: ConsoleLogLevel[] = ['error', 'warn', 'info', 'debug'];

export function ConsoleLogStream() {
  const { t } = useTranslation();
  const { showNotification, showConfirmation } = useNotificationStore();
  const apiBase = useAuthStore((state) => state.apiBase);
  const managementKey = useAuthStore((state) => state.managementKey);

  const [entries, setEntries] = useState<ConsoleLogEntry[]>([]);
  const [logLevel, setLogLevel] = useState<string>('info');
  const [levelBusy, setLevelBusy] = useState(false);
  const [streamStatus, setStreamStatus] = useState<'connected' | 'paused' | 'reconnecting'>('reconnecting');
  const [isPaused, setIsPaused] = useState(false);
  const [mutedLevels, setMutedLevels] = useState<Set<ConsoleLogLevel>>(new Set());
  const [searchQuery, setSearchQuery] = useState('');
  const [wrapLines, setWrapLines] = useState(true);
  const [isFollowing, setIsFollowing] = useState(true);

  const viewportRef = useRef<HTMLDivElement | null>(null);
  const abortControllerRef = useRef<AbortController | null>(null);
  const isPausedRef = useRef(isPaused);

  useEffect(() => {
    isPausedRef.current = isPaused;
  }, [isPaused]);

  // 1. Fetch current server log level
  useEffect(() => {
    let active = true;
    consoleLogsApi
      .getLevel()
      .then((res) => {
        if (active && res.level) setLogLevel(res.level);
      })
      .catch((err) => {
        console.warn('Failed to get console log level', err);
      });
    return () => {
      active = false;
    };
  }, []);

  // 2. Change server log level
  const handleLevelChange = async (next: string) => {
    const prev = logLevel;
    setLogLevel(next);
    setLevelBusy(true);
    try {
      const res = await consoleLogsApi.setLevel(next);
      if (res.success && res.level) {
        setLogLevel(res.level);
        showNotification(
          t('logs.console_level_updated', {
            level: res.level,
            defaultValue: `Server log level set to ${res.level} (no restart needed)`,
          }),
          'success'
        );
      }
    } catch (err) {
      setLogLevel(prev);
      showNotification(
        t('logs.console_level_failed', {
          defaultValue: `Failed to change log level: ${err instanceof Error ? err.message : String(err)}`,
        }),
        'error'
      );
    } finally {
      setLevelBusy(false);
    }
  };

  // 3. Live SSE Connection via fetch and ReadableStream
  useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();
    abortControllerRef.current = controller;

    const connectStream = async () => {
      try {
        setStreamStatus('reconnecting');
        const streamUrl = consoleLogsApi.getStreamUrl(apiBase, managementKey);
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

        if (!res.ok) {
          throw new Error(`HTTP ${res.status}: ${res.statusText}`);
        }

        if (!res.body) {
          throw new Error('ReadableStream not supported on this response');
        }

        setStreamStatus(isPausedRef.current ? 'paused' : 'connected');

        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';

        while (!cancelled) {
          const { done, value } = await reader.read();
          if (done) break;

          buffer += decoder.decode(value, { stream: true });
          const frames = buffer.split('\n\n');
          buffer = frames.pop() || '';

          for (const frame of frames) {
            const lines = frame.split('\n');
            for (const rawLine of lines) {
              const trimmed = rawLine.trim();
              if (!trimmed.startsWith('data: ')) continue;
              const jsonStr = trimmed.slice(6);
              try {
                const ev = JSON.parse(jsonStr) as {
                  type: string;
                  entry?: ConsoleLogEntry;
                  entries?: ConsoleLogEntry[];
                };

                if (ev.type === 'init' && Array.isArray(ev.entries)) {
                  setEntries(ev.entries.slice(-MAX_STORED_LINES));
                } else if (ev.type === 'line' && ev.entry) {
                  if (!isPausedRef.current) {
                    setEntries((prev) => {
                      const next = [...prev, ev.entry!];
                      return next.length > MAX_STORED_LINES ? next.slice(-MAX_STORED_LINES) : next;
                    });
                  }
                } else if (ev.type === 'clear') {
                  setEntries([]);
                }
              } catch {
                // Ignore parse errors from ping or partial frames
              }
            }
          }
        }
      } catch (err: unknown) {
        if (!cancelled && !(err instanceof DOMException && err.name === 'AbortError')) {
          setStreamStatus('reconnecting');
          // Reconnect with backoff
          setTimeout(() => {
            if (!cancelled) connectStream();
          }, 3000);
        }
      }
    };

    connectStream();

    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [apiBase, managementKey]);

  // 4. Auto-scroll logic
  useEffect(() => {
    if (isFollowing && viewportRef.current) {
      viewportRef.current.scrollTop = viewportRef.current.scrollHeight;
    }
  }, [entries, isFollowing]);

  const handleScroll = () => {
    if (!viewportRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = viewportRef.current;
    const distanceToBottom = scrollHeight - scrollTop - clientHeight;
    setIsFollowing(distanceToBottom < 30);
  };

  const scrollToBottom = () => {
    if (viewportRef.current) {
      viewportRef.current.scrollTop = viewportRef.current.scrollHeight;
      setIsFollowing(true);
    }
  };

  // 5. Level filter toggle
  const toggleLevel = (lvl: ConsoleLogLevel) => {
    setMutedLevels((prev) => {
      const next = new Set(prev);
      if (next.has(lvl)) next.delete(lvl);
      else next.add(lvl);
      return next;
    });
  };

  // 6. Counts per level
  const counts = useMemo(() => {
    const res = { error: 0, warn: 0, info: 0, debug: 0 };
    for (const e of entries) {
      if (e.level in res) {
        res[e.level as ConsoleLogLevel]++;
      }
    }
    return res;
  }, [entries]);

  // 7. Filtered entries
  const visibleEntries = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    return entries.filter((e) => {
      if (mutedLevels.has(e.level)) return false;
      if (!q) return true;
      const lineText = `${e.time} ${e.level} ${e.request_id || ''} ${e.caller || ''} ${e.line || e.message || ''}`.toLowerCase();
      return lineText.includes(q);
    });
  }, [entries, mutedLevels, searchQuery]);

  // 8. Actions
  const handleClear = () => {
    showConfirmation({
      title: t('logs.console_clear_title', { defaultValue: 'Clear Console Logs' }),
      message: t('logs.console_clear_confirm', {
        defaultValue: 'Are you sure you want to clear the in-memory console log buffer?',
      }),
      confirmText: t('common.clear', { defaultValue: 'Clear' }),
      onConfirm: async () => {
        try {
          await consoleLogsApi.clearLogs();
          setEntries([]);
          showNotification(
            t('logs.console_cleared', { defaultValue: 'Console logs cleared' }),
            'success'
          );
        } catch (err) {
          showNotification(
            t('logs.console_clear_failed', {
              defaultValue: `Failed to clear logs: ${err instanceof Error ? err.message : String(err)}`,
            }),
            'error'
          );
        }
      },
    });
  };

  const handleCopy = async () => {
    if (visibleEntries.length === 0) return;
    const text = visibleEntries.map((e) => e.line || e.message).join('\n');
    const success = await copyToClipboard(text);
    if (success) {
      showNotification(
        t('logs.console_copied', {
          count: visibleEntries.length,
          defaultValue: `Copied ${visibleEntries.length} log lines`,
        }),
        'success'
      );
    }
  };

  const handleDownload = () => {
    if (visibleEntries.length === 0) return;
    const text = visibleEntries.map((e) => e.line || e.message).join('\n');
    const blob = new Blob([text], { type: 'text/plain;charset=utf-8' });
    const filename = `xinuproxyapi-console-${new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-')}.log`;
    downloadBlob({ filename, blob });
    showNotification(
      t('logs.console_downloaded', {
        count: visibleEntries.length,
        defaultValue: `Downloaded ${visibleEntries.length} log lines`,
      }),
      'success'
    );
  };

  const togglePause = () => {
    setIsPaused((prev) => {
      const next = !prev;
      setStreamStatus(next ? 'paused' : 'connected');
      return next;
    });
  };

  const formatTimestamp = (iso: string) => {
    try {
      const d = new Date(iso);
      if (Number.isNaN(d.getTime())) return iso;
      return d.toLocaleTimeString(undefined, {
        hour12: false,
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      });
    } catch {
      return iso;
    }
  };

  return (
    <div className={styles.consoleCard}>
      {/* Toolbar */}
      <div className={styles.toolbar}>
        <div className={styles.toolbarLeft}>
          {/* Status Indicator */}
          <div className={styles.statusIndicator}>
            <span
              className={`${styles.statusDot} ${
                streamStatus === 'connected'
                  ? styles.connected
                  : streamStatus === 'paused'
                    ? styles.paused
                    : styles.reconnecting
              }`}
            />
            <span>
              {streamStatus === 'connected'
                ? t('logs.console_connected', { defaultValue: 'LIVE' })
                : streamStatus === 'paused'
                  ? t('logs.console_paused', { defaultValue: 'PAUSED' })
                  : t('logs.console_connecting', { defaultValue: 'CONNECTING' })}
            </span>
          </div>

          {/* Level Switcher */}
          <div className={styles.levelSelectWrapper}>
            <span>{t('logs.console_level_label', { defaultValue: 'Level:' })}</span>
            <select
              className={styles.levelSelect}
              value={logLevel}
              disabled={levelBusy}
              onChange={(e) => handleLevelChange(e.target.value)}
              title={t('logs.console_level_select_hint', {
                defaultValue: 'Change server log level on-the-fly (no restart required)',
              })}
            >
              <option value="debug">DEBUG</option>
              <option value="info">INFO</option>
              <option value="warn">WARN</option>
              <option value="error">ERROR</option>
            </select>
          </div>

          {/* Filter Chips */}
          <div className={styles.chipGroup}>
            {LOG_LEVELS.map((lvl) => {
              const isMuted = mutedLevels.has(lvl);
              const chipClass =
                lvl === 'error'
                  ? styles.chipError
                  : lvl === 'warn'
                    ? styles.chipWarn
                    : lvl === 'info'
                      ? styles.chipInfo
                      : styles.chipDebug;

              return (
                <button
                  key={lvl}
                  type="button"
                  onClick={() => toggleLevel(lvl)}
                  className={`${styles.chip} ${!isMuted ? `${styles.active} ${chipClass}` : styles.muted}`}
                  title={`${isMuted ? 'Show' : 'Hide'} ${lvl.toUpperCase()} logs`}
                >
                  <span>{lvl.slice(0, 3).toUpperCase()}</span>
                  <span className={styles.chipCount}>{counts[lvl]}</span>
                </button>
              );
            })}
          </div>

          {/* Search Box */}
          <div className={styles.searchWrapper}>
            <IconSearch size={14} className={styles.searchIcon} />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder={t('logs.search_placeholder', { defaultValue: 'Search console...' })}
              className={styles.searchInput}
            />
          </div>
        </div>

        {/* Action Buttons */}
        <div className={styles.toolbarRight}>
          {/* Pause / Resume */}
          <Button
            variant="ghost"
            size="sm"
            onClick={togglePause}
            title={isPaused ? 'Resume live streaming' : 'Pause live streaming'}
          >
            {isPaused ? <IconPlay size={15} /> : <IconPause size={15} />}
            {' '}
            {isPaused
              ? t('logs.console_resume', { defaultValue: 'Resume' })
              : t('logs.console_pause', { defaultValue: 'Pause' })}
          </Button>

          {/* Follow toggle / scroll down */}
          {!isFollowing && (
            <Button
              variant="secondary"
              size="sm"
              onClick={scrollToBottom}
              title="Jump to latest logs"
            >
              <IconArrowDown size={15} />
              {' '}
              {t('logs.console_jump_latest', { defaultValue: 'Follow' })}
            </Button>
          )}

          {/* Wrap toggle */}
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setWrapLines((prev) => !prev)}
            title="Toggle word wrap"
          >
            {wrapLines ? 'Wrap' : 'No Wrap'}
          </Button>

          {/* Copy */}
          <Button
            variant="ghost"
            size="sm"
            onClick={handleCopy}
            disabled={visibleEntries.length === 0}
            title="Copy visible console lines"
          >
            <IconCopy size={15} />
          </Button>

          {/* Download */}
          <Button
            variant="ghost"
            size="sm"
            onClick={handleDownload}
            disabled={visibleEntries.length === 0}
            title="Download visible console lines"
          >
            <IconDownload size={15} />
          </Button>

          {/* Clear */}
          <Button
            variant="ghost"
            size="sm"
            onClick={handleClear}
            disabled={entries.length === 0}
            title="Clear in-memory console logs"
          >
            <IconTrash2 size={15} />
          </Button>
        </div>
      </div>

      {/* Viewport */}
      <div
        ref={viewportRef}
        onScroll={handleScroll}
        className={styles.viewport}
      >
        {visibleEntries.length === 0 ? (
          <div className={styles.emptyState}>
            <IconTerminal size={32} />
            <span>
              {entries.length === 0
                ? t('logs.console_empty', { defaultValue: 'Waiting for server log output...' })
                : t('logs.search_empty_desc', { defaultValue: 'No logs match your filter criteria' })}
            </span>
          </div>
        ) : (
          visibleEntries.map((entry, idx) => {
            const lvl = entry.level.toLowerCase();
            const tagClass =
              lvl === 'error'
                ? styles.err
                : lvl === 'warn'
                  ? styles.wrn
                  : lvl === 'info'
                    ? styles.inf
                    : styles.dbg;

            return (
              <div
                key={`${entry.time}-${idx}`}
                className={`${styles.logLine} ${!wrapLines ? styles.nowrap : ''}`}
              >
                <span className={styles.timeCol}>{formatTimestamp(entry.time)}</span>
                <span className={`${styles.levelTag} ${tagClass}`}>
                  {entry.level.slice(0, 3).toUpperCase()}
                </span>
                {entry.request_id && (
                  <span className={styles.reqIdTag}>{entry.request_id}</span>
                )}
                {entry.caller && (
                  <span className={styles.callerTag}>[{entry.caller}]</span>
                )}
                <span className={styles.messageCol}>{entry.line || entry.message}</span>
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}
