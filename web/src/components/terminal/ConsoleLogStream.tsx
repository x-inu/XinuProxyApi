import { useEffect, useMemo, useRef, useState } from 'react';
import {
  IconArrowDown,
  IconCopy,
  IconDownload,
  IconSearch,
  IconX,
} from '@/components/ui/icons';
import { Button } from '@/components/ui/Button';
import { useNotificationStore } from '@/stores';
import {
  countByLevel,
  downloadFilename,
  filterEntries,
  formatClock,
  splitTag,
  toPlainText,
  type ConsoleEntry,
  type LogLevel,
} from '@/lib/consoleLog';
import { copyToClipboard } from '@/utils/clipboard';
import { downloadBlob } from '@/utils/download';
import styles from './TerminalView.module.scss';

interface Props {
  entries: ConsoleEntry[];
}

const LEVEL_ORDER: LogLevel[] = ['error', 'warn', 'info', 'debug'];

export function ConsoleLogStream({ entries }: Props) {
  const { showNotification } = useNotificationStore();

  const LEVEL_META: Record<
    LogLevel,
    { label: string; textClass: string; chipClass: string; dotClass: string }
  > = useMemo(
    () => ({
      error: {
        label: 'ERR',
        textClass: styles.levelErrorText,
        chipClass: styles.levelErrorChip,
        dotClass: styles.levelErrorDot,
      },
      warn: {
        label: 'WRN',
        textClass: styles.levelWarnText,
        chipClass: styles.levelWarnChip,
        dotClass: styles.levelWarnDot,
      },
      info: {
        label: 'INF',
        textClass: styles.levelInfoText,
        chipClass: styles.levelInfoChip,
        dotClass: styles.levelInfoDot,
      },
      debug: {
        label: 'DBG',
        textClass: styles.levelDebugText,
        chipClass: styles.levelDebugChip,
        dotClass: styles.levelDebugDot,
      },
    }),
    []
  );

  const [query, setQuery] = useState('');
  const [wrap, setWrap] = useState(false);
  const [following, setFollowing] = useState(true);
  const [mutedLevels, setMutedLevels] = useState<Set<LogLevel>>(new Set());
  const viewportRef = useRef<HTMLDivElement | null>(null);

  const counts = useMemo(() => countByLevel(entries), [entries]);
  const kept = useMemo(
    () => entries.filter((entry) => !mutedLevels.has(entry.level)),
    [entries, mutedLevels]
  );
  const visible = useMemo(() => filterEntries(kept, query), [kept, query]);

  const toggleLevel = (level: LogLevel) => {
    setMutedLevels((prev) => {
      const next = new Set(prev);
      if (next.has(level)) next.delete(level);
      else next.add(level);
      return next;
    });
  };

  const scrollToLatest = () => {
    if (viewportRef.current) {
      viewportRef.current.scrollTop = viewportRef.current.scrollHeight;
    }
  };

  const handleScroll = () => {
    if (!viewportRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = viewportRef.current;
    setFollowing(scrollHeight - scrollTop - clientHeight < 24);
  };

  const copyVisible = async () => {
    try {
      const text = toPlainText(visible);
      const success = await copyToClipboard(text);
      if (success) {
        showNotification(
          `Copied ${visible.length} log ${visible.length === 1 ? 'line' : 'lines'}`,
          'success'
        );
      }
    } catch {
      showNotification('Clipboard is unavailable in this browser', 'error');
    }
  };

  const downloadVisible = () => {
    const text = toPlainText(visible);
    const blob = new Blob([text], { type: 'text/plain;charset=utf-8' });
    const filename = downloadFilename();
    downloadBlob({ filename, blob });
    showNotification(
      `Downloaded ${visible.length} log ${visible.length === 1 ? 'line' : 'lines'}`,
      'success'
    );
  };

  useEffect(() => {
    const handleKeydown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && query) {
        setQuery('');
        e.preventDefault();
      }
    };
    window.addEventListener('keydown', handleKeydown);
    return () => window.removeEventListener('keydown', handleKeydown);
  }, [query]);

  useEffect(() => {
    if (following && viewportRef.current) {
      viewportRef.current.scrollTop = viewportRef.current.scrollHeight;
    }
  }, [entries.length, following]);

  return (
    <div className={styles.streamContainer}>
      {/* Stream Toolbar */}
      <div className={styles.toolbar}>
        {/* Level counts double as filters */}
        <div className={styles.chipGroup}>
          {LEVEL_ORDER.map((level) => {
            const meta = LEVEL_META[level];
            const muted = mutedLevels.has(level);
            return (
              <button
                key={level}
                type="button"
                aria-pressed={!muted}
                title={muted ? `Show ${meta.label} lines` : `Hide ${meta.label} lines`}
                onClick={() => toggleLevel(level)}
                className={`${styles.chipButton} ${meta.chipClass} ${
                  muted ? styles.muted : ''
                }`}
              >
                <span className={`${styles.chipDot} ${meta.dotClass}`} />
                {meta.label}
                <span className={styles.chipCount}>{counts[level]}</span>
              </button>
            );
          })}
        </div>

        {/* Search input */}
        <div className={styles.searchContainer}>
          <IconSearch size={14} className={styles.searchIcon} />
          <input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search lines…"
            aria-label="Search console log lines"
            className={styles.searchInput}
          />
          {query ? (
            <button
              type="button"
              aria-label="Clear search"
              onClick={() => setQuery('')}
              className={styles.searchClearButton}
            >
              <IconX size={14} />
            </button>
          ) : null}
        </div>

        {/* Wrap Toggle */}
        <button
          type="button"
          aria-pressed={wrap}
          title={wrap ? 'Disable line wrap' : 'Wrap long lines'}
          onClick={() => setWrap(!wrap)}
          className={`${styles.wrapButton} ${wrap ? styles.active : ''}`}
        >
          Wrap
        </button>

        {/* Copy */}
        <Button
          size="sm"
          variant="outline"
          title="Copy visible lines"
          disabled={visible.length === 0}
          onClick={copyVisible}
          className={styles.actionButton}
        >
          <IconCopy size={14} />
          Copy
        </Button>

        {/* Export */}
        <Button
          size="sm"
          variant="outline"
          title="Download visible lines"
          disabled={visible.length === 0}
          onClick={downloadVisible}
          className={styles.actionButton}
        >
          <IconDownload size={14} />
          Export
        </Button>
      </div>

      {/* Viewport */}
      <div
        ref={viewportRef}
        onScroll={handleScroll}
        className={styles.viewport}
      >
        {entries.length === 0 ? (
          <div className={styles.emptyState}>
            <p className={styles.emptyTitle}>No console output yet</p>
            <p className={styles.emptyDesc}>
              Nothing has been logged since the server started or the last clear. Send a request, or
              set the level to <span className={styles.codeSpan}>debug</span>, and rows appear here
              live.
            </p>
          </div>
        ) : visible.length === 0 ? (
          <div className={styles.emptyState}>
            <p className={styles.emptyTitle}>Nothing matches the current filter</p>
            <p className={styles.emptyDesc}>
              {mutedLevels.size > 0
                ? `${Array.from(mutedLevels)
                    .map((level) => LEVEL_META[level].label)
                    .join(', ')} rows are hidden by the level chips.`
                : `No line contains “${query.trim()}”.`}
            </p>
            <Button
              size="sm"
              variant="outline"
              onClick={() => {
                setQuery('');
                setMutedLevels(new Set());
              }}
            >
              Show all {entries.length} lines
            </Button>
          </div>
        ) : (
          <div className={styles.rowList}>
            {visible.map((entry) => {
              const meta = LEVEL_META[entry.level];
              const parts = splitTag(entry.line);
              return (
                <div key={entry.seq} className={styles.rowItem}>
                  <time className={styles.rowTime}>{formatClock(entry.time)}</time>
                  <span className={`${styles.rowLevel} ${meta.textClass}`}>{meta.label}</span>
                  <span
                    className={`${styles.rowMessage} ${
                      wrap ? styles.wrap : styles.nowrap
                    } ${meta.textClass}`}
                  >
                    {parts ? (
                      <>
                        <span className={styles.rowTag}>[{parts.tag}]</span>{' '}
                      </>
                    ) : null}
                    {parts?.message ?? entry.line}
                  </span>
                </div>
              );
            })}
          </div>
        )}

        {/* Jump to latest floating pill */}
        {!following && entries.length > 0 && (
          <button
            type="button"
            onClick={() => {
              setFollowing(true);
              scrollToLatest();
            }}
            className={styles.jumpButton}
          >
            <IconArrowDown size={14} />
            Jump to latest
          </button>
        )}
      </div>

      {/* Footer statistics */}
      <div className={styles.footer}>
        <span className={styles.tabularNums}>
          Showing {visible.length} of {entries.length} buffered{' '}
          {entries.length === 1 ? 'line' : 'lines'}
        </span>
        <span className={styles.tabularNums}>
          {counts.error} error · {counts.warn} warn · {counts.info} info · {counts.debug} debug
        </span>
      </div>
    </div>
  );
}
